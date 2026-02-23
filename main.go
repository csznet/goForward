package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"csz.net/goForward/conf"
	"csz.net/goForward/forward"
	"csz.net/goForward/sql"
	"csz.net/goForward/web"
)

func main() {
	go web.Run()
	if conf.TcpTimeout < 5 {
		conf.TcpTimeout = 5
	}

	forwardList := sql.GetAction()
	if len(forwardList) == 0 {
		// 添加测试数据
		testData := conf.ConnectionStats{
			LocalPort:  conf.WebPort,
			RemotePort: conf.WebPort,
			RemoteAddr: "127.0.0.1",
			Protocol:   "udp",
		}
		sql.AddForward(testData)
		forwardList = sql.GetForwardList()
	}
	var largeStats forward.LargeConnectionStats
	largeStats.Connections = make([]*forward.ConnectionStats, len(forwardList))
	for i := range forwardList {
		connectionStats := &forward.ConnectionStats{
			ConnectionStats: conf.ConnectionStats{
				Id:         forwardList[i].Id,
				Protocol:   forwardList[i].Protocol,
				LocalPort:  forwardList[i].LocalPort,
				RemotePort: forwardList[i].RemotePort,
				RemoteAddr: sql.NormalizeAddr(forwardList[i].RemoteAddr), // 兼容旧数据中可能存在的方括号
				TotalBytes: forwardList[i].TotalBytes,
			},
			TotalBytesOld:  forwardList[i].TotalBytes,
			TotalBytesLock: sync.Mutex{},
		}
		largeStats.Connections[i] = connectionStats
	}

	// 监听系统退出信号，优雅关闭所有转发
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh
		log.Println("收到终止信号，正在关闭所有转发...")
		conf.StopFuncs.Range(func(key, value interface{}) bool {
			if cancelFn, ok := value.(context.CancelFunc); ok {
				cancelFn()
			}
			return true
		})
	}()

	// 设置 WaitGroup 计数为连接数
	conf.Wg.Add(len(largeStats.Connections))
	// 并发执行多个转发
	for _, stats := range largeStats.Connections {
		go func(s *forward.ConnectionStats) {
			forward.Run(s)
			conf.Wg.Done()
		}(stats)
	}
	conf.Wg.Wait()
	sql.Close()
}

func init() {
	flag.StringVar(&conf.WebPort, "port", "8889", "Web Port")
	flag.StringVar(&conf.Db, "db", "goForward.db", "Db Path")
	flag.StringVar(&conf.WebIP, "ip", "0.0.0.0", "Web IP")
	flag.StringVar(&conf.WebPass, "pass", "", "Web Password")
	flag.IntVar(&conf.TcpTimeout, "tt", 60, "Tcp Timeout")
	flag.Parse()
	if !strings.HasSuffix(conf.Db, ".db") {
		conf.Db += ".db"
	}
	sql.Once()
}
