package utils

import (
	"context"
	"sync"

	"csz.net/goForward/conf"
	"csz.net/goForward/forward"
	"csz.net/goForward/sql"
)

// 增加转发并开启
func AddForward(newF conf.ConnectionStats) bool {
	if newF.LocalPort == conf.WebPort && newF.Protocol == "tcp" {
		return false
	}
	id := sql.AddForward(newF)
	if id > 0 {
		stats := &forward.ConnectionStats{
			ConnectionStats: conf.ConnectionStats{
				Id:         id,
				LocalPort:  newF.LocalPort,
				RemotePort: newF.RemotePort,
				RemoteAddr: sql.NormalizeAddr(newF.RemoteAddr), // 确保裸 IPv6（无方括号）
				Protocol:   newF.Protocol,
				TotalBytes: 0,
			},
			TotalBytesOld:  0,
			TotalBytesLock: sync.Mutex{},
		}
		conf.Wg.Add(1)
		go func() {
			forward.Run(stats)
			conf.Wg.Done()
		}()
		return true
	}
	return false
}

// 删除并关闭指定转发
func DelForward(f conf.ConnectionStats) bool {
	sql.DelForward(f.Id)
	key := f.LocalPort + f.Protocol
	if cancelFn, ok := conf.StopFuncs.Load(key); ok {
		cancelFn.(context.CancelFunc)()
	}
	return true
}

// 改变转发状态
func ExStatus(f conf.ConnectionStats) bool {
	if sql.FreeForward(f.LocalPort, f.Protocol) {
		return false
	}
	if sql.UpdateForwardStatus(f.Id, f.Status) {
		// 启用转发
		if f.Status == 0 {
			stats := &forward.ConnectionStats{
				ConnectionStats: conf.ConnectionStats{
					Id:         f.Id,
					LocalPort:  f.LocalPort,
					RemotePort: f.RemotePort,
					RemoteAddr: sql.NormalizeAddr(f.RemoteAddr), // 确保裸 IPv6（无方括号）
					Protocol:   f.Protocol,
					TotalBytes: f.TotalBytes,
				},
				TotalBytesOld:  f.TotalBytes,
				TotalBytesLock: sync.Mutex{},
			}
			conf.Wg.Add(1)
			go func() {
				forward.Run(stats)
				conf.Wg.Done()
			}()
			return true
		} else {
			// 停止转发
			key := f.LocalPort + f.Protocol
			if cancelFn, ok := conf.StopFuncs.Load(key); ok {
				cancelFn.(context.CancelFunc)()
			}
			return true
		}
	}
	return false
}
