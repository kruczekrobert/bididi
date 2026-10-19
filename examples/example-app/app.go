package exampleapp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

var Module = fx.Module("example-app",
	fx.Provide(
		func() http.Handler {
			return NewRouter()
		},
	))

func NewRouter() *gin.Engine {
	router := gin.New()
	router.GET("/hello/:name", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "hello " + c.Param("name")})
	})
	return router
}
