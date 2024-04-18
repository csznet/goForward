package conf

import (
	"fmt"
	"sync"
)

// ConnectionStats 结构体用于保存多个连接信息
type ConnectionStats struct {
	Id            int `gorm:"primaryKey;autoIncrement"`
	LocalPort     string
	RemoteAddr    string
	RemotePort    string
	Protocol      string
	Status        int
	TotalBytes    uint64
	TotalGigabyte uint64
}

type IpBan struct {
	Id        int `gorm:"primaryKey;autoIncrement"`
	Ip        string
	TimeStamp int64
}

// 全局转发协程等待组
var Wg sync.WaitGroup

// 全局协程通道 未初始化默认为nil
var Ch chan string

// Web管理面板端口
var WebPort string

// Web IP绑定
var WebIP string

// Web管理面板密码
var WebPass string

// TCP超时
var TcpTimeout int

// 版本号
var version string

// 数据库位置
var Db = "goForward.db"

func init() {
	if version != "" {
		fmt.Println("goForward Version " + version)
	}
}
