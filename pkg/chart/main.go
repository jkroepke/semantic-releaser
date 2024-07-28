package chart

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/jkroepke/semantic-releaser/pkg/changelog"
	"github.com/jkroepke/semantic-releaser/pkg/config"
	"github.com/jkroepke/semantic-releaser/pkg/helm"
	"github.com/jkroepke/semantic-releaser/pkg/utils"
	cc "github.com/leodido/go-conventionalcommits"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

var regexpVersion = regexp.MustCompile(`^version: .+$`)

type Chart struct {
	logger       zerolog.Logger
	conf         *config.Config
	repo         *git.Repository
	commitParser cc.Machine

	name           string
	chartPath      string
	currentVersion *semver.Version
}

type chartYAML struct {
	version string `yaml:"version"`
}

func New(
	logger zerolog.Logger, conf *config.Config, repo *git.Repository, commitParser cc.Machine, name string,
) (*Chart, error) {
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	chartPath := filepath.Join(conf.ChartsDir, name)

	chartYaml := filepath.Join(chartPath, "Chart.yaml")
	if _, err = worktree.Filesystem.Stat(chartYaml); err != nil {
		return nil, ErrChartYamlNotFound
	}

	chartYamlBytes, err := os.ReadFile(chartYaml)
	if err != nil {
		return nil, fmt.Errorf("failed to read Chart.yaml: %w", err)
	}

	var chartYAML chartYAML

	if err := yaml.Unmarshal(chartYamlBytes, &chartYAML); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Chart.yaml: %w", err)
	}

	version, err := semver.NewVersion(chartYAML.version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version: %w", err)
	}

	return &Chart{
		logger:         logger,
		conf:           conf,
		repo:           repo,
		commitParser:   commitParser,
		name:           name,
		currentVersion: version,
		chartPath:      chartPath,
	}, nil
}

func (c *Chart) Release(helmClient helm.Client, version semver.Version, changelogEntries *changelog.Changelog) error {
	c.logger.Info().Str("version", version.String()).Msg("releasing chart")

	if err := c.SetVersion(version); err != nil {
		return fmt.Errorf("failed to set chart version: %w", err)
	}

	if c.conf.OciRepositoryBase != "" {
		if err := helmClient.Package(c.chartPath); err != nil {
			return fmt.Errorf("failed to package chart: %w", err)
		}

		chartTgz := fmt.Sprintf("%s-%s.tgz", c.name, version.String())
		if err := helmClient.Push(c.conf.OciRepositoryBase, c.chartPath, chartTgz); err != nil {
			return fmt.Errorf("failed to push chart: %w", err)
		}
	}

	if err := c.CommitToRepository(version, changelogEntries); err != nil {
		return fmt.Errorf("failed to set chart version: %w", err)
	}

	return nil
}

func (c *Chart) CommitToRepository(version semver.Version, changelogEntries *changelog.Changelog) error {
	err := changelogEntries.WriteTo(filepath.Join(c.chartPath, "CHANGELOG.md"))
	if err != nil {
		return fmt.Errorf("failed to write changelog: %w", err)
	}

	worktree, err := c.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	_, err = worktree.Add(filepath.Join(c.chartPath, "Chart.yaml"))
	if err != nil {
		return fmt.Errorf("failed to add Chart.yaml: %w", err)
	}

	_, err = worktree.Add(filepath.Join(c.chartPath, "CHANGELOG.md"))
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

func (c *Chart) SetVersion(version semver.Version) error {
	chartYamlPath := filepath.Join(c.chartPath, "Chart.yaml")

	chartYamlBytes, err := os.ReadFile(chartYamlPath)
	if err != nil {
		return fmt.Errorf("failed to read Chart.yaml: %w", err)
	}

	chartYamlBytes = regexpVersion.ReplaceAll(chartYamlBytes, []byte("version: "+version.String()))
	if err := os.WriteFile(chartYamlPath, chartYamlBytes, 0o600); err != nil {
		return fmt.Errorf("failed to write Chart.yaml: %w", err)
	}

	return nil
}

func (c *Chart) CurrentVersion() string {
	return c.currentVersion.String()
}

func (c *Chart) getGitTag(version string) string {
	return strings.NewReplacer("{chart}", c.name, "{version}", version).Replace(c.conf.GitTagPattern)
}

//nolint:cyclop
func (c *Chart) DetectRelease() (semver.Version, *changelog.Changelog, error) {
	repoLogs, err := c.repo.Log(&git.LogOptions{
		PathFilter: func(s string) bool {
			return strings.HasPrefix(s, c.chartPath)
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

func (c *Chart) parseCommitMessage(commitMessage []byte) (cc.VersionBump, error) {
	message, err := c.commitParser.Parse(commitMessage)
	if err != nil {
		return cc.UnknownVersion, fmt.Errorf("failed to parse commit message: %w", err)
	}

	return message.VersionBump(cc.DefaultStrategy), nil
}
