package main

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/equinor/radix-acr-cleanup/pkg/delaytick"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const clusterTypeLabel = "clusterType"

var (
	clusterTypes    = [...]string{"development", "production", "playground"}
	nrImagesDeleted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "radix_acr_images_deleted",
			Help: "The total number of image manifests deleted",
		}, []string{clusterTypeLabel})
)

type Image struct {
	Registry   string
	Repository string
	Tag        string
}

type Manifest struct {
	Digest    string
	Tags      []string
	Timestamp string
}

func main() {
	fs := initializeFlagSet()

	var (
		period         = fs.Duration("period", time.Minute*60, "Interval between checks")
		registry       = fs.String("registry", "", "Name of the ACR registry (Required)")
		clusterType    = fs.String("clusterType", "", "Type of cluster (Required)")
		deleteUntagged = fs.Bool("deleteUntagged", false, "Solution can delete untagged images")
	)

	parseFlagsFromArgs(fs)

	if registry == nil || clusterType == nil {
		flag.PrintDefaults()
		os.Exit(1)
	}

	go maintainImages(*period, *registry, *clusterType, *deleteUntagged)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func maintainImages(period time.Duration, registry, clusterType string, deleteUntagged bool) {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		println(fmt.Sprintf("It took %s to run", duration))
	}()

	source := rand.NewSource(time.Now().UnixNano())
	tick := delaytick.New(source, period)
	for time := range tick {
		println(fmt.Sprintf("Start deleting images %s", time))
		deleteImagesBelongingTo(registry, clusterType, deleteUntagged)
	}
}

func initializeFlagSet() *pflag.FlagSet {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  radix api-server.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}
	return fs
}

func parseFlagsFromArgs(fs *pflag.FlagSet) {
	err := fs.Parse(os.Args[1:])
	switch {
	case err == pflag.ErrHelp:
		os.Exit(0)
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err.Error())
		fs.Usage()
		os.Exit(2)
	}
}

func deleteImagesBelongingTo(registry, clusterType string, deleteUntagged bool) {
	_, radixClient := getKubernetesClient()

	images := listActiveImagesInCluster(radixClient)
	repositories := listRepositories(registry)

	numRepositories := len(repositories)
	processedRepositories := 0

	for _, repository := range repositories {
		existInCluster, image := existInCluster(repository, images)

		manifests := listManifests(registry, repository)
		for _, manifest := range manifests {
			isUntaggedForType := isUntaggedForType(manifest)
			if isUntaggedForType && !deleteUntagged {
				continue
			}

			isTaggedForType := isTaggedForType(manifest, clusterType)
			if !isTaggedForType {
				continue
			}

			isTagInCluster := isTagInCluster(manifest, image)
			if isTagInCluster {
				continue
			}

			if !existInCluster {
				deleteManifest(registry, repository, manifest.Digest)
				println(fmt.Sprintf("Deleted digest %s for repository %s", manifest.Digest, repository))
				addImageDeleted(clusterType)
			}
		}

		processedRepositories++

		if (processedRepositories % 10) == 0 {
			println(fmt.Sprintf("Processed %d out of %d repositories", processedRepositories, numRepositories))
		}
	}
}

func existInCluster(repository string, images []Image) (bool, Image) {
	for _, image := range images {
		if image.Repository == repository {
			return true, image
		}
	}

	return false, Image{}
}

func isUntaggedForType(manifest Manifest) bool {
	for _, clusterType := range clusterTypes {
		if isTaggedForType(manifest, clusterType) {
			return false
		}
	}

	return true
}

func isTaggedForType(manifest Manifest, clusterType string) bool {
	for _, tag := range manifest.Tags {
		if strings.HasPrefix(tag, clusterType+"-") {
			return true
		}
	}

	return false
}

func isTagInCluster(manifest Manifest, image Image) bool {
	for _, tag := range manifest.Tags {
		if image.Tag == tag {
			return true
		}
	}

	return false
}

func listActiveImagesInCluster(radixClient radixclient.Interface) []Image {
	images := make([]Image, 0)

	rds, _ := radixClient.RadixV1().RadixDeployments(corev1.NamespaceAll).List(metav1.ListOptions{})
	for _, rd := range rds.Items {
		for _, component := range rd.Spec.Components {
			image := getImage(component.Image)
			if image == nil {
				continue
			}

			images = append(images, *image)
		}
	}

	return images
}

func getImage(image string) *Image {
	imageRepository := strings.Split(image, "/")
	if len(imageRepository) == 1 {
		return nil
	}

	repository := imageRepository[0]
	imageTag := strings.Split(imageRepository[1], ":")

	if len(imageTag) == 1 {
		return nil
	}

	return &Image{
		repository,
		imageTag[0],
		imageTag[1],
	}
}

func getKubernetesClient() (kubernetes.Interface, radixclient.Interface) {
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)

	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig k8s client: %v", err)
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig radix client: %v", err)
	}

	log.Printf("Successfully constructed k8s client to API server %v", config.Host)
	return client, radixClient
}

func listRepositories(registry string) []string {
	listCmd := newListRepositoriesCommand(registry)

	var outb bytes.Buffer
	listCmd.Stdout = &outb

	if err := listCmd.Run(); err != nil {
		log.Fatalf("Error listing manifests: %v", err)
	}

	return getRepositoriesFromStringData(outb.String())
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

func listManifests(registry, repository string) []Manifest {
	listCmd := newListManifestsCommand(registry, repository)

	var outb bytes.Buffer
	listCmd.Stdout = &outb

	if err := listCmd.Run(); err != nil {
		log.Fatalf("Error listing manifests: %v", err)
	}

	return getManifestsFromStringData(outb.String())
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

func getManifestsFromStringData(data string) []Manifest {
	maifests := make([]Manifest, 0)
	err := yaml.Unmarshal([]byte(data), &maifests)
	if err != nil {
		return maifests
	}
	return maifests
}

func deleteManifest(registry, repository, digest string) {
	listCmd := newDeleteManifestsCommand(registry, repository, digest)

	var outb bytes.Buffer
	listCmd.Stdout = &outb

	if err := listCmd.Run(); err != nil {
		log.Errorf("Error deleting manifest: %v", err)
	}
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

func addImageDeleted(clusterType string) {
	nrImagesDeleted.With(prometheus.Labels{clusterTypeLabel: clusterType}).Inc()
}
