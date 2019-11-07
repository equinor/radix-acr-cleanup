# radix-acr-cleanup

## Introduction

`radix-acr-cleanup` will delete images no longer referenced in the cluster, tagged with the cluster type, or delete untagged images (not tagged with cluster type) no longer in the cluster if mandated, except for a list of whitelisted images. `radix-pipeline` will tag manifest with cluster type and cluster name. I.e. a manifest may look like this:

```
  {
    "digest": "sha256:74e708055583f994e52f174b08f88a3b1d5cf4b69350499bf647a59876543210",
    "tags": [
      "ss5zv",
      "production-ss5zv",
      "prod-39-ss5zv"
    ],
    "timestamp": "2019-10-30T07:38:55.8812664Z"
  }
```

Only a `production` type cluster should be able to delete this manifest. If the `production-*` and `prod-39*` tags were missing, then `production` cluster can only delete this if the `delete-untagged` parameter has been set. Note that this can potentially create a problem for another cluster using the same registry. Also, a non-active cluster will not perform any cleanup.

## Installation

This can be installed to cluster manually using the ```make deploy-via-helm```, and will be deployed using flux https://github.com/equinor/radix-flux

`radix-acr-cleanup` is built using [Azure Devops](https://dev.azure.com/omnia-radix/radix-operator/_build?definitionId=5), then deployed to cluster through a Helm release using the [Flux Operator](https://github.com/weaveworks/flux) whenever a new image is pushed to the container registry for the corresponding branch.

[![Build Status](https://dev.azure.com/omnia-radix/radix-operator/_apis/build/status/equinor.radix-acr-cleanup?branchName=master)](https://dev.azure.com/omnia-radix/radix-operator/_build/latest?definitionId=5&branchName=master)

## Configuration

The following arguments can be passed to radix-acr-cleanup via the values of the Helm chart:

```
Flags:
      --registry string            The registry to perform cleanup of
      --cluster-type string         The type of cluster to check for tags of
      --delete-untagged bool        If true, the solution can be responsible for deleting untagged                                 images
      --retain-latest-untagged int   Will ensure that x number of untagged manifests will be retained
      --perform-delete bool         If this is false, the solution won't perform an
                                   actual delete, only log a delete for simulation purposes
      --period duration            Interval between checks (default 1h0m0s)
      --cleanup-days strings        Only cleanup on these days (default [su,mo,tu,we,th,fr,sa])
      --cleanup-start string        Only cleanup after this time of day (default "0:00")
      --cleanup-end string          Only cleanup before this time of day (default "23:59")
      --whitelisted strings        List of whitelisted repositories (i.e. radix-operator,
                                   radix-pipeline)
```

## Setting a schedule

Use --cleanup-days, --cleanup-start, and --cleanup-end to set a schedule. time-zone will be the `Local` timezone for the cluster. For example, business hours can be specified with:

	--cleanup-days mon,tue,wed,thu,fri
	--cleanup-start 8am
	--cleanup-end 4pm

Times can be formatted in numerous ways, including 5pm, 5:00pm 17:00, and 17.

Note that when using smaller time windows, you should consider shortening the check period (--period).

## Prometheus Metrics

The `radix-acr-cleanup` pod exposes metrics (:8080/metrics), `radix_acr_images_deleted` which tells the number of manifests deleted (or which would have been deleted if `perform-delete` set to `true`) and `radix_acr_images_retained` for the number of images not deleted from ACR.

## Developing

You need Go installed. Make sure GOPATH and GOROOT are properly set up. Clone the repo into your GOPATH and run go mod download.
