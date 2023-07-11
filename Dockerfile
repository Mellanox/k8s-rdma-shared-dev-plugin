FROM golang:alpine as builder

COPY . /usr/src/k8s-rdma-shared-dp

ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy

RUN apk add --no-cache --virtual build-base=0.5-r3 linux-headers=5.19.5-r0
WORKDIR /usr/src/k8s-rdma-shared-dp
RUN make clean && \
    make build

FROM alpine:3.18.2
RUN apk add --no-cache kmod=30-r3 hwdata-pci=0.370-r0
COPY --from=builder /usr/src/k8s-rdma-shared-dp/build/k8s-rdma-shared-dp /bin/

LABEL io.k8s.display-name="RDMA Shared Device Plugin"

CMD ["/bin/k8s-rdma-shared-dp"]
