#!/bin/bash
IMAGE_NAME=$1
VERSION=$2
CPU_ARCH=$3
export DOCKER_CLI_EXPERIMENTAL="enabled";
docker tag $IMAGE_NAME ${IMAGE_NAME}-${CPU_ARCH}:${VERSION}
docker push ${IMAGE_NAME}-${CPU_ARCH}:${VERSION}
docker manifest create ${IMAGE_NAME}:${VERSION} ${IMAGE_NAME}-${CPU_ARCH}:${VERSION}
docker manifest annotate ${IMAGE_NAME}:${VERSION} ${IMAGE_NAME}-${CPU_ARCH}:${VERSION} --arch ${CPU_ARCH}
docker manifest push ${IMAGE_NAME}:${VERSION}
