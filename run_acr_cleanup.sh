#!/bin/bash
if [[ -z "${SP_USER}" ]]; then
  SP_USER=$(cat ${AZURE_CREDENTIALS} | jq -r '.id')
fi

if [[ -z "${SP_SECRET}" ]]; then
  SP_SECRET=$(cat ${AZURE_CREDENTIALS} | jq -r '.password')
fi

az login --service-principal -u ${SP_USER} -p ${SP_SECRET} --tenant ${TENANT}
/usr/local/bin/radix-acr-cleanup --registry=${REGISTRY} --clusterType=${CLUSTER_TYPE} --deleteUntagged=${DELETE_UNTAGGED}