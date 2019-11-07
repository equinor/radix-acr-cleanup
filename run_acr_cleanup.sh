#!/bin/bash
if [[ -z "${SP_USER}" ]]; then
  SP_USER=$(cat ${AZURE_CREDENTIALS} | jq -r '.id')
fi

if [[ -z "${SP_SECRET}" ]]; then
  SP_SECRET=$(cat ${AZURE_CREDENTIALS} | jq -r '.password')
fi

az login --service-principal -u ${SP_USER} -p ${SP_SECRET} --tenant ${TENANT}
/radix-acr-cleanup/radix-acr-cleanup \
  --period=${PERIOD} \
  --registry=${REGISTRY} \
  --cluster-type=${CLUSTER_TYPE} \
  --delete-untagged=${DELETE_UNTAGGED} \
  --retain-latest-untagged=${RETAIN_LATEST_UNTAGGED} \
  --perform-delete=${PERFORM_DELETE} \
  --cleanup-days="${CLEANUP_DAYS}" \
  --cleanup-start="${CLEANUP_START}" \
  --cleanup-end="${CLEANUP_END}" \
  --whitelisted="${WHITELISTED}"