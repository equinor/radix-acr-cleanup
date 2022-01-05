package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/equinor/radix-acr-cleanup/pkg/acr"
	"github.com/equinor/radix-acr-cleanup/pkg/delaytick"
	"github.com/equinor/radix-acr-cleanup/pkg/image"
	"github.com/equinor/radix-acr-cleanup/pkg/manifest"
	"github.com/equinor/radix-acr-cleanup/pkg/timewindow"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	timezone            = "Local"
	clusterTypeLabel    = "clusterType"
	repositoryLabel     = "repository"
	isTaggedLabel       = "tagged"
	manifestGracePeriod = 2 * time.Hour
)

var nrImagesDeleted = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "radix_acr_images_deleted",
		Help: "The total number of image manifests deleted",
	}, []string{clusterTypeLabel, repositoryLabel, isTaggedLabel})

var nrImagesRetained = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "radix_acr_images_retained",
		Help: "The total number of image manifests retained",
	}, []string{clusterTypeLabel, repositoryLabel, isTaggedLabel})

var nrImagesDeleteErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "radix_acr_image_delete_errors",
		Help: "The total number of image manifest delete errors",
	}, []string{clusterTypeLabel, repositoryLabel})

var nrListManifestErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "radix_acr_list_manifest_errors",
		Help: "The total number of manifest list request errors",
	}, []string{clusterTypeLabel, repositoryLabel})

func main() {
	fs := initializeFlagSet()

	var (
		period               = fs.Duration("period", time.Minute*60, "Interval between checks")
		registry             = fs.String("registry", "", "Name of the ACR registry (Required)")
		clusterType          = fs.String("cluster-type", "", "Type of cluster (Required)")
		deleteUntagged       = fs.Bool("delete-untagged", false, "Solution can delete untagged images")
		retainLatestUntagged = fs.Int("retain-latest-untagged", 5, "Solution can retain x number of untagged images if set to delete")
		performDelete        = fs.Bool("perform-delete", false, "Can control that the solution can actually delete manifest")
		cleanupDays          = fs.StringSlice("cleanup-days", timewindow.EveryDay, "Schedule cleanup on these days")
		cleanupStart         = fs.String("cleanup-start", "0:00", "Start time")
		cleanupEnd           = fs.String("cleanup-end", "6:00", "End time")
		whitelisted          = fs.StringSlice("whitelisted", []string{}, "Lists repositories which are whitelisted")
	)

	parseFlagsFromArgs(fs)

	if registry == nil || clusterType == nil {
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.Infof("Cleanup days: %s", *cleanupDays)
	log.Infof("Cleanup start: %s", *cleanupStart)
	log.Infof("Cleanup end: %s", *cleanupEnd)
	log.Infof("Period: %s", *period)
	log.Infof("Registry: %s", *registry)
	log.Infof("Clustertype: %s", *clusterType)
	log.Infof("Delete untagged: %t", *deleteUntagged)
	log.Infof("Retain untagged: %d", *retainLatestUntagged)
	log.Infof("Perform delete: %t", *performDelete)
	log.Infof("Whitelisted: %s", *whitelisted)

	kubeClient, radixClient := getKubernetesClient()

	go maintainImages(kubeClient, radixClient, *cleanupDays, *cleanupStart, *cleanupEnd, *period,
		*registry, *clusterType, *deleteUntagged, *retainLatestUntagged, *performDelete, *whitelisted)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func maintainImages(kubeClient kubernetes.Interface, radixClient radixclient.Interface,
	cleanupDays []string, cleanupStart string, cleanupEnd string, period time.Duration,
	registry, clusterType string, deleteUntagged bool,
	retainLatestUntagged int, performDelete bool, whitelisted []string) {
	window, err := timewindow.New(cleanupDays, cleanupStart, cleanupEnd, timezone)

	if err != nil {
		log.Fatalf("Failed to build time window: %v", err)
	}

	source := rand.NewSource(time.Now().UnixNano())
	tick := delaytick.New(source, period)
	for range tick {
		time := time.Now()
		if window.Contains(time) {
			log.Infof("Start deleting images %s", time)
			deleteImagesBelongingTo(kubeClient, radixClient, registry, clusterType,
				deleteUntagged, retainLatestUntagged, performDelete, whitelisted)
		} else {
			log.Infof("%s is outside of window. Continue sleeping", time)
		}
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

func deleteImagesBelongingTo(kubeClient kubernetes.Interface, radixClient radixclient.Interface, registry, clusterType string,
	deleteUntagged bool, retainLatestUntagged int, performDelete bool, whitelisted []string) {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		log.Infof("It took %s to run", duration)
	}()

	if !isActiveCluster(kubeClient) {
		log.Error("Current cluster is not active cluster, abort")
		return
	}

	imagesInCluster, err := listActiveImagesInCluster(radixClient)
	if err != nil {
		log.Errorf("Unable to list images in cluster, %v", err)
		return
	}

	repositories, err := acr.ListRepositories(registry)
	if err != nil {
		log.Errorf("Unable to get repositories: %v", err)
		return
	}

	numRepositories := len(repositories)
	processedRepositories := 0
	for _, repository := range repositories {
		if isWhitelisted(repository, whitelisted) {
			log.Infof("Skip repository %s, as it is whitelisted", repository)
			continue
		}

		log.Debugf("Process repository %s", repository)
		manifests, err := acr.ListManifests(registry, repository)
		if err != nil {
			log.Errorf("Unable to get manifests for repository %s: %v", repository, err)
			addListManifestError(clusterType, repository)
			continue
		}
		numManifests := len(manifests)

		for _, manifest := range manifests {
			isNotTaggedForAnyClustertype := manifest.IsNotTaggedForAnyClustertype()

			// If this manifest has a timestamp newer than start,
			// the list of images might not be correct
			// The grace period will prevent images from being deleted if they are created before, but close to, the start time.
			if isManifestWithinGracePeriod(manifest, start, manifestGracePeriod) {
				if isNotTaggedForAnyClustertype {
					addUntaggedImageRetained(clusterType, repository)
				} else {
					addImageRetained(clusterType, repository)
				}

				continue
			}

			manifestExistInCluster := doesManifestExistInCluster(repository, manifest, imagesInCluster)
			if isNotTaggedForAnyClustertype && !deleteUntagged {
				addUntaggedImageRetained(clusterType, repository)
				log.Debugf("Manifest %s is untagged, %s, and is not mandated for deletion", manifest.Digest, strings.Join(manifest.Tags, ","))
				continue
			} else if isNotTaggedForAnyClustertype && deleteUntagged && !manifestExistInCluster {
				if numManifests > retainLatestUntagged {
					log.Debugf("Manifest %s is untagged, %s, and is mandated for deletion", manifest.Digest, strings.Join(manifest.Tags, ","))
					untagged := true
					deleteManifest(registry, repository, clusterType, performDelete, untagged, manifest)
					numManifests--
				} else {
					addUntaggedImageRetained(clusterType, repository)
					log.Infof("Manifest %s is untagged, %s, and is mandated for deletion, but will be retained", manifest.Digest, strings.Join(manifest.Tags, ","))
				}

				continue
			}

			isTaggedForCurrentClustertype := manifest.IsTaggedForCurrentClustertype(clusterType)
			if !isTaggedForCurrentClustertype {
				addImageRetained(clusterType, repository)
				log.Debugf("Manifest %s is tagged for different cluster type, %s, and should not be deleted", manifest.Digest, strings.Join(manifest.Tags, ","))
				continue
			}

			if !manifestExistInCluster {
				untagged := false
				deleteManifest(registry, repository, clusterType, performDelete, untagged, manifest)
			} else {
				addImageRetained(clusterType, repository)
				log.Debugf("Manifest %s exists in cluster for tags %s", manifest.Digest, strings.Join(manifest.Tags, ","))
			}
		}

		processedRepositories++

		if (processedRepositories % 10) == 0 {
			log.Debugf("Processed %d out of %d repositories", processedRepositories, numRepositories)
		}
	}
}

func deleteManifest(registry, repository, clusterType string, performDelete, untagged bool, manifest manifest.Data) {
	if performDelete {
		if err := acr.DeleteManifest(registry, repository, manifest); err != nil {
			log.Errorf("Error deleting manifest: %v", err)
			addImageDeleteError(clusterType, repository)
			return
		}

		log.Infof("Deleted digest %s for repository %s for tags %s", manifest.Digest, repository, strings.Join(manifest.Tags, ","))

	} else {
		log.Infof("Digest %s for repository %s for tags %s would have been deleted", manifest.Digest, repository, strings.Join(manifest.Tags, ","))
	}

	// Will log a delete even if perform delete is false, so that
	// we can test the consequences of this utility
	if !untagged {
		addImageDeleted(clusterType, repository)
	} else {
		addUntaggedImageDeleted(clusterType, repository)
	}

}

func isWhitelisted(repository string, whitelisted []string) bool {
	for _, wlRepo := range whitelisted {
		if strings.EqualFold(repository, wlRepo) {
			return true
		}
	}

	return false
}

// Checks for existence of active cluster ingresses to determine if this is the active cluster
func isActiveCluster(kubeClient kubernetes.Interface) bool {
	ingresses, err := kubeClient.NetworkingV1().Ingresses(corev1.NamespaceAll).List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", kube.RadixActiveClusterAliasLabel, strconv.FormatBool(true)),
		},
	)

	if err == nil && len(ingresses.Items) > 0 {
		return true
	}

	return false
}

// Checks if manifest exists in cluster
func doesManifestExistInCluster(repository string, manifest manifest.Data, imagesInCluster []image.Data) bool {
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

// Test if the manifest was created after a specified time and a grace period
func isManifestWithinGracePeriod(manifest manifest.Data, time time.Time, gracePeriod time.Duration) bool {
	createdWithGracePeriod := manifest.Timestamp.Add(gracePeriod)
	return createdWithGracePeriod.After(time)
}

// Lists distinct images in cluster based on all RadixDeployments
func listActiveImagesInCluster(radixClient radixclient.Interface) ([]image.Data, error) {
	imagesInCluster := make([]image.Data, 0)

	rds, err := radixClient.RadixV1().RadixDeployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return imagesInCluster, err
	}

	for _, rd := range rds.Items {
		for _, component := range rd.Spec.Components {
			image := image.Parse(component.Image)
			if image == nil {
				continue
			}

			imagesInCluster = append(imagesInCluster, *image)
		}

		for _, job := range rd.Spec.Jobs {
			image := image.Parse(job.Image)
			if image == nil {
				continue
			}

			imagesInCluster = append(imagesInCluster, *image)
		}
	}

	return imagesInCluster, nil
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

// Metrics

func addUntaggedImageDeleted(clusterType, repository string) {
	nrImagesDeleted.With(prometheus.Labels{clusterTypeLabel: clusterType, repositoryLabel: repository, isTaggedLabel: "false"}).Inc()
}

func addImageDeleted(clusterType, repository string) {
	nrImagesDeleted.With(prometheus.Labels{clusterTypeLabel: clusterType, repositoryLabel: repository, isTaggedLabel: "true"}).Inc()
}

func addUntaggedImageRetained(clusterType, repository string) {
	nrImagesRetained.With(prometheus.Labels{clusterTypeLabel: clusterType, repositoryLabel: repository, isTaggedLabel: "false"}).Inc()
}

func addImageRetained(clusterType, repository string) {
	nrImagesRetained.With(prometheus.Labels{clusterTypeLabel: clusterType, repositoryLabel: repository, isTaggedLabel: "true"}).Inc()
}

func addImageDeleteError(clusterType, repository string) {
	nrImagesDeleteErrors.With(prometheus.Labels{clusterTypeLabel: clusterType, repositoryLabel: repository}).Inc()
}

func addListManifestError(clusterType, repository string) {
	nrListManifestErrors.With(prometheus.Labels{clusterTypeLabel: clusterType, repositoryLabel: repository}).Inc()
}
