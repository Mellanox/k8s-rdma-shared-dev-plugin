ARG BASE_IMAGE=alpine:3.15.4
FROM golang:alpine as builder

ADD . /usr/src/k8s-rdma-shared-dp

ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy

RUN apk add --update --virtual build-dependencies build-base linux-headers git && \
    cd /usr/src/k8s-rdma-shared-dp && \
    make clean && \
    make build

FROM ${BASE_IMAGE}
RUN apk add kmod hwdata-pci
COPY --from=builder /usr/src/k8s-rdma-shared-dp/build/k8s-rdma-shared-dp /bin/

LABEL io.k8s.display-name="RDMA Shared Device Plugin"

CMD ["/bin/k8s-rdma-shared-dp"]
