package dash

import (
	"github.com/gin-gonic/gin"
	doc "github.com/tbxark/go-base-api/docs/dashboard"
	"github.com/tbxark/go-base-api/pkg/web/docs"
)

func (w *Web) bindDocRoute(r gin.IRouter) {
	docs.SetupDoc(doc.SwaggerInfoDashboard, "Dashboard", r)
}

//func (w *Web) bindDocRoute(r gin.IRouter) {
//}
