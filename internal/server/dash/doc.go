package dash

import (
	"github.com/gin-gonic/gin"
	doc "github.com/tbxark/go-base-api/docs/dash"
	"github.com/tbxark/go-base-api/pkg/web/docs"
)

func (w *Web) bindDocRoute(r gin.IRouter) {
	docs.SetupDoc(doc.SwaggerInfoDash, "Dash", r)
}

//func (w *Web) bindDocRoute(r gin.IRouter) {
//}
