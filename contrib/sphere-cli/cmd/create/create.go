package create

import (
	"flag"
	"github.com/TBXark/sphere/contrib/sphere-cli/internal/command"
	"github.com/TBXark/sphere/contrib/sphere-cli/internal/renamer"
	"github.com/TBXark/sphere/contrib/sphere-cli/internal/zip"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	sphereModule                = "github.com/TBXark/sphere"
	defaultProjectLayout        = "https://github.com/TBXark/sphere/archive/refs/heads/master.zip"
	defaultProjectLayoutModName = "github.com/TBXark/sphere/layout"
)

func NewCommand() *command.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	name := fs.String("name", "", "project name")
	mod := fs.String("mod", "", "go module name")
	return command.NewCommand(fs, func() error {
		if *name == "" || *mod == "" {
			fs.Usage()
			return nil
		}
		return createProject(*name, *mod)
	})
}

func createProject(name, mod string) error {
	tempDir, err := cloneLayoutDir(defaultProjectLayout)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()
	tempLayout := filepath.Join(tempDir, "sphere-master", "layout")
	err = initGitRepo(tempLayout)
	if err != nil {
		return err
	}
	err = renameGoModule(defaultProjectLayoutModName, mod, tempLayout)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(filepath.Join(".", name))
	if err != nil {
		return err
	}
	err = moveTempDirToTarget(tempLayout, target)
	if err != nil {
		return err
	}
	return nil
}

func cloneLayoutDir(uri string) (string, error) {
	tempDir, err := zip.UnzipToTemp(uri)
	if err != nil {
		return "", err
	}
	return tempDir, nil
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
	defer file.Close()
	_, err = replacer.WriteString(file, string(content))
	return err
}
