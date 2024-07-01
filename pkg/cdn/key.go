package cdn

import (
	"crypto/md5"
	"encoding/hex"
	"path"
	"strconv"
	"time"
)

func DefaultKeyBuilder(prefix string) func(fileName string, dir ...string) string {
	return func(fileName string, dir ...string) string {
		fileExt := path.Ext(fileName)
		sum := md5.Sum([]byte(fileName))
		nameMd5 := hex.EncodeToString(sum[:])
		name := strconv.Itoa(int(time.Now().Unix())) + "_" + nameMd5 + fileExt
		if prefix != "" {
			name = prefix + "_" + name
		}
		return path.Join(path.Join(dir...), name)
	}
}

func KeepFileNameKeyBuilder() func(fileName string, dir ...string) string {
	return func(fileName string, dir ...string) string {
		sum := md5.Sum([]byte(fileName))
		nameMd5 := hex.EncodeToString(sum[:])
		name := strconv.Itoa(int(time.Now().Unix())) + "_" + nameMd5
		return path.Join(path.Join(dir...), name, fileName)
	}
}
