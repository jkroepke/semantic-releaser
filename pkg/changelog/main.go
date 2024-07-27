package changelog

import "strings"

type Changelog struct {
	breaking []string
	fixes    []string
	features []string
}

func New() *Changelog {
	return &Changelog{}
}

func (c *Changelog) Len() int {
	return len(c.breaking) + len(c.fixes) + len(c.features)
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

func (c *Changelog) String() string {
	sb := strings.Builder{}

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
