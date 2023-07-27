package main

import (
	"testing"
	"time"

	"github.com/equinor/radix-acr-cleanup/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

func Test_isManifestWithinGracePeriod(t *testing.T) {
	created, _ := time.Parse(time.RFC3339, "2010-01-01T15:00:00Z")
	manifest := manifest.Data{LastUpdateTime: created}

	timeAfter, _ := time.Parse(time.RFC3339, "2010-01-01T16:00:00Z")
	assert.True(t, isManifestWithinGracePeriod(manifest, timeAfter, 2*time.Hour))
	assert.False(t, isManifestWithinGracePeriod(manifest, timeAfter, 0))

	timeBefore, _ := time.Parse(time.RFC3339, "2010-01-01T14:00:00Z")
	assert.True(t, isManifestWithinGracePeriod(manifest, timeBefore, 0))
}
