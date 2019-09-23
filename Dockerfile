FROM debian:stretch-slim

ADD /bin/k8s-rdma-shared-dp /bin/k8s-rdma-shared-dp

CMD ["/bin/k8s-rdma-shared-dp"]
