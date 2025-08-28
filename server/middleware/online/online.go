package online

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-sphere/sphere/cache/mcache"
)

type Online struct {
	cache *mcache.Map[string, struct{}]
}

func NewOnline() *Online {
	return &Online{
		cache: mcache.NewMapCache[struct{}](),
	}
}

func (l *Online) Middleware(keygen func(ctx *gin.Context) string, ttl time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		key := keygen(ctx)
		if key != "" {
			_ = l.cache.SetWithTTL(ctx, key, struct{}{}, ttl)
		}
		ctx.Next()
	}
}

func (l *Online) OnlineCount() int {
	return l.cache.Count()
}
