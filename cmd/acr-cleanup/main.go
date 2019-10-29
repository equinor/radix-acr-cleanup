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
	"github.com/equinor/radix-acr-cleanup/pkg/image"
	"github.com/equinor/radix-acr-cleanup/pkg/manifest"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const clusterTypeLabel = "clusterType"
const repositoryLabel = "repository"
const isTaggedLabel = "tagged"

var nrImagesDeleted = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "radix_acr_images_deleted",
		Help: "The total number of image manifests deleted",
	}, []string{clusterTypeLabel, repositoryLabel, isTaggedLabel})

func main() {
	fs := initializeFlagSet()

	var (
		period         = fs.Duration("period", time.Minute*60, "Interval between checks")
		registry       = fs.String("registry", "", "Name of the ACR registry (Required)")
		clusterType    = fs.String("clusterType", "", "Type of cluster (Required)")
		deleteUntagged = fs.Bool("deleteUntagged", false, "Solution can delete untagged images")
		performDelete  = fs.Bool("performDelete", false, "Can control that the solution can actually delete manifest")
	)

	parseFlagsFromArgs(fs)

	if registry == nil || clusterType == nil {
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.Info("1.0.0")
	log.Infof("Period: %s", *period)
	log.Infof("Registry: %s", *registry)
	log.Infof("Clustertype: %s", *clusterType)
	log.Infof("Delete untagged: %t", *deleteUntagged)
	log.Infof("Perform delete: %t", *performDelete)

	go maintainImages(*period, *registry, *clusterType, *deleteUntagged, *performDelete)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func maintainImages(period time.Duration, registry, clusterType string, deleteUntagged, performDelete bool) {
	source := rand.NewSource(time.Now().UnixNano())
	tick := delaytick.New(source, period)
	for time := range tick {
		log.Infof("Start deleting images %s", time)
		deleteImagesBelongingTo(registry, clusterType, deleteUntagged, performDelete)
	}
}

func initializeFlagSet() *pflag.FlagSet {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  Radix acr cleanup.\n")
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

func deleteImagesBelongingTo(registry, clusterType string, deleteUntagged, performDelete bool) {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		log.Infof("It took %s to run", duration)
	}()

	_, radixClient := getKubernetesClient()

	imagesInCluster := listActiveImagesInCluster(radixClient)
	repositories := listRepositories(registry)

	numRepositories := len(repositories)
	processedRepositories := 0

	for _, repository := range repositories {
		log.Infof("Process repository %s", repository)
		manifests := listManifests(registry, repository)
		for _, manifest := range manifests {
			manifestExistInCluster := manifestExistInCluster(repository, manifest, imagesInCluster)

			isNotTaggedForAnyClustertype := manifest.IsNotTaggedForAnyClustertype()
			if isNotTaggedForAnyClustertype && !deleteUntagged {
				log.Infof("Manifest %s is untagged, %s, and is not mandated for deletion", manifest.Digest, strings.Join(manifest.Tags, ","))
				continue
			} else if deleteUntagged && !manifestExistInCluster {
				log.Infof("Manifest %s is untagged, %s, and is mandated for deletion", manifest.Digest, strings.Join(manifest.Tags, ","))
				untagged := true
				deleteManifest(registry, repository, clusterType, performDelete, untagged, manifest)
				continue
			}

			isTaggedForCurrentClustertype := manifest.IsTaggedForCurrentClustertype(clusterType)
			if !isTaggedForCurrentClustertype {
				log.Infof("Manifest %s is tagged for different cluster type, %s, and should not be deleted", manifest.Digest, strings.Join(manifest.Tags, ","))
				continue
			}

			if !manifestExistInCluster {
				untagged := false
				deleteManifest(registry, repository, clusterType, performDelete, untagged, manifest)
			} else {
				log.Infof("Manifest %s exists in cluster for tags %s", manifest.Digest, strings.Join(manifest.Tags, ","))
			}
		}

		processedRepositories++

		if (processedRepositories % 10) == 0 {
			log.Debugf("Processed %d out of %d repositories", processedRepositories, numRepositories)
		}
	}
}

func manifestExistInCluster(repository string, manifest manifest.Data, imagesInCluster []image.Data) bool {
	manifestExistInCluster := false

	for _, image := range imagesInCluster {
		if strings.EqualFold(image.Repository, repository) {
			if manifest.Contains(image.Tag) {
				manifestExistInCluster = true
				break
			}
		}
	}

	return manifestExistInCluster
}

func listActiveImagesInCluster(radixClient radixclient.Interface) []image.Data {
	imagesInCluster := make([]image.Data, 0)

	rds, _ := radixClient.RadixV1().RadixDeployments(corev1.NamespaceAll).List(metav1.ListOptions{})
	for _, rd := range rds.Items {
		for _, component := range rd.Spec.Components {
			image := image.Parse(component.Image)
			if image == nil {
				continue
			}

			imagesInCluster = append(imagesInCluster, *image)
		}
	}

	return imagesInCluster
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

func listManifests(registry, repository string) []manifest.Data {
	listCmd := newListManifestsCommand(registry, repository)

	var outb bytes.Buffer
	listCmd.Stdout = &outb

	if err := listCmd.Run(); err != nil {
		log.Fatalf("Error listing manifests: %v", err)
	}

	return manifest.FromStringData(outb.String())
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

func deleteManifest(registry, repository, clusterType string, performDelete, untagged bool, manifest manifest.Data) {
	if performDelete {
		// Will perform an actual delete
		deleteCmd := newDeleteManifestsCommand(registry, repository, manifest.Digest)

		var outb bytes.Buffer
		deleteCmd.Stdout = &outb

		if err := deleteCmd.Run(); err != nil {
			log.Errorf("Error deleting manifest: %v", err)
		}
	}

	// Will log a delete even if perform delete is false, so that
	// we can test the consequences of this utility
	log.Infof("Deleted digest %s for repository %s for tags %s", manifest.Digest, repository, strings.Join(manifest.Tags, ","))
	if !untagged {
		addImageDeleted(clusterType, repository)
	} else {
		addUntaggedImageDeleted(clusterType, repository)
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

func addUntaggedImageDeleted(clusterType, repository string) {
	nrImagesDeleted.With(prometheus.Labels{clusterTypeLabel: clusterType, repositoryLabel: repository, isTaggedLabel: "false"}).Inc()
}

func addImageDeleted(clusterType, repository string) {
	nrImagesDeleted.With(prometheus.Labels{clusterTypeLabel: clusterType, repositoryLabel: repository, isTaggedLabel: "true"}).Inc()
}
