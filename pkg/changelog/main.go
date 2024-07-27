package changelog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Changelog struct {
	newVersion string
	oldVersion string
	link       string
	breaking   []string
	fixes      []string
	features   []string
}

func New() *Changelog {
	return &Changelog{}
}

func (c *Changelog) Len() int {
	return len(c.breaking) + len(c.fixes) + len(c.features)
}

func (c *Changelog) SetOldVersion(version string) {
	c.oldVersion = version
}

func (c *Changelog) SetNewVersion(version string) {
	c.newVersion = version
}

func (c *Changelog) AddBreaking(msg string) {
	c.breaking = append(c.breaking, msg)
}

func (c *Changelog) AddFix(msg string) {
	c.fixes = append(c.fixes, msg)
}

func (c *Changelog) AddFeature(msg string) {
	c.features = append(c.features, msg)
}

func (c *Changelog) getCompareLink() string {
	switch true {
	case os.Getenv("GITHUB_ACTIONS") == "true":
		return fmt.Sprintf("https://github.com/%s/compare/v%s...v%s", os.Getenv("GITHUB_REPOSITORY"), c.oldVersion, c.newVersion)
	}

	return ""
}

func (c *Changelog) String() string {
	sb := strings.Builder{}

	date := time.Now().Format("2006-01-02")
	link := c.getCompareLink()

	if link != "" {
		sb.WriteString(fmt.Sprintf("## [%s](%s) (%s)", c.newVersion, link, date))
	} else {
		sb.WriteString(fmt.Sprintf("## %s (%s)", c.newVersion, date))
	}

	c.writeSection(sb, "⚠ BREAKING CHANGES", c.breaking)
	c.writeSection(sb, "Features", c.features)
	c.writeSection(sb, "Bug Fixes", c.fixes)

	return sb.String()
}

func (c *Changelog) writeSection(sb strings.Builder, header string, entries []string) {
	if len(entries) == 0 {
		return
	}

	sb.WriteString("## ")
	sb.WriteString(header)
	sb.WriteString("\n\n * ")
	sb.WriteString(strings.Join(entries, "\n* "))
	sb.WriteString("\n\n")
}

func (c *Changelog) WriteTo(filePath string) error {
	data, err := os.ReadFile(filePath)
	if errors.Is(err, os.ErrNotExist) {
		changelog := fmt.Sprintf("# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n<!-- INSERT COMMENT -->\n%s\n", c.String())

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
		return errors.New("changelog file does not contain <!-- INSERT COMMENT -->")
	}

	data = bytes.Replace(data, []byte("<!-- INSERT COMMENT -->"), []byte(fmt.Sprintf("<!-- INSERT COMMENT -->\n%s\n\n", c.String())), 1)

	err = os.WriteFile(filePath, data, 0)
	if err != nil {
		return fmt.Errorf("failed to write changelog: %w", err)
	}

	return nil
}
