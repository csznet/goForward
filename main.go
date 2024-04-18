package main

import (
	"flag"
	"sync"

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
	// 初始化通道
	conf.Ch = make(chan string)
	forwardList := sql.GetAction()
	if len(forwardList) == 0 {
		//添加测试数据
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
				RemoteAddr: forwardList[i].RemoteAddr,
				TotalBytes: forwardList[i].TotalBytes,
			},
			TotalBytesOld:  forwardList[i].TotalBytes,
			TotalBytesLock: sync.Mutex{},
		}

		largeStats.Connections[i] = connectionStats
	}
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
	defer close(conf.Ch)
}
func init() {
	flag.StringVar(&conf.WebPort, "port", "8889", "Web Port")
	flag.StringVar(&conf.Db, "db", "goForward.db", "Db Path")
	flag.StringVar(&conf.WebIP, "ip", "0.0.0.0", "Web IP")
	flag.StringVar(&conf.WebPass, "pass", "", "Web Password")
	flag.IntVar(&conf.TcpTimeout, "tt", 60, "Tcp Timeout")
	flag.Parse()
}
