package web

import (
	"net/http"

	"csz.net/goForward/conf"
	"csz.net/goForward/sql"
	"csz.net/goForward/utils"
	"github.com/gin-gonic/gin"
)

func Run() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"forwardList": sql.GetForwardList(),
		})
	})
	r.POST("/add", func(c *gin.Context) {
		if c.PostForm("localPort") != "" && c.PostForm("remoteAddr") != "" && c.PostForm("remotePort") != "" && c.PostForm("protocol") != "" {
			f := conf.ConnectionStats{
				LocalPort:  c.PostForm("localPort"),
				RemotePort: c.PostForm("remotePort"),
				RemoteAddr: c.PostForm("remoteAddr"),
				Protocol:   c.PostForm("protocol"),
			}
			if utils.AddForward(f) {
				c.HTML(200, "msg.tmpl", gin.H{
					"msg": "添加成功",
					"suc": true,
				})
			} else {
				c.HTML(200, "msg.tmpl", gin.H{
					"msg": "添加失败",
					"suc": false,
				})
			}
		} else {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "添加失败，表单信息不完整",
				"suc": false,
			})
		}
	})
	r.GET("/del/:port", func(c *gin.Context) {
		port := c.Param("port")
		if port != "" {
			if utils.DelForward(port) {
				c.HTML(200, "msg.tmpl", gin.H{
					"msg": "删除成功",
					"suc": true,
				})
			} else {
				c.HTML(200, "msg.tmpl", gin.H{
					"msg": "删除失败",
					"suc": false,
				})
			}
		} else {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "删除失败",
				"suc": false,
			})
		}
	})
	r.Run(":8889")
}
