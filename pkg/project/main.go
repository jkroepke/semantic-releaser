package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/jkroepke/semantic-releaser/pkg/changelog"
	"github.com/jkroepke/semantic-releaser/pkg/config"
	"github.com/jkroepke/semantic-releaser/pkg/utils"
	cc "github.com/leodido/go-conventionalcommits"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

func New(
	logger zerolog.Logger, conf *config.Config, repo *git.Repository, commitParser cc.Machine, name string,
) (*Project, error) {
	project := &Project{
		logger:       logger,
		conf:         conf,
		repo:         repo,
		commitParser: commitParser,
		name:         name,
		projectPath:  filepath.Join(conf.ProjectsDir, name),
	}

	if err := project.ReadProjectConfig(); err != nil {
		return nil, fmt.Errorf("failed to read project config: %w", err)
	}

	return project, nil
}

// ReadProjectConfig reads the project configuration from the project config file.
func (c *Project) ReadProjectConfig() error {
	worktree, err := c.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	projectConfig := filepath.Join(c.projectPath, c.conf.ConfigFilePath)

	configContent, err := worktree.Filesystem.Open(projectConfig)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrProjectFileNotFound
		}

		return fmt.Errorf("failed to read %s: %w", c.conf.ConfigFilePath, err)
	}

	if err = yaml.NewDecoder(configContent).Decode(&c.config); err != nil {
		return fmt.Errorf("failed to YAML decode %s: %w", c.conf.ConfigFilePath, err)
	}

	return nil
}

func (c *Project) Release(version semver.Version, changelogEntries *changelog.Changelog) error {
	c.logger.Info().Str("version", version.String()).Msg("releasing project")

	if err := c.SetVersion(version); err != nil {
		return fmt.Errorf("failed to set project version: %w", err)
	}

	if err := c.CommitToRepository(version, changelogEntries); err != nil {
		return fmt.Errorf("failed to commit to repository: %w", err)
	}

	return nil
}

func (c *Project) CommitToRepository(version semver.Version, changelogEntries *changelog.Changelog) error {
	worktree, err := c.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	file, err := worktree.Filesystem.Open(filepath.Join(c.projectPath, "CHANGELOG.md"))
	if err != nil {
		return fmt.Errorf("failed to open changelog: %w", err)
	}
	defer file.Close()

	err = changelogEntries.WriteTo(file)
	if err != nil {
		return fmt.Errorf("failed to write changelog: %w", err)
	}

	_, err = worktree.Add(filepath.Join(c.projectPath, "CHANGELOG.md"))
	if err != nil {
		return fmt.Errorf("failed to add CHANGELOG.md: %w", err)
	}

	changelogSummarize := ""
	if changelogEntries != nil {
		changelogSummarize = "\n\n" + changelogEntries.String()
	}

	commitMessage := fmt.Sprintf("chore(%s): release %s [skip ci]%s", c.name, version.String(), changelogSummarize)

	commit, err := worktree.Commit(commitMessage, &git.CommitOptions{
		AllowEmptyCommits: false,
	})
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	_, err = c.repo.CreateTag(c.getGitTag(version.String()), commit, nil)
	if err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	err = c.repo.Push(&git.PushOptions{
		FollowTags: true,
	})
	if err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	return nil
}

func (c *Project) SetVersion(version semver.Version) error {
	chartYamlPath := filepath.Join(c.projectPath, "Project.yaml")

	chartYamlBytes, err := os.ReadFile(chartYamlPath)
	if err != nil {
		return fmt.Errorf("failed to read Project.yaml: %w", err)
	}

	chartYamlBytes = regexpVersion.ReplaceAll(chartYamlBytes, []byte("version: "+version.String()))
	if err := os.WriteFile(chartYamlPath, chartYamlBytes, 0o600); err != nil {
		return fmt.Errorf("failed to write Project.yaml: %w", err)
	}

	return nil
}

func (c *Project) CurrentVersion() string {
	return c.currentVersion.String()
}

func (c *Project) getGitTag(version string) string {
	return strings.NewReplacer("{project}", c.name, "{version}", version).Replace(c.conf.GitTagPattern)
}

//nolint:cyclop
func (c *Project) DetectRelease() (semver.Version, *changelog.Changelog, error) {
	repoLogs, err := c.repo.Log(&git.LogOptions{
		PathFilter: func(s string) bool {
			return strings.HasPrefix(s, c.projectPath)
		},
	})
	if err != nil {
		return semver.Version{}, nil, fmt.Errorf("failed to get log: %w", err)
	}

	changelogEntries := changelog.New()
	changelogEntries.SetOldVersion(c.currentVersion.String())

	if remote, err := c.repo.Remote("origin"); err == nil {
		changelogEntries.SetRemote(remote.Config().URLs[0])
	}

	bump := cc.UnknownVersion

	tagCommitHash := ""

	tag, err := c.repo.Tag(c.getGitTag(c.currentVersion.String()))
	if err == nil {
		tagCommitHash = tag.Hash().String()
	}

	for log, err := repoLogs.Next(); err == nil; _, err = repoLogs.Next() {
		commitHash := log.Hash.String()
		if tagCommitHash == commitHash {
			break
		}

		// parse only the first line of the message
		log.Message, _, _ = strings.Cut(log.Message, "\n")

		commitVersionBump, _ := c.parseCommitMessage([]byte(log.Message))
		commitHash = commitHash[:7]

		switch commitVersionBump {
		case cc.MajorVersion:
			bump = cc.MajorVersion

			changelogEntries.AddBreaking(log.Message, commitHash)
			c.logger.Info().Str("message", log.Message).Msg("MAJOR")
		case cc.MinorVersion:
			if bump != cc.MajorVersion {
				bump = cc.MinorVersion
			}

			changelogEntries.AddFeature(log.Message, commitHash)
			c.logger.Info().Str("message", log.Message).Msg("MINOR")
		case cc.PatchVersion:
			if bump == cc.UnknownVersion {
				bump = cc.PatchVersion
			}

			changelogEntries.AddFix(log.Message, commitHash)
			c.logger.Info().Str("message", log.Message).Msg("PATCH")
		case cc.UnknownVersion:
			c.logger.Info().Str("message", log.Message).Msg("SKIP")
		}
	}

	if bump == cc.UnknownVersion {
		return *c.currentVersion, changelogEntries, nil
	}

	version := utils.IncrementSemVerVersion(c.currentVersion, bump)

	changelogEntries.SetNewVersion(version.String())
	c.logger.Info().Str("version", version.String()).Msg("commits detected")

	return version, changelogEntries, nil
}

func (c *Project) parseCommitMessage(commitMessage []byte) (cc.VersionBump, error) {
	message, err := c.commitParser.Parse(commitMessage)
	if err != nil {
		return cc.UnknownVersion, fmt.Errorf("failed to parse commit message: %w", err)
	}

	return message.VersionBump(cc.DefaultStrategy), nil
}
