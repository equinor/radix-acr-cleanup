package acr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/equinor/radix-acr-cleanup/pkg/logwriter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/equinor/radix-acr-cleanup/pkg/manifest"
)

// ListRepositoriesError error
func ListRepositoriesError(registry string, cause error) error {
	return fmt.Errorf("list repositories for registry %s failed: %w", registry, cause)
}

// ListManifestsError error
func ListManifestsError(repository string, cause error) error {
	return fmt.Errorf("list manifests for repository %s failed: %w", repository, cause)
}

// ListRepositories Is all available repositories in provided ACR registry
func ListRepositories(registry string) ([]string, error) {
	listCmd := newListRepositoriesCommand(registry)

	var outb bytes.Buffer
	listCmd.Stdout = &outb

	if err := listCmd.Run(); err != nil {
		return nil, ListRepositoriesError(registry, err)
	}

	repos, err := getRepositoriesFromStringData(outb.String())
	if err != nil {
		return nil, ListRepositoriesError(registry, err)
	}

	return repos, nil
}

// ListManifests Lists all available manifests for a single repository
func ListManifests(registry, repository string) ([]manifest.Data, error) {
	listCmd := newListManifestsCommand(registry, repository)

	var outb bytes.Buffer
	listCmd.Stdout = &outb

	if err := listCmd.Run(); err != nil {
		return nil, ListManifestsError(repository, err)
	}

	manifests, err := manifest.FromDataSorted(outb.Bytes())
	if err != nil {
		return nil, ListManifestsError(repository, err)
	}

	return manifests, nil
}

// DeleteManifest Will delete a single manifest
func DeleteManifest(registry, repository string, manifest manifest.Data) error {
	// Will perform an actual delete
	deleteCmd := newDeleteManifestsCommand(registry, repository, manifest.Digest)

	var outb bytes.Buffer
	deleteCmd.Stdout = &outb

	return deleteCmd.Run()
}

func newListRepositoriesCommand(registry string) *exec.Cmd {
	args := []string{"acr", "repository", "list",
		"--name", registry}

	cmd := exec.Command("az", args...)
	logger := log.With().
		Str("cmd", cmd.Args[0]).
		Str("std", "err").
		Logger()

	cmd.Stderr = logwriter.New(&logger, zerolog.WarnLevel)

	return cmd
}

func getRepositoriesFromStringData(data string) ([]string, error) {
	repositories := make([]string, 0)
	err := json.Unmarshal([]byte(data), &repositories)
	if err != nil {
		return nil, err
	}
	return repositories, nil
}

func newListManifestsCommand(registry, repository string) *exec.Cmd {
	args := []string{"acr", "manifest", "list-metadata",
		"--registry", registry,
		"--name", repository,
		"--orderby", "time_asc"}

	cmd := exec.Command("az", args...)
	logger := log.With().
		Str("cmd", cmd.Args[0]).
		Str("std", "err").
		Logger()

	cmd.Stderr = logwriter.New(&logger, zerolog.WarnLevel)

	return cmd
}

func newDeleteManifestsCommand(registry, repository, digest string) *exec.Cmd {
	args := []string{"acr", "repository", "delete",
		"--name", registry,
		"--image", fmt.Sprintf("%s@%s", repository, digest),
		"--yes"}

	cmd := exec.Command("az", args...)
	logger := log.With().
		Str("cmd", cmd.Args[0]).
		Str("std", "err").
		Logger()

	cmd.Stderr = logwriter.New(&logger, zerolog.WarnLevel)

	return cmd
}
