package cors

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Setup configures CORS middleware for the given router with specified allowed origins.
// It sets up permissive CORS settings allowing common HTTP methods and headers
// with credentials support and a 12-hour max age for preflight requests.
func Setup(route gin.IRouter, origins []string) {
	route.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
}
