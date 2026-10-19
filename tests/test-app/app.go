package testapp

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

var Module = fx.Module("test-app", fx.Provide(func(service Service, mockURL string) http.Handler {
	return NewRouter(service, mockURL)
}))

type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Service interface {
	Lookup(id string) (Item, bool)
}

func NewRouter(service Service, mockURL string) *gin.Engine {
	router := gin.New()
	router.GET("/headers", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"trace": c.GetHeader("X-Trace")})
	})

	router.POST("/echo", func(c *gin.Context) {
		var payload map[string]interface{}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"payload": payload,
			"trace":   c.GetHeader("X-Trace"),
		})
	})

	router.GET("/items/:id", func(c *gin.Context) {
		item, ok := service.Lookup(c.Param("id"))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusOK, item)
	})

	router.POST("/items/:id/notify", func(c *gin.Context) {
		item, ok := service.Lookup(c.Param("id"))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		body, err := json.Marshal(item)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot encode item"})
			return
		}
		request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, mockURL+"/events", bytes.NewReader(body))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create notification"})
			return
		}
		request.Header.Set("Content-Type", "application/json")

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "notification failed"})
			return
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			c.JSON(http.StatusBadGateway, gin.H{"error": "notification failed"})
			return
		}
		var notification map[string]interface{}
		if err := json.NewDecoder(response.Body).Decode(&notification); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "invalid notification response"})
			return
		}
		reply := gin.H{"item": item, "notification": notification}
		if header := response.Header.Get("X-Mock"); header != "" {
			reply["mock_header"] = header
		}
		c.JSON(http.StatusOK, reply)
	})

	return router
}
