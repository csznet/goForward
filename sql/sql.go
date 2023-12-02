package sql

import (
	"fmt"
	"log"

	"csz.net/goForward/conf"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 定义数据库指针
var db *gorm.DB

func init() {
	var err error
	db, err = gorm.Open(sqlite.Open("goForward.db"), &gorm.Config{})
	if err != nil {
		log.Println("连接数据库失败")
		return
	}
	db.AutoMigrate(&conf.ConnectionStats{})
}

// 获取转发列表
func GetForwardList() []conf.ConnectionStats {
	var res []conf.ConnectionStats
	db.Model(&conf.ConnectionStats{}).Find(&res)
	return res
}

// 修改指定转发统计流量
func UpdateForwardBytes(id int, bytes uint64) bool {
	res := db.Model(&conf.ConnectionStats{}).Where("id = ?", id).Update("total_bytes", bytes)
	if res.Error != nil {
		fmt.Println(res.Error)
		return false
	}
	return true
}

// 判断指定端口转发是否可添加
func freeForward(localPort string) bool {
	var get conf.ConnectionStats
	res := db.Model(&conf.ConnectionStats{}).Where("local_port = ?", localPort).Find(&get)
	if res.Error == nil {
		if get.Id == 0 {
			return true
		} else {
			return false
		}
	}
	return false
}

// 增加转发
func AddForward(newForward conf.ConnectionStats) int {
	if !freeForward(newForward.LocalPort) {
		return 0
	}
	//开启事务
	if newForward.Protocol != "udp" {
		newForward.Protocol = "tcp"
	}
	tx := db.Begin()
	if tx.Error != nil {
		log.Println("开启事务失败")
		return 0
	}
	// 在事务中执行插入操作
	if err := tx.Create(&newForward).Error; err != nil {
		log.Println("插入新转发失败")
		log.Println(err)
		tx.Rollback() // 回滚事务
		return 0
	}
	// 提交事务
	tx.Commit()
	return newForward.Id
}

// 删除转发
func DelForward(port string) bool {
	if freeForward(port) {
		return false
	}
	if err := db.Where("local_port = ?", port).Delete(&conf.ConnectionStats{}).Error; err != nil {
		log.Println(err)
		return false
	}
	return true
}
