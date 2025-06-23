package ginx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestShouldBind(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	type Params struct {
		Name  string `uri:"name"`
		Query string `form:"query"`
	}
	router.GET("/test/:name", func(c *gin.Context) {
		var obj Params
		err := ShouldBind(c, &obj, true, true, false)
		if err != nil {
			c.AbortWithStatus(400)
		} else {
			assert.Equal(t, "demo", obj.Name)
			assert.Equal(t, "example", obj.Query)
			c.AbortWithStatus(200)
		}
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test/demo?query=example", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}
