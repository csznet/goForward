package web

import (
	"net/http"

	"csz.net/goForward/sql"
	"github.com/gin-gonic/gin"
)

func Run() {
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"forwardList": sql.GetForwardList(),
		})
	})
	r.Run(":8000")
}
