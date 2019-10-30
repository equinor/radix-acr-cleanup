# radix-acr-cleanup

## Introduction

radix-acr-cleanup will delete images no longer referenced in the cluster, tagged with the cluster type, or delete untagged images (not tagged with cluster type) no longer in the cluster if mandated, except for a list of whitelisted images

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