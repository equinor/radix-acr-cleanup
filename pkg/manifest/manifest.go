package manifest

import (
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// Data Structure to hold manifest information
type Data struct {
	Digest    string
	Tags      []string
	Timestamp time.Time
}

// FromStringData Returns manifests from string data
func FromStringData(data string) ([]Data, error) {
	manifests := make([]Data, 0)
	err := yaml.Unmarshal([]byte(data), &manifests)
	if err != nil {
		return nil, err
	}

	return manifests, nil
}

// FromStringDataSorted Returns data sorted by timestamp asc
func FromStringDataSorted(data string) ([]Data, error) {
	manifests, err := FromStringData(data)
	if err != nil {
		return nil, err
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[j].Timestamp.After(manifests[i].Timestamp)
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
