# radix-acr-cleanup

## Introduction

radix-acr-cleanup will delete images no longer referenced in the cluster, tagged with the cluster type, or delete untagged images (not tagged with cluster type) no longer in the cluster if mandated, except for a list of whitelisted images. `radix-pipeline` will tag manifest with cluster type and cluster name. I.e. a manifest may look like this:

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

Only a `production` type cluster should be able to delete this manifest. If the `production-*` and `prod-39*` tags where missing, then `production` cluster can only delete this if the `deleteUntagged` parameter has been set. Note that this can potentially create a problem for another cluster using the same registry.

## Installation

This can be installed to cluster manually using the ```make deploy-via-helm```, and will be deployed using flux https://github.com/equinor/radix-flux

## Configuration

The following arguments can be passed to radix-acr-cleanup via the values of the Helm chart:

```
Flags:
      --registry string       The registry to perform cleanup of
      --clusterType string    The type of cluster to check for tags of
      --deleteUntagged bool   If true, the solution can be responsible for deleting untagged images
      --performDelete bool    If this is false, the solution won't perform an 
                              actual delete, only log a delete for simulation purposes
      --period duration       Interval between checks (default 1h0m0s)
      --cleanupDays strings   Only cleanup on these days (default [su,mo,tu,we,th,fr,sa])
      --cleanupStart string   Only cleanup after this time of day (default "0:00")
      --cleanupEnd string     Only cleanup before this time of day (default "23:59")
      --whitelisted strings   List of whitelisted repositories (i.e. radix-operator,radix-pipeline)
```