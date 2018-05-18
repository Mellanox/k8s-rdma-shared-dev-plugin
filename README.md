# rdma-sriov-dp (https://hub.docker.com/r/mellanox/rdma-sriov-dev-plugin)

This is simple rdma device plugin that support IB and RoCE SRIOV vHCA.
This also support DPDK applications for Mellanox NICs.
This plugin runs as daemonset.
Its container image is available at mellanox/rdma-sriov-dev-plugin.

## How to

**1.** Create per node sriov configuration

Edit rdma-example/rdma-sriov-node-config.yaml to describe sriov PF netdevice.
In this example it is eth0 and eth1.
Do not add any VFs.

Do not enable SRIOV manually. This plugin enables SRIOV for a given PF and
does necessary configuration for IB and RoCE link layers.

**2.** Create ConfigMap

Create config map which holds SRIOV netdevice information.
This is per node configuration.
In below example, they are eth0 and eth1 in rdma-sriov-node-config.yaml.

```
kubectl create -f rdma-example/rdma-sriov-node-config.yaml
```

**3.** Deploy device plugin

```
kubectl create -f device-plugin.yaml
```

**4.** Create Test pod

Create test pod which requests 1 vhca resource.
```
kubectl create -f rdma-example/test-pod.yaml
```
