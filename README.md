# k8s-rdma-sriov-dev-plugin
(https://hub.docker.com/r/rdma/k8s-rdma-sriov-dev-plugin)

This is simple rdma device plugin that support IB and RoCE SRIOV vHCA and HCA.
This also support DPDK applications for Mellanox NICs.
This plugin runs as daemonset.
Its container image is available at rdma/k8s-rdma-sriov-dev-plugin.

# How to use SRIOV mode?

**1.** Create per node sriov configuration

Edit example/sriov/rdma-sriov-node-config.yaml to describe sriov PF netdevice(s).
In this example it is eth0 and eth1.

**Note:**
(a) Do not add any VFs.
(b) Do not enable SRIOV manually.

This plugin enables SRIOV for a given PF and does necessary configuration for IB and RoCE link layers.

**2.** Create ConfigMap

Create config map which holds SRIOV netdevice information from this config yaml
file. This is per node configuration.
In below example, they are eth0 and eth1 in rdma-sriov-node-config.yaml.

```
kubectl create -f example/sriov/rdma-sriov-node-config.yaml
```

**3.** Deploy device plugin

```
kubectl create -f example/device-plugin.yaml
```

**4.** Create Test pod

Create test pod which requests 1 vhca resource.
```
kubectl create -f example/sriov/test-sriov-pod.yaml
```

# How to use HCA mode?

**1.** Use CNI plugin such as Contiv, Calico, Cluster

Make sure to configure ib0 or appropriate IPoIB netdevice as the parent netdevice for creating overlay/virtual netdevices.

**2.** Create ConfigMap

Create config map to describe mode as "hca" mode.
This is per node configuration.

```
kubectl create -f example/hca/rdma-hca-node-config.yaml
```

**3.** Deploy device plugin

```
kubectl create -f example/device-plugin.yaml
```

**4.** Create Test pod

Create test pod which requests 1 vhca resource.
```
kubectl create -f example/hca/test-hca-pod.yaml
```
