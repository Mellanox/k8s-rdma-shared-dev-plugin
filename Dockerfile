FROM debian:stretch-slim

ADD /bin/k8s-rdma-sriov-dp /bin/k8s-rdma-sriov-dp

CMD ["/bin/k8s-rdma-sriov-dp"]
