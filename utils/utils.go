package utils

import (
	"sync"

	"csz.net/goForward/conf"
	"csz.net/goForward/forward"
	"csz.net/goForward/sql"
)

// 增加转发并开启
func AddForward(newF conf.ConnectionStats) bool {
	id := sql.AddForward(newF)
	if id > 0 {
		stats := &forward.ConnectionStats{
			ConnectionStats: conf.ConnectionStats{
				Id:         id,
				LocalPort:  newF.LocalPort,
				RemotePort: newF.RemotePort,
				RemoteAddr: newF.RemoteAddr,
				Protocol:   newF.Protocol,
				TotalBytes: 0,
			},
			TotalBytesOld:  0,
			TotalBytesLock: sync.Mutex{},
		}
		conf.Wg.Add(1)
		go forward.Run(stats, &conf.Wg)
		return true
	}
	return false
}

func closeForward(port string) {
	conf.Ch <- port
}
