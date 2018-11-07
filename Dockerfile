#FROM centos:centos7
FROM debian:stretch-slim
ADD gopath/src/k8s-rdma-sriov-dev-plugin/bin/k8s-rdma-sriov-dp /bin/k8s-rdma-sriov-dp

CMD ["/bin/k8s-rdma-sriov-dp"]
