package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var testManifest = `
[
  {
    "digest": "sha256:7f8343822a17acc74c88c96badc2a1a981ad9fc749b4c5194816c0ef01fc9457",
    "tags": [
      "second"
    ],
    "timestamp": "2019-10-25T10:07:31.3052551Z"
  },
  {
    "digest": "sha256:7f8343822a17acc74c88c96badc2a1a981ad9fc749b4c5194816c0ef01fc9457",
    "tags": [
	  "third",
	  "development-third"
    ],
    "timestamp": "2019-10-25T10:08:31.3052551Z"
  },
  {
    "digest": "sha256:7f8343822a17acc74c88c96badc2a1a981ad9fc749b4c5194816c0ef01fc9457",
    "tags": [
	  "first",
	  "playground-first"
    ],
    "timestamp": "2019-10-25T09:07:31.3052551Z"
  },
  {
    "digest": "sha256:7f8343822a17acc74c88c96badc2a1a981ad9fc749b4c5194816c0ef01fc9457",
    "tags": [
	  "fourth",
	  "production-fourth"
    ],
    "timestamp": "2019-10-26T10:07:31.3052551Z"
  }
]
`

func TestFromStringData(t *testing.T) {
	manifests, err := FromStringData(testManifest)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(manifests))
	assert.True(t, manifests[0].IsNotTaggedForAnyClustertype())
	assert.False(t, manifests[1].IsNotTaggedForAnyClustertype())
	assert.False(t, manifests[2].IsNotTaggedForAnyClustertype())
	assert.False(t, manifests[3].IsNotTaggedForAnyClustertype())
	assert.True(t, manifests[1].IsTaggedForCurrentClustertype("development"))
	assert.True(t, manifests[2].IsTaggedForCurrentClustertype("playground"))
	assert.True(t, manifests[3].IsTaggedForCurrentClustertype("production"))
}

func TestFromStringDataSorted(t *testing.T) {
	manifests, err := FromStringDataSorted(testManifest)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(manifests))
	assert.Equal(t, "first", manifests[0].Tags[0])
	assert.Equal(t, "second", manifests[1].Tags[0])
	assert.Equal(t, "third", manifests[2].Tags[0])
	assert.Equal(t, "fourth", manifests[3].Tags[0])
}
