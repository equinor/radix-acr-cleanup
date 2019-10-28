package image

import "strings"

// Data Structure to hold image information
type Data struct {
	Registry   string
	Repository string
	Tag        string
}

// Parse will deconstruct container image
func Parse(image string) *Data {
	imageRepository := strings.Split(image, "/")
	if len(imageRepository) == 1 {
		return nil
	}

	repository := imageRepository[0]
	imageTag := strings.Split(imageRepository[1], ":")

	if len(imageTag) == 1 {
		return nil
	}

	return &Data{
		repository,
		imageTag[0],
		imageTag[1],
	}
}
