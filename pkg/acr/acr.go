package acr

import (
	"context"
	"fmt"
	"sort"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	"github.com/equinor/radix-acr-cleanup/pkg/manifest"
)

type Client struct {
	client *azcontainerregistry.Client
}

func NewClient(registry string) (*Client, error) {
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	client, err := azcontainerregistry.NewClient(fmt.Sprintf("https://%s", registry), credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACR client: %w", err)
	}

	return &Client{client: client}, nil
}

func (client *Client) ListRepositories(ctx context.Context) ([]string, error) {
	repositories := make([]string, 0)
	pager := client.client.NewListRepositoriesPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}
		for _, repository := range page.Repositories.Names {
			if repository != nil {
				repositories = append(repositories, *repository)
			}
		}
	}

	return repositories, nil
}

func (client *Client) ListManifests(ctx context.Context, repository string) ([]manifest.Data, error) {
	orderBy := azcontainerregistry.ArtifactManifestOrderByLastUpdatedOnAscending
	pager := client.client.NewListManifestsPager(repository, &azcontainerregistry.ClientListManifestsOptions{
		OrderBy: &orderBy,
	})
	manifests := make([]manifest.Data, 0)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list manifests: %w", err)
		}
		for _, attributes := range page.Manifests.Attributes {
			if attributes == nil {
				continue
			}

			item := manifest.Data{}
			if attributes.Digest != nil {
				item.Digest = *attributes.Digest
			}
			if attributes.LastUpdatedOn != nil {
				item.LastUpdateTime = *attributes.LastUpdatedOn
			}
			for _, tag := range attributes.Tags {
				if tag != nil {
					item.Tags = append(item.Tags, *tag)
				}
			}
			manifests = append(manifests, item)
		}
	}

	sort.SliceStable(manifests, func(i, j int) bool {
		return manifests[i].LastUpdateTime.Before(manifests[j].LastUpdateTime)
	})
	return manifests, nil
}

func (client *Client) DeleteManifest(ctx context.Context, repository string, item manifest.Data) error {
	if _, err := client.client.DeleteManifest(ctx, repository, item.Digest, nil); err != nil {
		return fmt.Errorf("failed to delete manifest: %w", err)
	}
	return nil
}
