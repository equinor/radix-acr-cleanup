#!/bin/bash
if [[ -z "${SP_USER}" ]]; then
  SP_USER=$(cat ${AZURE_CREDENTIALS} | jq -r '.id')
fi

if [[ -z "${SP_SECRET}" ]]; then
  SP_SECRET=$(cat ${AZURE_CREDENTIALS} | jq -r '.password')
fi

echo ${PERIOD}
echo ${REGISTRY}
echo ${CLUSTER_TYPE}
echo ${DELETE_UNTAGGED}

az login --service-principal -u ${SP_USER} -p ${SP_SECRET} --tenant ${TENANT}
/radix-acr-cleanup/radix-acr-cleanup --period=${PERIOD} --registry=${REGISTRY} --clusterType=${CLUSTER_TYPE} --deleteUntagged=${DELETE_UNTAGGED}