package config

import (
	"flag"
	"fmt"
	"io"
)

type Config struct {
	ChartsDir         string
	CreateGitTag      bool
	OciRepositoryBase string
	HelmBinaryPath    string
}

func New() *Config {
	return &Config{
		ChartsDir: "charts",
	}
}

func (c *Config) Load(args []string, logWriter io.Writer) error {
	flagSet := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flagSet.SetOutput(logWriter)
	flagSet.StringVar(&c.ChartsDir,
		"charts-dir",
		lookupEnvOrString("CHARTS_DIR", c.ChartsDir),
		"Location of charts",
	)

	flagSet.BoolVar(&c.CreateGitTag,
		"create-git-tag",
		lookupEnvOrBool("CREATE_GIT_TAG", c.CreateGitTag),
		"Create git tags for each release",
	)

	flagSet.StringVar(&c.OciRepositoryBase,
		"oci-repo-base",
		lookupEnvOrString("OCI_REPO_BASE", c.OciRepositoryBase),
		"If set, push charts to this OCI repository",
	)

	flagSet.StringVar(&c.HelmBinaryPath,
		"helm-binary-path",
		lookupEnvOrString("HELM_BINARY_PATH", c.HelmBinaryPath),
		"Path to the helm binary",
	)

	if err := flagSet.Parse(args[1:]); err != nil {
		return fmt.Errorf("error parsing cli args: %w", err)
	}

	return nil
}
