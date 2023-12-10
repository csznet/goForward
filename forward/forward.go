package forward

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"csz.net/goForward/conf"
	"csz.net/goForward/sql"
)

type ConnectionStats struct {
	conf.ConnectionStats
	TotalBytesOld  uint64     `gorm:"-"`
	TotalBytesLock sync.Mutex `gorm:"-"`
}

// 保存多个连接信息
type LargeConnectionStats struct {
	Connections []*ConnectionStats `json:"connections"`
}

// 复用缓冲区
var bufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 8192)
	},
}

// 开启转发，负责分发具体转发
func Run(stats *ConnectionStats, wg *sync.WaitGroup) {
	defer wg.Done()

	var ctx, cancel = context.WithCancel(context.Background())
	var innerWg sync.WaitGroup

	defer cancel()

	innerWg.Add(1)
	go stats.printStats(&innerWg, ctx)

	if stats.Protocol == "udp" {
		// UDP转发
		localAddr, err := net.ResolveUDPAddr("udp", ":"+stats.LocalPort)
		if err != nil {
			fmt.Println("解析本地地址时发生错误:", err)
			os.Exit(1)
		}

		remoteAddr, err := net.ResolveUDPAddr("udp", stats.RemoteAddr+":"+stats.RemotePort)
		if err != nil {
			fmt.Println("解析远程地址时发生错误:", err)
			os.Exit(1)
		}

		conn, err := net.ListenUDP("udp", localAddr)
		if err != nil {
			fmt.Println("监听时发生错误:", err)
			os.Exit(1)
		}
		defer conn.Close()
		go func() {
			for {
				select {
				case stopPort := <-conf.Ch:
					if stopPort == stats.LocalPort+stats.Protocol {
						fmt.Printf("【%s】停止监听端口 %s\n", stats.Protocol, stats.LocalPort)
						conn.Close()
						cancel()
						return
					} else {
						conf.Ch <- stopPort
						time.Sleep(3 * time.Second)
					}
				default:
					time.Sleep(1 * time.Second)
				}
			}
		}()
		fmt.Printf("【%s】监听端口 %s 转发至 %s:%s\n", stats.Protocol, stats.LocalPort, stats.RemoteAddr, stats.RemotePort)

		innerWg.Add(1)
		go stats.handleUDPConnection(&innerWg, conn, remoteAddr, ctx)
	} else {
		// TCP转发
		listener, err := net.Listen("tcp", ":"+stats.LocalPort)

		if err != nil {
			fmt.Println("监听时发生错误:", err)
			os.Exit(1)
		}
		defer listener.Close()
		go func() {
			for {
				select {
				case stopPort := <-conf.Ch:
					fmt.Println("通道信息:" + stopPort)
					fmt.Println("当前端口:" + stats.LocalPort)
					if stopPort == stats.LocalPort+stats.Protocol {
						fmt.Printf("【%s】停止监听端口 %s\n", stats.Protocol, stats.LocalPort)
						listener.Close()
						cancel()
						return
					} else {
						conf.Ch <- stopPort
						time.Sleep(3 * time.Second)
					}
				default:
					time.Sleep(1 * time.Second)
				}
			}
		}()
		fmt.Printf("【%s】监听端口 %s 转发至 %s:%s\n", stats.Protocol, stats.LocalPort, stats.RemoteAddr, stats.RemotePort)
		for {
			clientConn, err := listener.Accept()
			if err != nil {
				fmt.Println("接受连接时发生错误:", err)
				cancel()
				break
			}

			innerWg.Add(1)
			go stats.handleTCPConnection(&innerWg, clientConn, ctx)
		}
	}
	innerWg.Wait()
}

// TCP转发
func (cs *ConnectionStats) handleTCPConnection(wg *sync.WaitGroup, clientConn net.Conn, ctx context.Context) {
	defer wg.Done()
	defer clientConn.Close()

	remoteConn, err := net.Dial("tcp", cs.RemoteAddr+":"+cs.RemotePort)
	if err != nil {
		fmt.Println("连接远程地址时发生错误:", err)
		return
	}
	defer remoteConn.Close()

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
	go func() {
		for {
			select {
			case <-ctx.Done():
				// 如果上级 context 被取消，停止接收新连接
				return
			default:
				time.Sleep(3 * time.Second)
			}
		}
	}()

	copyWG.Wait()
}

// UDP转发
func (cs *ConnectionStats) handleUDPConnection(wg *sync.WaitGroup, localConn *net.UDPConn, remoteAddr *net.UDPAddr, ctx context.Context) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			buf := bufPool.Get().([]byte)
			n, _, err := localConn.ReadFromUDP(buf)
			if err != nil {
				fmt.Println("从源读取时发生错误:", err)
				return
			}
			fmt.Printf("收到长度为 %d 的UDP数据包\n", n)
			cs.TotalBytesLock.Lock()
			cs.TotalBytes += uint64(n)
			cs.TotalBytesLock.Unlock()

			// 处理消息的边界和错误情况
			go cs.forwardUDPMessage(localConn, remoteAddr, buf[:n])
			bufPool.Put(buf[:n])
		}
	}
}

func (cs *ConnectionStats) forwardUDPMessage(localConn *net.UDPConn, remoteAddr *net.UDPAddr, message []byte) {
	// 在消息前面添加消息长度信息
	length := make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(len(message)))

	// 组合消息长度和实际消息
	data := append(length, message...)

	_, err := localConn.WriteToUDP(data, remoteAddr)
	if err != nil {
		fmt.Println("写入目标时发生错误:", err)
	}
}

func (cs *ConnectionStats) copyBytes(dst, src net.Conn) {
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			cs.TotalBytesLock.Lock()
			cs.TotalBytes += uint64(n)
			cs.TotalBytesLock.Unlock()

			_, err := dst.Write(buf[:n])
			if err != nil {
				fmt.Println("写入目标时发生错误:", err)
				break
			}
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			fmt.Println("从源读取时发生错误:", err)
			break
		}
	}

	// 关闭连接
	dst.Close()
	src.Close()
}

// 定时打印和处理流量变化
func (cs *ConnectionStats) printStats(wg *sync.WaitGroup, ctx context.Context) {
	defer wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop() // 在函数结束时停止定时器
	for {
		select {
		case <-ticker.C:
			cs.TotalBytesLock.Lock()
			if cs.TotalBytes > cs.TotalBytesOld {
				var total string
				if cs.TotalBytes > 0 && float64(cs.TotalBytes)/(1024*1024) < 0.5 {
					total = strconv.FormatFloat(float64(cs.TotalBytes)/(1024), 'f', 2, 64) + "KB"
				} else {
					total = strconv.FormatFloat(float64(cs.TotalBytes)/(1024*1024), 'f', 2, 64) + "MB"
				}
				fmt.Printf("【%s】端口 %s 统计流量: %s\n", cs.Protocol, cs.LocalPort, total)
				//统计更换单位
				var gb uint64 = 1073741824
				if cs.TotalBytes >= gb {
					cs.TotalGigabyte = cs.TotalGigabyte + 1
					sql.UpdateForwardGb(cs.Id, cs.TotalGigabyte)
					cs.TotalBytes = cs.TotalBytes - gb
				}
				cs.TotalBytesOld = cs.TotalBytes
				sql.UpdateForwardBytes(cs.Id, cs.TotalBytes)
			}
			cs.TotalBytesLock.Unlock()
		//当协程退出时执行
		case <-ctx.Done():
			return
		}
	}
}
