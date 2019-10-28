package manifest

import (
	"strings"

	"gopkg.in/yaml.v2"
)

// Data Structure to hold manifest information
type Data struct {
	Digest    string
	Tags      []string
	Timestamp string
}

// FromStringData Returns manifests from string data
func FromStringData(data string) []Data {
	manifests := make([]Data, 0)
	err := yaml.Unmarshal([]byte(data), &manifests)
	if err != nil {
		return manifests
	}
	return manifests
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
