#!/bin/sh

branch=${GITHUB_REF##*/}
sha=${GITHUB_SHA::8}
ts=$(date +%s)
build_id=${branch}-${sha}-${ts}

image_tag=${ACR_NAME}.azurecr.io/${IMAGE_NAME}:$build_id
docker build . -t $image_tag

az acr login --name ${ACR_NAME}
docker push $image_tag