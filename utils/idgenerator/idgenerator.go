package idgenerator

import (
	"os"
	"strconv"
	"time"

	"github.com/yitter/idgenerator-go/idgen"
)

func init() {
	workerIDRaw := os.Getenv("WORKER_ID")
	workerID, err := strconv.ParseUint(workerIDRaw, 10, 16)
	if err != nil || workerID == 0 {
		workerID = 1
	}
	options := idgen.NewIdGeneratorOptions(uint16(workerID))
	options.BaseTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli()
	idgen.SetIdGenerator(options)
}

func NextId() int64 {
	return idgen.NextId()
}

func NewIdGenerator(workerID uint16) func() int64 {
	options := idgen.NewIdGeneratorOptions(workerID)
	options.BaseTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli()
	generator := idgen.NewDefaultIdGenerator(options)
	return func() int64 {
		return generator.NewLong()
	}
}
