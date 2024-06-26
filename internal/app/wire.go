//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/internal/biz"
	"github.com/tbxark/go-base-api/internal/biz/api"
	"github.com/tbxark/go-base-api/internal/biz/dash"
	ipkg "github.com/tbxark/go-base-api/internal/pkg"
	"github.com/tbxark/go-base-api/internal/task"
	"github.com/tbxark/go-base-api/pkg"
	"github.com/tbxark/go-base-api/pkg/dao/client"
	"github.com/tbxark/go-base-api/pkg/qniu"
	"github.com/tbxark/go-base-api/pkg/wechat"
)

func NewApplication(_api *api.Config, _dash *dash.Config, _dao *client.Config, _wx *wechat.Config, _cdn *qniu.Config) (*Application, error) {
	wire.Build(pkg.ProviderSet, ipkg.ProviderSet, biz.ProviderSet, task.ProviderSet, wire.NewSet(CreateApplication))
	return &Application{}, nil
}
