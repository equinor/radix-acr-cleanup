package image

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRelevant(t *testing.T) {
	image := Parse("repo.azurecr.io/some-repo:some-tag")
	assert.Equal(t, "repo.azurecr.io", image.Registry)
	assert.Equal(t, "some-repo", image.Repository)
	assert.Equal(t, "some-tag", image.Tag)
}

func TestParseIrrelevant(t *testing.T) {
	image := Parse("repo.azurecr.io/some-repo")
	assert.Nil(t, image)

	image = Parse("some-repo:some-tag")
	assert.Nil(t, image)
}
