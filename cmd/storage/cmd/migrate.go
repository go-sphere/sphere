//go:build !cdncommand

package cmd

import (
	"context"
	"github.com/tbxark/sphere/configs"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/storage"
	"github.com/tbxark/sphere/pkg/storage/qiniu"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// migrateCmd represents the cdn command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Qiniu migration Tools",
	Long:  `Move files from one qiniu bucket to another bucket.`,
	Run:   runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().StringP("files", "f", "", "list of files to move")
	migrateCmd.Flags().StringP("config", "c", "config.json", "config file path")
	migrateCmd.Flags().StringP("output", "o", "output.txt", "output file path")
	migrateCmd.Flags().StringP("storage", "s", "assets", "save directory of cdn")
	migrateCmd.Flags().BoolP("keepPath", "k", false, "keep file path")
}

func runMigrate(cmd *cobra.Command, args []string) {
	fileP := cmd.Flag("files").Value.String()
	cfgP := cmd.Flag("config").Value.String()
	outP := cmd.Flag("output").Value.String()
	dir := cmd.Flag("storage").Value.String()
	keepPath := cmd.Flag("keepPath").Value.String() == "true"

	file, err := os.ReadFile(fileP)
	if err != nil {
		log.Panicf("read file error: %v", err)
	}
	cfg, err := configs.LoadLocalConfig(cfgP)
	if err != nil {
		log.Panicf("load config error: %v", err)
	}
	upload := qiniu.NewQiniu(cfg.Storage)
	list := strings.Split(string(file), "\n")
	ctx := context.Background()
	result := make(map[string]string, len(list))
	nameBuilder := storage.DefaultKeyBuilder("")
	for _, u := range list {
		if _, exist := result[u]; exist {
			continue
		}
		if u == "" {
			continue
		}
		up, e := url.Parse(u)
		if e != nil {
			log.Errorf("parse url error: %v", e)
			continue
		}
		key := nameBuilder(up.Path, dir)
		if keepPath {
			key = strings.TrimPrefix(up.Path, "/")
		}
		resp, e := http.Get(u)
		if e != nil {
			log.Errorf("get file error: %v", e)
			continue
		}
		ret, e := upload.UploadFile(ctx, resp.Body, resp.ContentLength, key)
		if e != nil {
			log.Errorf("upload file error: %v", e)
			continue
		}
		nu := upload.GenerateURL(ret.Key)
		result[u] = nu
		log.Debugf("move file success: %s -> %s", u, nu)
	}
	resBuf := strings.Builder{}
	for _, u := range list {
		if nu, exist := result[u]; exist {
			resBuf.WriteString(u)
			resBuf.WriteString("\n -> ")
			resBuf.WriteString(nu)
			resBuf.WriteString("\n\n ")
		}
	}
	err = os.WriteFile(outP, []byte(resBuf.String()), 0644)
	if err != nil {
		log.Panicf("write output error: %v", err)
	}
}
