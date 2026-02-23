package sql

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"csz.net/goForward/conf"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 定义数据库指针
var db *gorm.DB

func Once() {
	var err error
	var dbPath string
	executablePath, err := os.Executable()
	if conf.Db == "goForward.db" {
		if err != nil {
			log.Println("获取可执行文件路径失败:", err)
			log.Println("使用默认获取的路径")
			dbPath = "goForward.db"
		} else {
			dbPath = filepath.Join(filepath.Dir(executablePath), "goForward.db")
		}
	} else {
		dbPath = conf.Db
	}
	log.Printf("Data: %s", dbPath)
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalln("连接数据库失败:", err)
	}
	db.AutoMigrate(&conf.ConnectionStats{})
	db.AutoMigrate(&conf.IpBan{})
}

// Close 关闭数据库连接
func Close() {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Println("获取数据库连接失败:", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		log.Println("关闭数据库连接失败:", err)
	}
}

// 获取转发列表
func GetForwardList() []conf.ConnectionStats {
	var res []conf.ConnectionStats
	db.Model(&conf.ConnectionStats{}).Find(&res)
	return res
}

// 获取启用的转发列表
func GetAction() []conf.ConnectionStats {
	var res []conf.ConnectionStats
	db.Model(&conf.ConnectionStats{}).Where("status = ?", 0).Find(&res)
	return res
}

// 获取ipban列表
func GetIpBan() []conf.IpBan {
	var res []conf.IpBan
	db.Model(&conf.IpBan{}).Find(&res)
	return res
}

// 修改指定转发统计流量(byte)
func UpdateForwardBytes(id int, bytes uint64) bool {
	res := db.Model(&conf.ConnectionStats{}).Where("id = ?", id).Update("total_bytes", bytes)
	if res.Error != nil {
		log.Println(res.Error)
		return false
	}
	return true
}

// 修改指定转发统计流量(gigabyte)
func UpdateForwardGb(id int, gb uint64) bool {
	res := db.Model(&conf.ConnectionStats{}).Where("id = ?", id).Update("total_gigabyte", gb)
	if res.Error != nil {
		log.Println(res.Error)
		return false
	}
	return true
}

// 修改指定转发状态
func UpdateForwardStatus(id int, status int) bool {
	res := db.Model(&conf.ConnectionStats{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		log.Println(res.Error)
		return false
	}
	return true
}

// 获取指定转发内容
func GetForward(id int) conf.ConnectionStats {
	var get conf.ConnectionStats
	db.Model(&conf.ConnectionStats{}).Where("id = ?", id).Find(&get)
	return get
}

// 判断指定端口转发是否可添加
func FreeForward(localPort, protocol string) bool {
	var get conf.ConnectionStats
	res := db.Model(&conf.ConnectionStats{}).Where("local_port = ? And protocol = ?", localPort, protocol).Find(&get)
	if res.Error == nil {
		if get.Id == 0 {
			return true
		} else {
			return false
		}
	}
	return false
}

// 去掉所有空格
func rmSpaces(input string) string {
	return strings.ReplaceAll(input, " ", "")
}

// NormalizeAddr 去除空格，并去掉 IPv6 地址外层的方括号，统一以裸地址形式存储。
// 连接时配合 net.JoinHostPort 使用，后者会自动为 IPv6 补回括号。
func NormalizeAddr(addr string) string {
	addr = rmSpaces(addr)
	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		addr = addr[1 : len(addr)-1]
	}
	return addr
}

// 增加转发
func AddForward(newForward conf.ConnectionStats) int {
	// 预处理：去空格，并标准化 RemoteAddr（去掉 IPv6 方括号）
	newForward.RemoteAddr = NormalizeAddr(newForward.RemoteAddr)
	newForward.RemotePort = rmSpaces(newForward.RemotePort)
	newForward.LocalPort = rmSpaces(newForward.LocalPort)
	if newForward.Protocol != "udp" {
		newForward.Protocol = "tcp"
	}
	if !FreeForward(newForward.LocalPort, newForward.Protocol) {
		return 0
	}
	// 开启事务
	tx := db.Begin()
	if tx.Error != nil {
		log.Println("开启事务失败")
		return 0
	}
	// 在事务中执行插入操作
	if err := tx.Create(&newForward).Error; err != nil {
		log.Println("插入新转发失败:", err)
		tx.Rollback()
		return 0
	}
	// 提交事务
	tx.Commit()
	return newForward.Id
}

// 删除转发
func DelForward(id int) bool {
	if err := db.Where("id = ?", id).Delete(&conf.ConnectionStats{}).Error; err != nil {
		log.Println(err)
		return false
	}
	return true
}

// 增加错误登录
func AddBan(ip conf.IpBan) bool {
	// 开启事务
	tx := db.Begin()
	if tx.Error != nil {
		return false
	}
	// 在事务中执行插入操作
	if err := tx.Create(&ip).Error; err != nil {
		log.Println(err)
		tx.Rollback()
		return false
	}
	// 提交事务
	tx.Commit()
	return true
}

// 检查过去一天内指定IP地址的记录条数是否超过三条
func IpFree(ip string) bool {
	oneDayAgo := time.Now().Add(-24 * time.Hour).Unix()
	var count int64
	if err := db.Model(&conf.IpBan{}).Where("ip = ? AND time_stamp > ?", ip, oneDayAgo).Count(&count).Error; err != nil {
		log.Println(err)
		return false
	}
	return count < 3
}
