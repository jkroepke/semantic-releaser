package changelog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

type Changelog struct {
	newVersion string
	oldVersion string
	links      links
	breaking   []commit
	fixes      []commit
	features   []commit
}

type commit struct {
	hash    string
	message string
}

type links struct {
	compareURL string
	prURL      string
	commitURL  string
}

var (
	regexpPRNumber        = regexp.MustCompile(`\(#(\d+)\)$`)
	regexpGithubRepoInfos = regexp.MustCompile(`^(?:https://github\.com/|git@github\.com:|ssh://git@github\.com/)([^/:]+/[^/:]+?)(?:\.git)?$`)
)

func New() *Changelog {
	return &Changelog{}
}

func (c *Changelog) Len() int {
	return len(c.breaking) + len(c.fixes) + len(c.features)
}

func (c *Changelog) SetRemote(remoteURL string) {
	//nolint:gocritic // more options coming
	switch {
	case regexpGithubRepoInfos.MatchString(remoteURL):
		matches := regexpGithubRepoInfos.FindStringSubmatch(remoteURL)

		//nolint:perfsprint
		c.links.compareURL = fmt.Sprintf("https://github.com/%s/compare/%%s...%%s", matches[1])
		c.links.prURL = fmt.Sprintf("https://github.com/%s/pull/$1", matches[1])
		//nolint:perfsprint
		c.links.commitURL = fmt.Sprintf("https://github.com/%s/commit/%%s", matches[1])
	}
}

func (c *Changelog) SetOldVersion(version string) {
	c.oldVersion = version
}

func (c *Changelog) SetNewVersion(version string) {
	c.newVersion = version
}

func (c *Changelog) AddBreaking(message, hash string) {
	c.breaking = append(c.breaking, commit{hash: hash, message: c.decorateMessage(message)})
}

func (c *Changelog) AddFix(message, hash string) {
	c.fixes = append(c.fixes, commit{hash: hash, message: c.decorateMessage(message)})
}

func (c *Changelog) AddFeature(message, hash string) {
	c.features = append(c.features, commit{hash: hash, message: c.decorateMessage(message)})
}

func (c *Changelog) getCompareLink() string {
	if c.links.compareURL == "" {
		return ""
	}

	return fmt.Sprintf(c.links.compareURL, c.oldVersion, c.newVersion)
}

func (c *Changelog) getCommitLink(hash string) string {
	if c.links.commitURL == "" {
		return ""
	}

	return fmt.Sprintf(c.links.commitURL, hash)
}

// decorateMessage decorates the message with links to PRs if possible.
func (c *Changelog) decorateMessage(message string) string {
	if c.links.prURL == "" {
		return message
	}

	//nolint:gocritic // more options coming
	switch {
	case regexpPRNumber.MatchString(message):
		return regexpPRNumber.ReplaceAllString(message, fmt.Sprintf("([#$1](%s))", c.links.prURL))
	}

	return message
}

func (c *Changelog) String() string {
	if c.Len() == 0 {
		return ""
	}

	sb := &strings.Builder{}

	date := time.Now().Format("2006-01-02")
	link := c.getCompareLink()

	if strings.HasSuffix(c.newVersion, ".0") {
		sb.WriteString("## ")
	} else {
		sb.WriteString("### ")
	}

	if link != "" {
		sb.WriteString(fmt.Sprintf("[%s](%s) (%s)\n\n", c.newVersion, link, date))
	} else {
		sb.WriteString(fmt.Sprintf("%s (%s)\n\n", c.newVersion, date))
	}

	c.writeSection(sb, "⚠ BREAKING CHANGES", c.breaking)
	c.writeSection(sb, "Features", c.features)
	c.writeSection(sb, "Bug Fixes", c.fixes)

	return sb.String()
}

func (c *Changelog) writeSection(sb *strings.Builder, header string, entries []commit) {
	if len(entries) == 0 {
		return
	}

	sb.WriteString("### ")
	sb.WriteString(header)
	sb.WriteString("\n\n")

	for _, entry := range entries {
		link := c.getCommitLink(entry.hash)

		if link != "" {
			sb.WriteString(fmt.Sprintf("* %s ([%s](%s))", entry.message, entry.hash, link))
		} else {
			sb.WriteString(fmt.Sprintf("* %s (%s)", entry.message, entry.hash))
		}

		sb.WriteString("\n")
	}

	sb.WriteString("\n")
}

func (c *Changelog) WriteTo(filePath string) error {
	data, err := os.ReadFile(filePath)
	if errors.Is(err, os.ErrNotExist) || len(data) == 0 {
		changelog := "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n<!-- INSERT COMMENT -->\n" + c.String()

		err = os.WriteFile(filePath, []byte(changelog), 0o600)
		if err != nil {
			return fmt.Errorf("failed to write changelog: %w", err)
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if !bytes.Contains(data, []byte("<!-- INSERT COMMENT -->")) {
		return ErrMissingPlaceholder
	}

	data = bytes.Replace(data, []byte("<!-- INSERT COMMENT -->"), []byte("<!-- INSERT COMMENT -->\n"+c.String()), 1)

	err = os.WriteFile(filePath, data, 0)
	if err != nil {
		return fmt.Errorf("failed to write changelog: %w", err)
	}

	return nil
}
