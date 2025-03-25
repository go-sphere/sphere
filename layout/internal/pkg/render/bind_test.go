package render

import (
	"fmt"
	"github.com/TBXark/sphere/database/bind"
	dashv1 "github.com/TBXark/sphere/layout/api/dash/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"testing"
)

func TestGen(t *testing.T) {
	fmt.Println(bind.Gen(bind.NewGenConf(ent.Admin{}, dashv1.AdminEdit{}, ent.AdminCreate{}).WithTargetPkgName("dashv1")))
	fmt.Println(bind.Gen(bind.NewGenConf(ent.Admin{}, dashv1.AdminEdit{}, ent.AdminUpdateOne{}).WithTargetPkgName("dashv1")))
}
