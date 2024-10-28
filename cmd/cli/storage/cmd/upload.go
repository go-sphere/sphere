//go:build !cdncommand

package cmd

import (
	"context"
	"github.com/spf13/cobra"
	"github.com/tbxark/sphere/internal/config"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/storage"
	"github.com/tbxark/sphere/pkg/storage/qiniu"
	"os"
	"path/filepath"
	"strings"
)

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload files to storage",
	Long:  `Upload files to Qiniu storage.`,
	Run:   runUpload,
}

func init() {
	rootCmd.AddCommand(uploadCmd)
	uploadCmd.Flags().StringP("files", "f", "", "directory of files to upload")
	uploadCmd.Flags().StringP("config", "c", "config.json", "config file path")
	uploadCmd.Flags().StringP("output", "o", "output.txt", "output file path")
	uploadCmd.Flags().StringP("storage", "s", "assets", "save directory of cdn")
}

func runUpload(cmd *cobra.Command, args []string) {
	fileP := cmd.Flag("files").Value.String()
	confP := cmd.Flag("config").Value.String()
	outP := cmd.Flag("output").Value.String()
	dir := cmd.Flag("storage").Value.String()

	cfg, err := config.NewConfig(confP)
	if err != nil {
		log.Panicf("load config error: %v", err)
	}

	upload := qiniu.NewQiniu(cfg.Storage)
	ctx := context.Background()
	resBuf := strings.Builder{}
	nameBuilder := storage.KeepFileNameKeyBuilder()
	err = filepath.Walk(fileP, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Errorf("walk file error: %v", err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		key := nameBuilder(info.Name(), dir)
		ret, err := upload.UploadLocalFile(ctx, path, key)
		if err != nil {
			log.Errorf("upload file error: %v", err)
			return nil
		}
		log.Debugf("upload file success: %s -> %s", path, ret.Key)
		resBuf.WriteString(info.Name())
		resBuf.WriteString("\n -> ")
		resBuf.WriteString(upload.GenerateURL(ret.Key))
		resBuf.WriteString("\n\n")
		return nil
	})
	if err != nil {
		log.Panicf("walk file error: %v", err)
	}
	err = os.WriteFile(outP, []byte(resBuf.String()), 0644)
	if err != nil {
		log.Panicf("write output file error: %v", err)
	}
}
