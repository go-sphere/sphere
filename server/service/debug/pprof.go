package debug

import (
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

func SetupPProf(route gin.IRouter, prefixOptions ...string) {
	pprof.Register(route, prefixOptions...)
}
