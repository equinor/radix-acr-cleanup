#!/bin/bash
if [[ -z "${SP_USER}" ]]; then
  SP_USER=$(cat ${AZURE_CREDENTIALS} | jq -r '.id')
fi

if [[ -z "${SP_SECRET}" ]]; then
  SP_SECRET=$(cat ${AZURE_CREDENTIALS} | jq -r '.password')
fi

# Exit script if az login is unsuccessful.
# radix-acr-cleanup does not work when login is unsuccesful.
# In some situation, e.g. when radix-acr-cleanup is scheduled to run on a newly created node,
# the network is not ready and az login cannot connect to login.microsoftonline.com.
az login --service-principal -u ${SP_USER} -p ${SP_SECRET} --tenant ${TENANT} || exit

./radix-acr-cleanup \
  --period=${PERIOD} \
  --registry=${REGISTRY} \
  --cluster-type=${CLUSTER_TYPE} \
  --active-cluster-name=${ACTIVE_CLUSTER_NAME} \
  --delete-untagged=${DELETE_UNTAGGED} \
  --retain-latest-untagged=${RETAIN_LATEST_UNTAGGED} \
  --perform-delete=${PERFORM_DELETE} \
  --cleanup-days="${CLEANUP_DAYS}" \
  --cleanup-start="${CLEANUP_START}" \
  --cleanup-end="${CLEANUP_END}" \
  --whitelisted="${WHITELISTED}"