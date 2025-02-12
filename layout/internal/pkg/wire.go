package pkg

import (
	"github.com/TBXark/sphere/layout/internal/pkg/dao"
	"github.com/TBXark/sphere/layout/internal/pkg/database/client"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(dao.NewDao, client.NewDataBaseClient)
