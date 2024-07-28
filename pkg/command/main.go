package command

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func Run(command string, dir string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/C", command)
	default:
		cmd = exec.Command("sh", "-c", command)
	}

	var stdout, stderr bytes.Buffer

	cmd.Env = os.Environ()
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to package project: %w\n\nSTDOUT:\n\n%s\n\nSTDERR:\n\n%s", err, stdout.String(), stderr.String())
	}

	return nil
}
