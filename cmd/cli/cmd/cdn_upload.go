//go:build !cdncommand

package cmd

import (
	"context"
	"github.com/spf13/cobra"
	"github.com/tbxark/go-base-api/cmd/cli/config"
	"github.com/tbxark/go-base-api/pkg/cdn"
	"github.com/tbxark/go-base-api/pkg/cdn/qiniu"
	"github.com/tbxark/go-base-api/pkg/log"
	"os"
	"path/filepath"
	"strings"
)

// cdnUploadCmd represents the upload command
var cdnUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload files to Qiniu",
	Long:  `Upload files to Qiniu CDN.`,
	Run:   runCdnUpload,
}

func init() {
	cdnCmd.AddCommand(cdnUploadCmd)
	cdnUploadCmd.Flags().StringP("files", "f", "", "directory of files to upload")
	cdnUploadCmd.Flags().StringP("config", "c", "config.json", "config file path")
	cdnUploadCmd.Flags().StringP("output", "o", "output.txt", "output file path")
	cdnUploadCmd.Flags().StringP("storage", "s", "assets", "save directory of cdn")
}

func runCdnUpload(cmd *cobra.Command, args []string) {
	fileP := cmd.Flag("files").Value.String()
	cfgP := cmd.Flag("config").Value.String()
	outP := cmd.Flag("output").Value.String()
	dir := cmd.Flag("storage").Value.String()

	cfg, err := config.LoadConfig(cfgP)
	if err != nil {
		log.Panicf("load config error: %v", err)
	}

	upload := qiniu.NewQiniu(cfg.CDN)
	ctx := context.Background()
	resBuf := strings.Builder{}
	nameBuilder := cdn.KeepFileNameKeyBuilder()
	err = filepath.Walk(fileP, func(path string, info os.FileInfo, err error) error {
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
		resBuf.WriteString(upload.RenderURL(ret.Key))
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
