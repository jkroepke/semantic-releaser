package config

import (
	"flag"
	"fmt"
	"io"
)

type Config struct {
	ProjectsDir       string
	ConfigFilePath    string
	GitTagPattern     string
	GenerateChangelog bool
	GitWriteBack      bool
}

func New() *Config {
	return &Config{
		ConfigFilePath:    ".releaser.yaml",
		GenerateChangelog: true,
		GitTagPattern:     "{project}/{version}",
		ProjectsDir:       "charts",
	}
}

func (c *Config) Load(args []string, logWriter io.Writer) error {
	flagSet := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flagSet.SetOutput(logWriter)
	flagSet.StringVar(&c.ProjectsDir,
		"projects-dir",
		lookupEnvOrString("PROJECTS_DIR", c.ProjectsDir),
		"Directory containing the projects. Each project should be in a subdirectory. If set to '.', the current directory will be used.",
	)

	flagSet.StringVar(&c.ConfigFilePath,
		"config-file-path",
		lookupEnvOrString("CONFIG_FILE_PATH", c.ConfigFilePath),
		"Path to config file relative to the project directory,",
	)

	flagSet.StringVar(&c.GitTagPattern,
		"git-tag-pattern",
		lookupEnvOrString("GIT_TAG_PATTERN", c.GitTagPattern),
		"Pattern for git tags. Use {project} and {version} as placeholders.",
	)

	flagSet.BoolVar(&c.GitWriteBack,
		"git-write-back",
		lookupEnvOrBool("GIT_WRITE_BACK", c.GitWriteBack),
		"If enabled, changes on local files will be commit back to git repository.",
	)

	flagSet.BoolVar(&c.GenerateChangelog,
		"generate-changelog",
		lookupEnvOrBool("GENERATE_CHANGELOG", c.GenerateChangelog),
		"If enabled, changes on local files will be commit back to git repository.",
	)

	if err := flagSet.Parse(args[1:]); err != nil {
		return fmt.Errorf("error parsing cli args: %w", err)
	}

	return nil
}
