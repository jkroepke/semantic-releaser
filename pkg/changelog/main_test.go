package changelog_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jkroepke/semantic-releaser/pkg/changelog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangelogEmpty(t *testing.T) {
	t.Parallel()

	changes := changelog.New()
	assert.Equal(t, 0, changes.Len())
	assert.Equal(t, "", changes.String())
}

func TestChangelog(t *testing.T) {
	t.Parallel()

	date := time.Now().Format("2006-01-02")

	for _, tc := range []struct {
		name              string
		clFunc            func(changes *changelog.Changelog)
		expectedChangelog string
	}{
		{
			name: "Only breaking change",
			clFunc: func(cl *changelog.Changelog) {
				cl.SetNewVersion("2.0.0")
				cl.AddBreaking("Breaking change", "123456")
			},
			expectedChangelog: "## 2.0.0 (%s)\n\n### ⚠ BREAKING CHANGES\n\n* Breaking change (123456)\n\n",
		},
		{
			name: "Only feat change",
			clFunc: func(cl *changelog.Changelog) {
				cl.SetNewVersion("1.1.0")
				cl.AddFeature("Adding a new feature", "123456")
			},
			expectedChangelog: "## 1.1.0 (%s)\n\n### Features\n\n* Adding a new feature (123456)\n\n",
		},
		{
			name: "Only fix change",
			clFunc: func(cl *changelog.Changelog) {
				cl.SetNewVersion("1.0.1")
				cl.AddFix("Fixing a bug", "123456")
			},
			expectedChangelog: "### 1.0.1 (%s)\n\n### Bug Fixes\n\n* Fixing a bug (123456)\n\n",
		},
		{
			name: "with http github repo",
			clFunc: func(cl *changelog.Changelog) {
				cl.SetRemote("https://github.com/jkroepke/semantic-releaser.git")
				cl.SetNewVersion("1.0.1")
				cl.AddFix("Fixing a bug", "123456")
			},
			expectedChangelog: `### [1.0.1](https://github.com/jkroepke/semantic-releaser/compare/1.0.0...1.0.1) (%s)

### Bug Fixes

* Fixing a bug ([123456](https://github.com/jkroepke/semantic-releaser/commit/123456))

`,
		},
		{
			name: "with ssh github repo",
			clFunc: func(cl *changelog.Changelog) {
				cl.SetRemote("git@github.com:jkroepke/semantic-releaser.git")
				cl.SetNewVersion("1.0.1")
				cl.AddFix("Fixing a bug (#866)", "123456")
			},

			expectedChangelog: `### [1.0.1](https://github.com/jkroepke/semantic-releaser/compare/1.0.0...1.0.1) (%s)

### Bug Fixes

* Fixing a bug ([#866](https://github.com/jkroepke/semantic-releaser/pull/866)) ([123456](https://github.com/jkroepke/semantic-releaser/commit/123456))

`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			changes := changelog.New()

			tc.clFunc(changes)
			changes.SetOldVersion("1.0.0")

			assert.Equal(t, 1, changes.Len())

			expected := fmt.Sprintf(tc.expectedChangelog, date)
			assert.Equal(t, expected, changes.String())
		})
	}
}

func TestChangelogNewFile(t *testing.T) {
	t.Parallel()

	changes := changelog.New()
	changes.SetOldVersion("1.0.0")
	changes.SetNewVersion("2.0.0")

	changes.AddBreaking("Breaking change", "123456")
	changes.AddFix("Fixing a bug", "123456")
	changes.AddFeature("Adding a new feature", "123456")

	assert.Equal(t, 3, changes.Len())

	testChangelogFile, err := os.CreateTemp(t.TempDir(), "CHANGELOG-*.md")
	require.NoError(t, err)
	defer os.Remove(testChangelogFile.Name())

	err = changes.WriteTo(testChangelogFile)
	require.NoError(t, err)

	bytes, err := os.ReadFile(testChangelogFile.Name())
	require.NoError(t, err)

	date := time.Now().Format("2006-01-02")
	expected := fmt.Sprintf("## 2.0.0 (%s)\n\n### ⚠ BREAKING CHANGES\n\n* Breaking change (123456)\n\n### Features\n\n* Adding a new feature (123456)\n\n### Bug Fixes\n\n* Fixing a bug (123456)\n\n", date)

	assert.Equal(t, expected, changes.String())
	assert.Equal(t, "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n<!-- INSERT COMMENT -->\n"+expected, string(bytes))
}

func TestChangelogMissingPlaceholder(t *testing.T) {
	t.Parallel()

	changes := changelog.New()
	changes.SetOldVersion("1.0.0")
	changes.SetNewVersion("2.0.0")

	assert.Equal(t, 0, changes.Len())

	testChangelogFile, err := os.CreateTemp(t.TempDir(), "CHANGELOG-*.md")
	require.NoError(t, err)
	defer os.Remove(testChangelogFile.Name())

	err = os.WriteFile(testChangelogFile.Name(), []byte("# Changelog\n\nAll notable changes to this project will be documented in this file.\n"), 0)
	require.NoError(t, err)

	err = changes.WriteTo(testChangelogFile)
	require.Equal(t, changelog.ErrMissingPlaceholder, err)
}

func TestChangelogExistingFile(t *testing.T) {
	t.Parallel()

	changes := changelog.New()
	changes.SetOldVersion("1.0.0")
	changes.SetNewVersion("1.1.0")

	changes.AddFeature("Adding a new feature", "123456")

	testChangelogFile, err := os.CreateTemp(t.TempDir(), "CHANGELOG-*.md")
	require.NoError(t, err)
	defer os.Remove(testChangelogFile.Name())

	err = changes.WriteTo(testChangelogFile)
	require.NoError(t, err)

	require.NoError(t, testChangelogFile.Close())
	testChangelogFile, err = os.OpenFile(testChangelogFile.Name(), os.O_RDWR, 0)
	require.NoError(t, err)

	changes = changelog.New()
	changes.SetOldVersion("1.1.0")
	changes.SetNewVersion("1.1.1")

	changes.AddFeature("Fixed a bug", "123456")

	err = changes.WriteTo(testChangelogFile)
	require.NoError(t, err)

	date := time.Now().Format("2006-01-02")
	expected := fmt.Sprintf("# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n<!-- INSERT COMMENT -->\n### 1.1.1 (%[1]s)\n\n### Features\n\n* Fixed a bug (123456)\n\n\n## 1.1.0 (%[1]s)\n\n### Features\n\n* Adding a new feature (123456)\n\n", date)

	bytes, err := os.ReadFile(testChangelogFile.Name())
	require.NoError(t, err)

	assert.Equal(t, expected, string(bytes))
}
