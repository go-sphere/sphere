package dash

import (
	"github.com/gin-gonic/gin"
	doc "github.com/tbxark/sphere/docs/dash"
	"github.com/tbxark/sphere/pkg/web/route/docs"
)

func (w *Web) bindDocRoute(r gin.IRouter) {
	docs.SetupDoc(doc.SwaggerInfoDash, "Dash", r)
}

//func (w *Web) bindDocRoute(r gin.IRouter) {
//}
