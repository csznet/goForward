package main

import (
	"sync"

	"csz.net/goForward/conf"
	"csz.net/goForward/forward"
	"csz.net/goForward/sql"
	"csz.net/goForward/web"
)

func main() {
	go web.Run()
	forwardList := sql.GetForwardList()

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

	// 初始化通道
	conf.Ch = make(chan string)

	// 并发执行多个转发
	for _, stats := range largeStats.Connections {
		go forward.Run(stats, &conf.Wg)
	}

	conf.Wg.Wait()
	defer close(conf.Ch)
}
