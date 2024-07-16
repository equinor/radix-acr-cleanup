package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/equinor/radix-acr-cleanup/pkg/acr"
	"github.com/equinor/radix-acr-cleanup/pkg/image"
	"github.com/equinor/radix-acr-cleanup/pkg/manifest"
	"github.com/equinor/radix-common/utils/delaytick"
	"github.com/equinor/radix-common/utils/timewindow"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
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
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGTERM)
	defer cancel()

	fs := initializeFlagSet()

	var (
		period               = fs.Duration("period", time.Minute*60, "Interval between checks")
		registry             = fs.String("registry", "", "Name of the ACR registry (Required)")
		clusterType          = fs.String("cluster-type", "", "Type of cluster (Required)")
		activeClusterName    = fs.String("active-cluster-name", "", "Name of the active cluster (Required)")
		deleteUntagged       = fs.Bool("delete-untagged", false, "Solution can delete untagged images")
		retainLatestUntagged = fs.Int("retain-latest-untagged", 5, "Solution can retain x number of untagged images if set to delete")
		performDelete        = fs.Bool("perform-delete", false, "Can control that the solution can actually delete manifest")
		cleanupDays          = fs.StringSlice("cleanup-days", timewindow.EveryDay, "Schedule cleanup on these days")
		cleanupStart         = fs.String("cleanup-start", "0:00", "Start time")
		cleanupEnd           = fs.String("cleanup-end", "6:00", "End time")
		whitelisted          = fs.StringSlice("whitelisted", []string{}, "Lists repositories which are whitelisted")
		prettyPrint          = fs.Bool("pretty-print", false, "Use colored text instead of json for log output")
		logLevel             = fs.String("log-level", "info", "Set log level for output, defaults to 'info', options: 'debug', 'info', 'warn', 'error'")
	)

	parseFlagsFromArgs(fs)

	stringIsNilOrEmpty := func(s *string) bool {
		return s == nil || len(strings.TrimSpace(*s)) == 0
	}

	if stringIsNilOrEmpty(registry) || stringIsNilOrEmpty(clusterType) || stringIsNilOrEmpty(activeClusterName) {
		flag.PrintDefaults()
		<-ctx.Done()
		os.Exit(1)
	}

	_, err := initZerologger(context.Background(), *logLevel, *prettyPrint)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Zerolog")
	}

	log.Info().Msgf("Cleanup days: %s", *cleanupDays)
	log.Info().Msgf("Cleanup start: %s", *cleanupStart)
	log.Info().Msgf("Cleanup end: %s", *cleanupEnd)
	log.Info().Msgf("Period: %s", *period)
	log.Info().Msgf("Registry: %s", *registry)
	log.Info().Msgf("Clustertype: %s", *clusterType)
	log.Info().Msgf("Active cluster name: %s", *activeClusterName)
	log.Info().Msgf("Delete untagged: %t", *deleteUntagged)
	log.Info().Msgf("Retain untagged: %d", *retainLatestUntagged)
	log.Info().Msgf("Perform delete: %t", *performDelete)
	log.Info().Msgf("Whitelisted: %s", *whitelisted)

	kubeClient, radixClient := getKubernetesClient()
	kubeutil, err := kube.New(kubeClient, radixClient, nil, nil)
	if err != nil {
		panic(err)
	}

	go maintainImages(ctx, kubeutil, *cleanupDays, *cleanupStart, *cleanupEnd, *period,
		*registry, *clusterType, *activeClusterName, *deleteUntagged, *retainLatestUntagged, *performDelete, *whitelisted)

	http.Handle("/metrics", promhttp.Handler())
	log.Info().Msg("API is serving on port :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal().Err(err).Msg("Server exited unexpectedly")
	}
	<-ctx.Done()
}

func initZerologger(ctx context.Context, logLevel string, prettyPrint bool) (context.Context, error) {
	if logLevel == "" {
		logLevel = "info"
	}

	zerologLevel, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		return nil, err
	}
	zerolog.SetGlobalLevel(zerologLevel)
	zerolog.DurationFieldUnit = time.Millisecond
	if prettyPrint {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.TimeOnly})
	}
	ctx = log.Logger.WithContext(ctx)
	return ctx, nil
}

func maintainImages(ctx context.Context, kubeutil *kube.Kube, cleanupDays []string, cleanupStart, cleanupEnd string, period time.Duration, registry, clusterType, activeClusterName string, deleteUntagged bool, retainLatestUntagged int, performDelete bool, whitelisted []string) {
	window, err := timewindow.New(cleanupDays, cleanupStart, cleanupEnd, timezone)

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to build time window")
	}

	source := rand.NewSource(time.Now().UnixNano())
	tick := delaytick.New(source, period)
	for range tick {
		now := time.Now()
		if window.Contains(now) {
			log.Info().Msgf("Start deleting images %s", now)
			deleteImagesBelongingTo(ctx, kubeutil, registry, clusterType, activeClusterName,
				deleteUntagged, retainLatestUntagged, performDelete, whitelisted)
		} else {
			log.Info().Msgf("%s is outside of window. Continue sleeping", now)
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

func deleteImagesBelongingTo(ctx context.Context, kubeutil *kube.Kube, registry, clusterType, activeClusterName string, deleteUntagged bool, retainLatestUntagged int, performDelete bool, whitelisted []string) {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		log.Info().Dur("ellapsed-ms", duration).Msgf("It took %s to run", duration)
	}()

	if !isActiveCluster(ctx, kubeutil, activeClusterName) {
		log.Error().Msg("Current cluster is not active cluster, abort")
		return
	}

	imagesInCluster, err := listActiveImagesInCluster(ctx, kubeutil)
	if err != nil {
		log.Error().Err(err).Msg("Unable to list images in cluster")
		return
	}

	repositories, err := acr.ListRepositories(registry)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get repositories")
		return
	}

	numRepositories := len(repositories)
	processedRepositories := 0
	for _, repository := range repositories {
		if isWhitelisted(repository, whitelisted) {
			log.Info().Str("repo", repository).Msg("Skip repository as it is whitelisted")
			continue
		}

		log.Debug().Str("repo", repository).Msg("Process repository")
		manifests, err := acr.ListManifests(registry, repository)
		if err != nil {
			log.Error().Str("repo", repository).Err(err).Msg("Unable to get manifests for repository")
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
				log.Debug().Str("repo", repository).Msgf("Manifest %s is untagged, %s, and is not mandated for deletion", manifest.Digest, strings.Join(manifest.Tags, ","))
				continue
			} else if isNotTaggedForAnyClustertype && deleteUntagged && !manifestExistInCluster {
				if numManifests > retainLatestUntagged {
					log.Debug().Str("repo", repository).Msgf("Manifest %s is untagged, %s, and is mandated for deletion", manifest.Digest, strings.Join(manifest.Tags, ","))
					untagged := true
					deleteManifest(registry, repository, clusterType, performDelete, untagged, manifest)
					numManifests--
				} else {
					addUntaggedImageRetained(clusterType, repository)
					log.Info().Str("repo", repository).Msgf("Manifest %s is untagged, %s, and is mandated for deletion, but will be retained", manifest.Digest, strings.Join(manifest.Tags, ","))
				}

				continue
			}

			isTaggedForCurrentClustertype := manifest.IsTaggedForCurrentClustertype(clusterType)
			if !isTaggedForCurrentClustertype {
				addImageRetained(clusterType, repository)
				log.Debug().Str("repo", repository).Msgf("Manifest %s is tagged for different cluster type, %s, and should not be deleted", manifest.Digest, strings.Join(manifest.Tags, ","))
				continue
			}

			if !manifestExistInCluster {
				untagged := false
				deleteManifest(registry, repository, clusterType, performDelete, untagged, manifest)
			} else {
				addImageRetained(clusterType, repository)
				log.Debug().Str("repo", repository).Msgf("Manifest %s exists in cluster for tags %s", manifest.Digest, strings.Join(manifest.Tags, ","))
			}
		}

		processedRepositories++

		if (processedRepositories % 10) == 0 {
			log.Debug().Msgf("Processed %d out of %d repositories", processedRepositories, numRepositories)
		}
	}
}

func deleteManifest(registry, repository, clusterType string, performDelete, untagged bool, manifest manifest.Data) {
	if performDelete {
		if err := acr.DeleteManifest(registry, repository, manifest); err != nil {
			log.Error().Err(err).Msg("Error deleting manifest")
			addImageDeleteError(clusterType, repository)
			return
		}

		log.Info().Str("repo", repository).Msgf("Deleted digest %s for repository %s for tags %s", manifest.Digest, repository, strings.Join(manifest.Tags, ","))

	} else {
		log.Info().Str("repo", repository).Msgf("Digest %s for repository %s for tags %s would have been deleted", manifest.Digest, repository, strings.Join(manifest.Tags, ","))
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

// Checks for existence of active cluster ingresses in prod environment for radix-api app to determine if this is the active cluster
func isActiveCluster(ctx context.Context, kubeutil *kube.Kube, activeClusterName string) bool {
	currentClusterName, err := kubeutil.GetClusterName(ctx)
	if err != nil {
		panic(err)
	}
	return strings.EqualFold(currentClusterName, activeClusterName)
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
	createdWithGracePeriod := manifest.LastUpdateTime.Add(gracePeriod)
	return createdWithGracePeriod.After(time)
}

// Lists distinct images in cluster based on all RadixDeployments
func listActiveImagesInCluster(ctx context.Context, kubeutil *kube.Kube) ([]image.Data, error) {
	imagesInCluster := make([]image.Data, 0)

	rds, err := kubeutil.ListRadixDeployments(ctx, corev1.NamespaceAll)
	if err != nil {
		return imagesInCluster, err
	}

	for _, rd := range rds {
		for _, component := range rd.Spec.Components {
			componentImage := image.Parse(component.Image)
			if componentImage == nil {
				continue
			}

			imagesInCluster = append(imagesInCluster, *componentImage)
		}

		for _, job := range rd.Spec.Jobs {
			componentImage := image.Parse(job.Image)
			if componentImage == nil {
				continue
			}

			imagesInCluster = append(imagesInCluster, *componentImage)
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
			log.Fatal().Err(err).Msg("getClusterConfig InClusterConfig")
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("getClusterConfig k8s client")
	}

	radixClient, err := radixclient.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msg("getClusterConfig radix client")
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
