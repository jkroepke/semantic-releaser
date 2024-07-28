package project

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/jkroepke/semantic-releaser/pkg/changelog"
	"github.com/jkroepke/semantic-releaser/pkg/command"
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
		logger:         logger,
		conf:           conf,
		repo:           repo,
		commitParser:   commitParser,
		name:           name,
		projectPath:    filepath.Join(conf.ProjectsDir, name),
		currentVersion: semver.New(0, 0, 0, "", ""),
	}

	if err := project.readProjectConfig(); err != nil {
		return nil, fmt.Errorf("failed to read project config: %w", err)
	}

	if err := project.readCurrentVersion(); err != nil {
		return nil, fmt.Errorf("failed to read current version: %w", err)
	}

	return project, nil
}

func (c *Project) CurrentVersion() string {
	return c.currentVersion.String()
}

func (c *Project) Release(version semver.Version, changelogEntries *changelog.Changelog) error {
	c.logger.Info().Str("version", version.String()).Msg("releasing project")

	if err := c.setVersion(version); err != nil {
		return fmt.Errorf("failed to set version: %w", err)
	}

	if err := c.commitToRepository(version, changelogEntries); err != nil {
		return fmt.Errorf("failed to commit to repository: %w", err)
	}

	if err := c.publish(version); err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	return nil
}

func (c *Project) setVersion(version semver.Version) error {
	if c.config.Commands.SetNewVersion == "" {
		return nil
	}

	tmpl, err := template.New("publish").Parse(c.config.Commands.SetNewVersion)
	if err != nil {
		return fmt.Errorf("failed to parse set version command template: %w", err)
	}

	var buf bytes.Buffer

	if err = tmpl.Execute(&buf, map[string]string{
		"nextVersion": version.String(),
		"projectName": c.name,
		"projectPath": c.projectPath,
	}); err != nil {
		return fmt.Errorf("failed to execute set version command template: %w", err)
	}

	if err = command.Run(buf.String(), c.projectPath); err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	return nil
}

// readProjectConfig reads the project configuration from the project config file.
func (c *Project) readProjectConfig() error {
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

func (c *Project) publish(version semver.Version) error {
	if c.config.Commands.Publish == "" {
		return nil
	}

	tmpl, err := template.New("publish").Parse(c.config.Commands.Publish)
	if err != nil {
		return fmt.Errorf("failed to parse publish command template: %w", err)
	}

	var buf bytes.Buffer

	if err = tmpl.Execute(&buf, map[string]string{
		"nextVersion": version.String(),
		"projectName": c.name,
		"projectPath": c.projectPath,
	}); err != nil {
		return fmt.Errorf("failed to execute publish command template: %w", err)
	}

	if err = command.Run(buf.String(), c.projectPath); err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	return nil
}

// readCurrentVersion reads the current version from the git repository file.
func (c *Project) readCurrentVersion() error {
	tags, err := c.repo.Tags()
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	tagRegex := strings.NewReplacer(`\{project\}`, c.name, `\{version\}`, "(.*)").
		Replace(regexp.QuoteMeta(c.conf.GitTagPattern))

	regTagPattern, err := regexp.Compile(tagRegex)
	if err != nil {
		return fmt.Errorf("failed to compile tag pattern: %w", err)
	}

	if err = tags.ForEach(func(tag *plumbing.Reference) error {
		found := regTagPattern.FindAllStringSubmatch(tag.Name().Short(), 1)
		switch len(found) {
		case 0:
			return nil
		case 1:
			version, err := semver.NewVersion(found[0][1])
			if err != nil {
				return fmt.Errorf("failed to parse version %q from tag %q: %w", found[0][1], tag.Name().Short(), err)
			}

			if version.GreaterThan(c.currentVersion) {
				c.currentVersion = version
			}

			return nil
		default:
			return fmt.Errorf("%s: %w", tag.Name().Short(), ErrMultipleMatchInTag)
		}
	}); err != nil {
		return fmt.Errorf("failed to iterate tags: %w", err)
	}

	return nil
}

func (c *Project) commitToRepository(version semver.Version, changelogEntries *changelog.Changelog) error {
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
