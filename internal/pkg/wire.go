package pkg

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/internal/pkg/dao"
)

var ProviderSet = wire.NewSet(dao.NewDatabase)
