package create

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TBXark/sphere/cmd/sphere-cli/internal/renamer"
	"github.com/TBXark/sphere/cmd/sphere-cli/internal/zip"
)

const (
	sphereModule                = "github.com/TBXark/sphere"
	defaultProjectLayout        = "https://github.com/TBXark/sphere/archive/refs/heads/master.zip"
	defaultProjectLayoutModName = "github.com/TBXark/sphere/layout"
	defaultLayoutPath           = "sphere-master/layout"
)

func Project(name, mod string) error {
	targetDir, err := filepath.Abs(filepath.Join(".", name))
	if err != nil {
		return err
	}

	// download and unzip the default project layout
	tempDir, err := zip.DownloadAndUnzip(defaultProjectLayout)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()
	layoutDir := filepath.Join(tempDir, defaultLayoutPath)

	// init git repository
	err = initGitRepo(layoutDir)
	if err != nil {
		return err
	}

	// rename the Go module name
	err = renameGoModule(defaultProjectLayoutModName, mod, layoutDir)
	if err != nil {
		return err
	}

	// Move the layout to the target directory
	err = moveTempDirToTarget(layoutDir, targetDir)
	if err != nil {
		return err
	}

	return nil
}

func moveTempDirToTarget(source, target string) error {
	err := os.Rename(source, target)
	if err != nil {
		return err
	}
	return nil
}

func initGitRepo(target string) error {
	return execCommands(target,
		[]string{"git", "init"},
		[]string{"git", "add", "."},
		[]string{"git", "commit", "-m", "feat: Initial commit"},
	)
}

func renameGoModule(oldModName, newModName, target string) error {
	err := execCommands(target,
		[]string{"go", "mod", "edit", "-module", newModName},
		[]string{"go", "mod", "edit", "-dropreplace", sphereModule},
	)
	if err != nil {
		return err
	}
	log.Printf("rename module: %s -> %s", oldModName, newModName)
	err = renamer.RenameDirModule(oldModName, newModName, target)
	if err != nil {
		return err
	}
	err = execCommands(target,
		[]string{"go", "get", sphereModule + "@latest"},
		[]string{"go", "mod", "tidy"},
		[]string{"go", "fmt", "./..."},
	)
	if err != nil {
		return err
	}
	files := []string{
		"buf.gen.yaml",
	}
	for _, file := range files {
		e := replaceFileContent(oldModName, newModName, filepath.Join(target, file))
		if e != nil {
			return e
		}
	}
	_, err = execCommand(target, "make", "init")
	if err != nil {
		log.Printf("make init failed: %v", err)
	}
	return nil
}

func execCommand(dir string, name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = dir
	var stdout strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	return stdout.String(), cmd.Run()
}

func execCommands(dir string, commands ...[]string) error {
	for _, cmd := range commands {
		_, err := execCommand(dir, cmd[0], cmd[1:]...)
		if err != nil {
			return err
		}
	}
	return nil
}

func replaceFileContent(old, new, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	replacer := strings.NewReplacer(old, new)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	_, err = replacer.WriteString(file, string(content))
	return err
}
