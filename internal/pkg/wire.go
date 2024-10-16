package pkg

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/internal/pkg/dao"
	"github.com/tbxark/go-base-api/internal/pkg/database/client"
)

var ProviderSet = wire.NewSet(dao.NewDao, client.NewDataBaseClient)
