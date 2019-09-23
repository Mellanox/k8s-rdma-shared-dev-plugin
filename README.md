# k8s-rdma-shared-dev-plugin
(https://hub.docker.com/r/rdma/k8s-rdma-shared-dev-plugin)

This is simple rdma device plugin that support IB and RoCE HCA.
This plugin runs as daemonset.
Its container image is available at rdma/k8s-rdma-shared-dev-plugin.

# How to use device plugin

**1.** Use CNI plugin such as Contiv, Calico, Cluster

Make sure to configure ib0 or appropriate IPoIB netdevice as the parent netdevice for creating overlay/virtual netdevices.

**2.** Create ConfigMap

Create config map to describe mode as "hca" mode.
This is per node configuration.

```
kubectl create -f example/rdma-hca-node-config.yaml
```

**3.** Deploy device plugin

```
kubectl create -f example/device-plugin.yaml
```

**4.** Create Test pod

Create test pod which requests 1 vhca resource.
```
kubectl create -f example/test-hca-pod.yaml
```
