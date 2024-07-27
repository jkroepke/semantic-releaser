package main

import (
	"errors"
	"flag"
	"os"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/jkroepke/helm-charts-semantic-releaser/pkg/config"
	"github.com/jkroepke/helm-charts-semantic-releaser/pkg/releaser"
	"github.com/leodido/go-conventionalcommits"
	"github.com/leodido/go-conventionalcommits/parser"
	"github.com/rs/zerolog"
)

func main() {
	os.Exit(run(os.Args, os.Stdout))
}

func run(args []string, logWriter *os.File) int {
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	logger := zerolog.New(output).With().Timestamp().Logger()

	conf := config.New()
	if err := conf.Load(args, logWriter); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}

		logger.Err(err).Msg("failed to load config")

		return 1
	}

	commitParser := parser.NewMachine(parser.WithTypes(conventionalcommits.TypesConventional))
	commitParser.WithBestEffort()

	rootFS := osfs.New(".")

	repo, err := git.Open(filesystem.NewStorage(rootFS, nil), rootFS)
	if err != nil {
		logger.Err(err).Msg("failed to open git repository")

		return 1
	}

	chartReleaser := releaser.New(logger, conf, repo, commitParser)
	if err := chartReleaser.Run(); err != nil {
		logger.Err(err).Msg("failed to run releaser")

		return 1
	}

	return 0
}
