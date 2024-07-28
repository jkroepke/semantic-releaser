package project

import (
	"github.com/go-git/go-git/v5"
	"github.com/jkroepke/semantic-releaser/pkg/config"
	cc "github.com/leodido/go-conventionalcommits"
	"github.com/rs/zerolog"
)

type Project struct {
	name        string
	projectPath string
	config      Config

	logger       zerolog.Logger
	conf         *config.Config
	repo         *git.Repository
	commitParser cc.Machine
}

type Config struct {
	Commands ConfigCommands `yaml:"commands"`
}

type ConfigCommands struct {
	SetNewVersion     string `yaml:"setNewVersion"`
	PublishNewVersion string `yaml:"publishNewVersion"`
}
