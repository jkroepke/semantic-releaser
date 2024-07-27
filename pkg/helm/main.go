package helm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type BinaryClient struct {
	binary string
}

type Client interface {
	Package(chartPath string) error
	Push(remote, chartPath, chartTgz string) error
}

func New(helmPath string) (BinaryClient, error) {
	if helmPath != "" {
		if _, err := os.Stat(helmPath); err != nil {
			return BinaryClient{}, fmt.Errorf("helm binary not found: %w", err)
		}

		return BinaryClient{binary: helmPath}, nil
	}

	if _, err := exec.LookPath("helm"); err == nil {
		return BinaryClient{}, fmt.Errorf("helm not found: %w", err)
	}

	return BinaryClient{binary: "helm"}, nil
}

func (c BinaryClient) Package(chartPath string) error {
	var stdout, stderr bytes.Buffer

	cmd := exec.Command(c.binary, "package", ".") //nolint:gosec
	cmd.Env = os.Environ()
	cmd.Dir = chartPath
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to package chart: %w\n\nSTDOUT:\n\n%s\n\nSTDERR:\n\n%s", err, stdout.String(), stderr.String())
	}

	return nil
}

func (c BinaryClient) Push(remote, chartPath, chartTgz string) error {
	var stdout, stderr bytes.Buffer

	if !strings.HasPrefix(remote, "oci://") {
		remote = "oci://" + remote
	}

	cmd := exec.Command(c.binary, "push", chartTgz, remote) //nolint:gosec
	cmd.Env = os.Environ()
	cmd.Dir = chartPath
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push chart: %w\n\nSTDOUT:\n\n%s\n\nSTDERR:\n\n%s", err, stdout.String(), stderr.String())
	}

	return nil
}
