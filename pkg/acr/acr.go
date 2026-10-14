package acr

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"github.com/equinor/radix-acr-cleanup/pkg/manifest"
)

// ListRepositories Is all available repositories in provided ACR registry
func ListRepositories(registry, subscriptionID string) ([]string, error) {
	listCmd := newListRepositoriesCommand(registry, subscriptionID)

	var stdout bytes.Buffer
	listCmd.Stdout = &stdout
	var stderr bytes.Buffer
	listCmd.Stderr = &stderr

	if err := listCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w: %w", err, errors.New(stderr.String()))
	}

	repos, err := getRepositoriesFromStringData(stdout.String())
	if err != nil {
		return nil, fmt.Errorf("failed process repository data: %w", err)
	}

	return repos, nil
}

// ListManifests Lists all available manifests for a single repository
func ListManifests(registry, subscriptionID, repository string) ([]manifest.Data, error) {
	listCmd := newListManifestsCommand(registry, subscriptionID, repository)

	var stdout bytes.Buffer
	listCmd.Stdout = &stdout
	var stderr bytes.Buffer
	listCmd.Stderr = &stderr

	if err := listCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to list manifests: %w: %w", err, errors.New(stderr.String()))
	}

	manifests, err := manifest.FromDataSorted(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to process manifest data: %w", err)
	}

	return manifests, nil
}

// DeleteManifest Will delete a single manifest
func DeleteManifest(registry, subscriptionID, repository string, manifest manifest.Data) error {
	// Will perform an actual delete
	deleteCmd := newDeleteManifestsCommand(registry, subscriptionID, repository, manifest.Digest)

	var stderr bytes.Buffer
	deleteCmd.Stderr = &stderr

	if err := deleteCmd.Run(); err != nil {
		return fmt.Errorf("failed to delete manifest: %w: %w", err, errors.New(stderr.String()))
	}
	return nil
}

func newListRepositoriesCommand(registry, subscriptionID string) *exec.Cmd {
	args := []string{"acr", "repository", "list",
		"--only-show-errors",
		"--name", registry,
		"--subscription", subscriptionID}

	return exec.Command("az", args...)
}

func getRepositoriesFromStringData(data string) ([]string, error) {
	repositories := make([]string, 0)
	err := json.Unmarshal([]byte(data), &repositories)
	if err != nil {
		return nil, err
	}
	return repositories, nil
}

func newListManifestsCommand(registry, subscriptionID, repository string) *exec.Cmd {
	args := []string{"acr", "manifest", "list-metadata",
		"--only-show-errors",
		"--registry", registry,
		"--name", repository,
		"--orderby", "time_asc",
		"--subscription", subscriptionID}

	return exec.Command("az", args...)
}

func newDeleteManifestsCommand(registry, subscriptionID, repository, digest string) *exec.Cmd {
	args := []string{"acr", "repository", "delete",
		"--only-show-errors",
		"--name", registry,
		"--image", fmt.Sprintf("%s@%s", repository, digest),
		"--subscription", subscriptionID,
		"--yes"}

	return exec.Command("az", args...)
}
