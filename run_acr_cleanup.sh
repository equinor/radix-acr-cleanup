#!/bin/bash

exec ./radix-acr-cleanup \
  --period=${PERIOD} \
  --registry=${REGISTRY} \
  --subscription-id=${SUBSCRIPTION_ID} \
  --cluster-type=${CLUSTER_TYPE} \
  --active-cluster-name=${ACTIVE_CLUSTER_NAME} \
  --current-cluster-name=${CURRENT_CLUSTER_NAME} \
  --delete-untagged=${DELETE_UNTAGGED} \
  --retain-latest-untagged=${RETAIN_LATEST_UNTAGGED} \
  --perform-delete=${PERFORM_DELETE} \
  --cleanup-days="${CLEANUP_DAYS}" \
  --cleanup-start="${CLEANUP_START}" \
  --cleanup-end="${CLEANUP_END}" \
  --whitelisted="${WHITELISTED}" \
  --log-level="${LOG_LEVEL}" \
  --pretty-print="${PRETTY_PRINT}"