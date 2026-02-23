package forward

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"csz.net/goForward/conf"
	"csz.net/goForward/sql"
)

const (
	bufSize      = 4096
	statsInterval = 10 * time.Second
	gbSize        = uint64(1 << 30) // 1 GiB
	tcpTimeStep   = 5               // TcpTime 每个统计周期增加的值
)

type ConnectionStats struct {
	conf.ConnectionStats
	TotalBytesOld  uint64     `gorm:"-"`
	TotalBytesLock sync.Mutex `gorm:"-"` // 保护 TotalBytes 相关字段及 TCPConnections
	TCPConnections []net.Conn `gorm:"-"`
	TcpTime        int        `gorm:"-"`
}

// 保存多个连接信息
type LargeConnectionStats struct {
	Connections []*ConnectionStats `json:"connections"`
}

// 复用缓冲区
var bufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, bufSize)
	},
}

// 开启转发，负责分发具体转发
func Run(stats *ConnectionStats) {
	defer releaseResources(stats)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 注册停止函数，外部通过 conf.StopFuncs 触发停止
	key := stats.LocalPort + stats.Protocol
	conf.StopFuncs.Store(key, cancel)
	defer conf.StopFuncs.Delete(key)

	var innerWg sync.WaitGroup
	innerWg.Add(1)
	go func() {
		defer innerWg.Done()
		stats.printStats(ctx)
	}()

	log.Printf("【%s】监听端口 %s 转发至 %s:%s", stats.Protocol, stats.LocalPort, stats.RemoteAddr, stats.RemotePort)

	if stats.Protocol == "udp" {
		localAddr, err := net.ResolveUDPAddr("udp", ":"+stats.LocalPort)
		if err != nil {
			log.Println("解析本地地址时发生错误:", err)
			return
		}
		remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(stats.RemoteAddr, stats.RemotePort))
		if err != nil {
			log.Println("解析远程地址时发生错误:", err)
			return
		}
		conn, err := net.ListenUDP("udp", localAddr)
		if err != nil {
			log.Println("监听时发生错误:", err)
			return
		}
		defer conn.Close()

		// context 取消时关闭 UDP 连接
		go func() {
			<-ctx.Done()
			conn.Close()
		}()

		innerWg.Add(1)
		go func() {
			defer innerWg.Done()
			stats.handleUDPConnection(conn, remoteAddr, ctx)
		}()
	} else {
		// TCP转发
		listener, err := net.Listen("tcp", ":"+stats.LocalPort)
		if err != nil {
			log.Println("监听时发生错误:", err)
			return
		}
		defer listener.Close()

		// context 取消时关闭监听器及所有活跃连接
		go func() {
			<-ctx.Done()
			listener.Close()
			// 加锁复制连接列表，释放锁后再关闭，避免长时间持锁
			stats.TotalBytesLock.Lock()
			conns := make([]net.Conn, len(stats.TCPConnections))
			copy(conns, stats.TCPConnections)
			stats.TotalBytesLock.Unlock()
			for _, c := range conns {
				c.Close()
			}
		}()

		for {
			clientConn, err := listener.Accept()
			if err != nil {
				log.Printf("【%s】停止接受新连接", stats.LocalPort)
				break
			}
			innerWg.Add(1)
			go func(conn net.Conn) {
				defer innerWg.Done()
				stats.handleTCPConnection(conn)
			}(clientConn)
		}
	}
	innerWg.Wait()
}

// TCP转发
func (cs *ConnectionStats) handleTCPConnection(clientConn net.Conn) {
	defer clientConn.Close()
	remoteConn, err := net.Dial("tcp", net.JoinHostPort(cs.RemoteAddr, cs.RemotePort))
	if err != nil {
		log.Printf("【%s】连接远程地址时发生错误: %v", cs.LocalPort, err)
		return
	}
	defer remoteConn.Close()

	// 加锁后添加连接，避免竞态
	cs.TotalBytesLock.Lock()
	cs.TCPConnections = append(cs.TCPConnections, clientConn, remoteConn)
	cs.TotalBytesLock.Unlock()

	var copyWG sync.WaitGroup
	copyWG.Add(2)
	go func() {
		defer copyWG.Done()
		cs.copyBytes(clientConn, remoteConn)
	}()
	go func() {
		defer copyWG.Done()
		cs.copyBytes(remoteConn, clientConn)
	}()
	copyWG.Wait()

	// 连接结束后从跟踪列表中移除
	cs.TotalBytesLock.Lock()
	cs.removeConns(clientConn, remoteConn)
	cs.TotalBytesLock.Unlock()
}

// removeConns 从 TCPConnections 中移除指定连接。调用时须持有 TotalBytesLock。
func (cs *ConnectionStats) removeConns(conns ...net.Conn) {
	set := make(map[net.Conn]struct{}, len(conns))
	for _, c := range conns {
		set[c] = struct{}{}
	}
	result := cs.TCPConnections[:0]
	for _, c := range cs.TCPConnections {
		if _, found := set[c]; !found {
			result = append(result, c)
		}
	}
	cs.TCPConnections = result
}

// UDP转发
func (cs *ConnectionStats) handleUDPConnection(localConn *net.UDPConn, remoteAddr *net.UDPAddr, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			buf, ok := bufPool.Get().([]byte)
			if !ok {
				log.Println("缓冲区类型断言失败")
				return
			}
			n, _, err := localConn.ReadFromUDP(buf)
			if err != nil {
				bufPool.Put(buf)
				log.Printf("【%s】从源读取时发生错误: %v", cs.LocalPort, err)
				return
			}
			cs.TotalBytesLock.Lock()
			cs.TotalBytes += uint64(n)
			cs.TotalBytesLock.Unlock()

			// WriteToUDP 写内核缓冲区后立即返回，直接调用无需额外 goroutine，
			// 避免高流量时频繁 goroutine 创建和 GC 带来的 CPU 压力
			data := make([]byte, n+2)
			binary.BigEndian.PutUint16(data[:2], uint16(n))
			copy(data[2:], buf[:n])
			bufPool.Put(buf)
			cs.forwardUDPMessage(localConn, remoteAddr, data)
		}
	}
}

func (cs *ConnectionStats) forwardUDPMessage(localConn *net.UDPConn, remoteAddr *net.UDPAddr, data []byte) {
	if _, err := localConn.WriteToUDP(data, remoteAddr); err != nil {
		log.Printf("【%s】写入目标时发生错误: %v", cs.LocalPort, err)
	}
}

func (cs *ConnectionStats) copyBytes(dst, src net.Conn) {
	buf, ok := bufPool.Get().([]byte)
	if !ok {
		log.Println("缓冲区类型断言失败")
		dst.Close()
		src.Close()
		return
	}
	defer bufPool.Put(buf)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			cs.TotalBytesLock.Lock()
			cs.TotalBytes += uint64(n)
			cs.TotalBytesLock.Unlock()
			if _, werr := dst.Write(buf[:n]); werr != nil {
				log.Printf("【%s】写入目标时发生错误: %v", cs.LocalPort, werr)
				break
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("【%s】从源读取时发生错误: %v", cs.LocalPort, err)
			}
			break
		}
	}
	// 关闭双向连接，通知对端 goroutine 退出
	dst.Close()
	src.Close()
}

// 定时打印和处理流量变化
func (cs *ConnectionStats) printStats(ctx context.Context) {
	ticker := time.NewTicker(statsInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// 在锁内只做内存状态更新，捕获需要的值后立即释放
			cs.TotalBytesLock.Lock()
			hasTraffic := cs.TotalBytes > cs.TotalBytesOld
			var totalStr string
			var bytesToSave, gbToSave uint64
			var needUpdateGb bool
			var connCount int

			if hasTraffic {
				if cs.Protocol == "tcp" {
					cs.TcpTime = 0
				}
				if float64(cs.TotalBytes)/(1024*1024) < 0.5 {
					totalStr = strconv.FormatFloat(float64(cs.TotalBytes)/1024, 'f', 2, 64) + "KB"
				} else {
					totalStr = strconv.FormatFloat(float64(cs.TotalBytes)/(1024*1024), 'f', 2, 64) + "MB"
				}
				if cs.TotalBytes >= gbSize {
					cs.TotalGigabyte++
					cs.TotalBytes -= gbSize
					needUpdateGb = true
					gbToSave = cs.TotalGigabyte
				}
				cs.TotalBytesOld = cs.TotalBytes
				bytesToSave = cs.TotalBytes
				connCount = len(cs.TCPConnections)
			} else if cs.Protocol == "tcp" {
				if cs.TcpTime >= conf.TcpTimeout {
					for i := len(cs.TCPConnections) - 1; i >= 0; i-- {
						cs.TCPConnections[i].Close()
					}
					cs.TCPConnections = nil
				} else {
					cs.TcpTime += tcpTimeStep
				}
			}
			cs.TotalBytesLock.Unlock()

			// 锁外执行日志输出和 SQL 写入，避免阻塞 copyBytes 的流量统计
			if hasTraffic {
				log.Printf("【%s】端口 %s 统计流量: %s", cs.Protocol, cs.LocalPort, totalStr)
				if needUpdateGb {
					sql.UpdateForwardGb(cs.Id, gbToSave)
				}
				sql.UpdateForwardBytes(cs.Id, bytesToSave)
				log.Printf("【%s】端口 %s 当前连接数: %d", cs.Protocol, cs.LocalPort, connCount)
			}
		case <-ctx.Done():
			return
		}
	}
}

// 关闭所有 TCP 连接并清空切片
func closeTCPConnections(stats *ConnectionStats) {
	stats.TotalBytesLock.Lock()
	defer stats.TotalBytesLock.Unlock()
	for _, conn := range stats.TCPConnections {
		conn.Close()
	}
	stats.TCPConnections = nil
}

// 释放资源
func releaseResources(stats *ConnectionStats) {
	closeTCPConnections(stats)
}
