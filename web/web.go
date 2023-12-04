package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"csz.net/goForward/assets"
	"csz.net/goForward/conf"
	"csz.net/goForward/sql"
	"csz.net/goForward/utils"
	"github.com/gin-gonic/gin"
)

func Run() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(checkCookieMiddleware)
	r.SetHTMLTemplate(template.Must(template.New("").Funcs(r.FuncMap).ParseFS(assets.Templates, "templates/*")))
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
					"msg": "添加失败，本地端口正在转发",
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
		c.SetCookie("p", c.PostForm("p"), 0, "/", c.Request.Host, false, true)
		c.Redirect(302, "/")
	})
	fmt.Println("Web管理面板端口:" + conf.WebPort)
	r.Run("0.0.0.0:" + conf.WebPort)
}

// 密码验证中间件
func checkCookieMiddleware(c *gin.Context) {
	cookie, err := c.Cookie("p")
	currenPath := c.Request.URL.Path
	if conf.WebPass != "" && currenPath != "/pwd" {
		if err != nil || cookie != conf.WebPass {
			c.Redirect(http.StatusFound, "/pwd")
			c.Abort()
			return
		}
	}
	// 继续处理请求
	c.Next()
}

// 提取路径的第一个部分作为一级目录
func getFirstLevelDir(path string) string {
	// 使用 strings.Split 将路径分割为多个部分
	parts := strings.Split(path, "/")

	// 如果路径包含多个部分，返回第一个部分，否则返回整个路径
	if len(parts) > 1 {
		return parts[1]
	}
	return path
}
