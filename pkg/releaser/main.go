package releaser

import (
	"errors"
	"fmt"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/jkroepke/semantic-releaser/pkg/config"
	"github.com/jkroepke/semantic-releaser/pkg/project"
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

	worktree, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	projects, err := worktree.Filesystem.ReadDir(r.conf.ProjectsDir)
	if err != nil {
		return fmt.Errorf("failed to read project directories: %w", err)
	}

	errCh := make(chan error, len(projects))

	for _, projectDir := range projects {
		if !projectDir.IsDir() {
			continue
		}

		wg.Add(1)

		go func() {
			defer wg.Done()

			proj, err := project.New(r.logger, r.conf, r.repo, r.commitParser, projectDir.Name())
			if err != nil {
				if errors.Is(err, project.ErrProjectFileNotFound) {
					return
				}

				errCh <- fmt.Errorf("failed to initialize project: %w", err)
			}

			nextVersion, changelog, err := proj.DetectRelease()
			if err != nil {
				errCh <- err

				return
			}

			if changelog.Len() == 0 {
				return
			}

			if err := proj.Release(nextVersion, changelog); err != nil {
				errCh <- fmt.Errorf("failed to release project: %w", err)

				return
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
