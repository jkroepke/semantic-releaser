package releaser

import (
	"errors"
	"fmt"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/jkroepke/semantic-releaser/pkg/chart"
	"github.com/jkroepke/semantic-releaser/pkg/config"
	"github.com/jkroepke/semantic-releaser/pkg/helm"
	cc "github.com/leodido/go-conventionalcommits"
	"github.com/rs/zerolog"
)

// Releaser handles the release process for Helm charts.
type Releaser struct {
	logger       zerolog.Logger
	conf         *config.Config
	repo         *git.Repository
	commitParser cc.Machine
}

// New creates a new Releaser instance.
func New(logger zerolog.Logger, conf *config.Config, repo *git.Repository, commitParser cc.Machine) *Releaser {
	return &Releaser{logger, conf, repo, commitParser}
}

// Run executes the release process for all Helm charts found in the configured directory.
//
//nolint:cyclop
func (r *Releaser) Run() error {
	wg := sync.WaitGroup{}
	errCh := make(chan error)

	worktree, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	chartDirectories, err := worktree.Filesystem.ReadDir(r.conf.ChartsDir)
	if err != nil {
		return fmt.Errorf("failed to read chart directories: %w", err)
	}

	helmClient, err := helm.New(r.conf.HelmBinaryPath)
	if err != nil {
		return fmt.Errorf("failed to initialize Helm client: %w", err)
	}

	for _, chartDirectory := range chartDirectories {
		if !chartDirectory.IsDir() {
			continue
		}

		helmChart, err := chart.New(r.logger, r.conf, r.repo, r.commitParser, chartDirectory.Name())
		if err != nil {
			if errors.Is(err, chart.ErrChartYamlNotFound) {
				continue
			}

			return fmt.Errorf("failed to initialize chart: %w", err)
		}

		wg.Add(1)

		go func() {
			defer wg.Done()

			nextVersion, changelog, err := helmChart.DetectRelease()
			if err != nil {
				errCh <- err
			}

			if changelog.Len() == 0 {
				return
			}

			if err := helmChart.Release(helmClient, nextVersion, changelog); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err = range errCh {
		if err != nil {
			return err //nolint:wrapcheck
		}
	}

	return nil
}
