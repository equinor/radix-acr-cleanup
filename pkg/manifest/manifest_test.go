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
      "jl5mh"
    ],
    "timestamp": "2019-10-25T10:07:31.3052551Z"
  },
  {
    "digest": "sha256:7f8343822a17acc74c88c96badc2a1a981ad9fc749b4c5194816c0ef01fc9457",
    "tags": [
	  "jl5mh",
	  "development-jl5mh"
    ],
    "timestamp": "2019-10-25T10:07:31.3052551Z"
  },
  {
    "digest": "sha256:7f8343822a17acc74c88c96badc2a1a981ad9fc749b4c5194816c0ef01fc9457",
    "tags": [
	  "jl5mh",
	  "playground-jl5mh"
    ],
    "timestamp": "2019-10-25T10:07:31.3052551Z"
  },
  {
    "digest": "sha256:7f8343822a17acc74c88c96badc2a1a981ad9fc749b4c5194816c0ef01fc9457",
    "tags": [
	  "jl5mh",
	  "production-jl5mh"
    ],
    "timestamp": "2019-10-25T10:07:31.3052551Z"
  }
]
`

func TestFromStringData(t *testing.T) {
	manifests := FromStringData(testManifest)
	assert.Equal(t, 4, len(manifests))
	assert.True(t, manifests[0].IsNotTaggedForAnyClustertype())
	assert.False(t, manifests[1].IsNotTaggedForAnyClustertype())
	assert.False(t, manifests[2].IsNotTaggedForAnyClustertype())
	assert.False(t, manifests[3].IsNotTaggedForAnyClustertype())
	assert.True(t, manifests[1].IsTaggedForCurrentClustertype("development"))
	assert.True(t, manifests[2].IsTaggedForCurrentClustertype("playground"))
	assert.True(t, manifests[3].IsTaggedForCurrentClustertype("production"))
}
