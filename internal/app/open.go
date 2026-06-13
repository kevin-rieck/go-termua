package app

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// OpenFolder opens a local folder in the platform file manager.
func OpenFolder(path string) error {
	if path == "" {
		return fmt.Errorf("folder path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a folder: %s", path)
	}

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
