package manifest

import (
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Data Structure to hold manifest information
type Data struct {
	Digest         string    `yaml:"digest"`
	Tags           []string  `yaml:"tags"`
	LastUpdateTime time.Time `yaml:"lastUpdateTime"`
}

// FromData Returns manifests from byte array
func FromData(data []byte) ([]Data, error) {
	manifests := make([]Data, 0)
	err := yaml.Unmarshal(data, &manifests)
	if err != nil {
		return nil, err
	}

	return manifests, nil
}

// FromDataSorted Returns data sorted by timestamp asc
func FromDataSorted(data []byte) ([]Data, error) {
	manifests, err := FromData(data)
	if err != nil {
		return nil, err
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[j].LastUpdateTime.After(manifests[i].LastUpdateTime)
	})
	return manifests, nil
}

var clusterTypes = [...]string{"development", "production", "playground"}

// IsTaggedForCurrentClustertype Indicates if manifest is tagged for cluster
func (manifest Data) IsTaggedForCurrentClustertype(clusterType string) bool {
	for _, tag := range manifest.Tags {
		if strings.HasPrefix(tag, clusterType+"-") {
			return true
		}
	}

	return false
}

// IsNotTaggedForAnyClustertype Indicates that manifest is not tagged for any
// cluster type
func (manifest Data) IsNotTaggedForAnyClustertype() bool {
	for _, clusterType := range clusterTypes {
		if manifest.IsTaggedForCurrentClustertype(clusterType) {
			return false
		}
	}

	return true
}

// Contains Manifest contains image tag
func (manifest Data) Contains(imageTag string) bool {
	contains := false
	for _, tag := range manifest.Tags {
		if imageTag == tag {
			contains = true
			break
		}
	}

	return contains
}
