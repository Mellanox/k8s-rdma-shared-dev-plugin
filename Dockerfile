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

FROM nvcr.io/nvidia/doca/doca:3.1.0-base-rt-host
COPY --from=builder /usr/src/k8s-rdma-shared-dp/build/k8s-rdma-shared-dp /bin/
COPY . /src

RUN apt-get update && apt-get install -y --no-install-recommends kmod=29-1ubuntu1 && \
    rm -rf /var/lib/apt/lists/*

LABEL io.k8s.display-name="RDMA Shared Device Plugin"

CMD ["/bin/k8s-rdma-shared-dp"]
