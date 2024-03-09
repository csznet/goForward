package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"csz.net/goForward/assets"
	"csz.net/goForward/conf"
	"csz.net/goForward/sql"
	"csz.net/goForward/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func Run() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("goForward", store))
	r.Use(checkCookieMiddleware)
	r.SetHTMLTemplate(template.Must(template.New("").Funcs(r.FuncMap).ParseFS(assets.Templates, "templates/*")))
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"forwardList": sql.GetForwardList(),
		})
	})
	r.GET("/ban", func(c *gin.Context) {
		c.JSON(200, sql.GetIpBan())
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
					"msg": "添加失败，端口已占用",
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
	r.GET("/do/:id", func(c *gin.Context) {
		id := c.Param("id")
		intID, err := strconv.Atoi(id)
		f := sql.GetForward(intID)
		status := false
		if err == nil {
			if f.Status == 0 {
				f.Status = 1
				if len(sql.GetAction()) == 1 {
					c.HTML(200, "msg.tmpl", gin.H{
						"msg": "停止失败，请确保有至少一个转发在运行",
						"suc": false,
					})
					return
				}
			} else {
				f.Status = 0
			}
			status = utils.ExStatus(f)
		}
		if status {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "操作成功",
				"suc": true,
			})
			return
		} else {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "操作失败",
				"suc": false,
			})
			return
		}
	})
	r.GET("/del/:id", func(c *gin.Context) {
		id := c.Param("id")
		intID, err := strconv.Atoi(id)
		f := sql.GetForward(intID)
		if err != nil {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "删除失败,ID错误",
				"suc": false,
			})
			return
		}
		if len(sql.GetForwardList()) == 1 {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "删除失败，请确保有至少一个转发在运行",
				"suc": false,
			})
			return
		}
		if f.Id != 0 && utils.DelForward(f) {
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
	})
	r.GET("/pwd", func(c *gin.Context) {
		c.HTML(200, "pwd.tmpl", nil)
	})
	r.POST("/pwd", func(c *gin.Context) {
		if !sql.IpFree(c.ClientIP()) {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "IP is Ban",
				"suc": false,
			})
			return
		}
		session := sessions.Default(c)
		session.Set("p", c.PostForm("p"))
		// 设置session的过期时间为1天
		session.Options(sessions.Options{MaxAge: 86400})
		session.Save()
		if c.PostForm("p") != conf.WebPass {
			ban := conf.IpBan{
				Ip:        c.ClientIP(),
				TimeStamp: time.Now().Unix(),
			}
			sql.AddBan(ban)
		}
		c.Redirect(302, "/")
	})
	fmt.Println("Web管理面板端口:" + conf.WebPort)
	r.Run("0.0.0.0:" + conf.WebPort)
}

// 密码验证中间件
func checkCookieMiddleware(c *gin.Context) {
	currenPath := c.Request.URL.Path
	if conf.WebPass != "" && currenPath != "/pwd" {
		session := sessions.Default(c)
		pass := session.Get("p")
		if pass != conf.WebPass {
			c.Redirect(http.StatusFound, "/pwd")
			c.Abort()
			return
		}
	}
	// 继续处理请求
	c.Next()
}
