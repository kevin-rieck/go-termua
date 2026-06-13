package tui

import (
	"os/exec"
	"path/filepath"
	"runtime"

	"termua/internal/config"
)

type ExportFolderOpener interface {
	OpenExportFolder(path string) error
}

type systemExportFolderOpener struct{}

func (systemExportFolderOpener) OpenExportFolder(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func exportsDir(paths config.Paths) string {
	return filepath.Join(pathOrDot(paths.CacheDir), "exports")
}
