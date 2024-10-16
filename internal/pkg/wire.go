package pkg

import (
	"github.com/google/wire"
	"github.com/tbxark/sphere/internal/pkg/dao"
	"github.com/tbxark/sphere/internal/pkg/database/client"
)

var ProviderSet = wire.NewSet(dao.NewDao, client.NewDataBaseClient)
