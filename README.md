# rdma-sriov-dp (https://hub.docker.com/r/mellanox/rdma-sriov-dev-plugin)

This is simple rdma device plugin that support IB and RoCE SRIOV vHCA.
This also support DPDK applications for Mellanox NICs.

## How to

**1.** Create ConfigMap
Create config map which holds SRIOV netdevice information.
This is per node configuration.

#Write PF netdevices in the config file. In below example, they are eth0 and eth1.
kubectl create -f rdma-example/rdma-sriov-node-config.yaml

**2.** Deploy device plugin
kubectl create -f device-plugin.yaml

**3.** Create Test pod
Create test pod which requests 1 vhca resource.
kubectl create -f rdma-example/test-pod.yaml
