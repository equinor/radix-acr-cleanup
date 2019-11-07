package acr

import (
	"bytes"
	"fmt"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/equinor/radix-acr-cleanup/pkg/manifest"
)

// ListRepositories Is all available repositories in provided ACR registry
func ListRepositories(registry string) []string {
	listCmd := newListRepositoriesCommand(registry)

	var outb bytes.Buffer
	listCmd.Stdout = &outb

	if err := listCmd.Run(); err != nil {
		log.Fatalf("Error listing manifests: %v", err)
	}

	return getRepositoriesFromStringData(outb.String())
}

// ListManifests Lists all available manifests for a single repository
func ListManifests(registry, repository string) []manifest.Data {
	listCmd := newListManifestsCommand(registry, repository)

	var outb bytes.Buffer
	listCmd.Stdout = &outb

	if err := listCmd.Run(); err != nil {
		log.Fatalf("Error listing manifests: %v", err)
	}

	return manifest.FromStringDataSorted(outb.String())
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
	cmd.Stderr = log.NewEntry(log.StandardLogger()).
		WithField("cmd", cmd.Args[0]).
		WithField("std", "err").
		WriterLevel(log.WarnLevel)

	return cmd
}

func getRepositoriesFromStringData(data string) []string {
	repositories := make([]string, 0)
	err := yaml.Unmarshal([]byte(data), &repositories)
	if err != nil {
		return repositories
	}
	return repositories
}

func newListManifestsCommand(registry, repository string) *exec.Cmd {
	args := []string{"acr", "repository", "show-manifests",
		"--name", registry,
		"--repository", repository,
		"--orderby", "time_asc"}

	cmd := exec.Command("az", args...)
	cmd.Stderr = log.NewEntry(log.StandardLogger()).
		WithField("cmd", cmd.Args[0]).
		WithField("std", "err").
		WriterLevel(log.WarnLevel)

	return cmd
}

func newDeleteManifestsCommand(registry, repository, digest string) *exec.Cmd {
	args := []string{"acr", "repository", "delete",
		"--name", registry,
		"--image", fmt.Sprintf("%s@%s", repository, digest),
		"--yes"}

	cmd := exec.Command("az", args...)
	cmd.Stderr = log.NewEntry(log.StandardLogger()).
		WithField("cmd", cmd.Args[0]).
		WithField("std", "err").
		WriterLevel(log.WarnLevel)

	return cmd
}
