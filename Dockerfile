# Copyright 2025 NVIDIA CORPORATION & AFFILIATES
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

FROM golang:alpine as builder

COPY . /usr/src/k8s-rdma-shared-dp

ARG GOPROXY
ENV GOPROXY=$GOPROXY

ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy

RUN apk add --no-cache --virtual build-base=0.5-r3 linux-headers=5.19.5-r0
WORKDIR /usr/src/k8s-rdma-shared-dp
RUN make clean && \
    make build

FROM alpine:3 AS pkgs
RUN apk add --no-cache hwdata-pci=0.395-r0 kmod=34.2-r0


FROM nvcr.io/nvidia/distroless/go:v3.1.13-dev

# hadolint ignore=DL3002
USER 0:0

SHELL ["/busybox/sh", "-c"]
# hadolint ignore=DL4005
RUN ln -s /busybox/sh /bin/sh

COPY --from=builder /usr/src/k8s-rdma-shared-dp/build/k8s-rdma-shared-dp /bin/
COPY . /src

RUN mkdir -p /usr/share/hwdata
COPY --from=pkgs /usr/share/hwdata/pci.ids /usr/share/hwdata/pci.ids
COPY --from=pkgs /sbin/lsmod /sbin/lsmod

LABEL io.k8s.display-name="RDMA Shared Device Plugin"

CMD ["/bin/k8s-rdma-shared-dp"]
