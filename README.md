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

# How to use device plugin for RDMA

The device plugin can be used with macvlan for RDMA, to do the following steps:

**2.** use macvlan cni

```
# cat > /etc/cni/net.d/00-macvlan.conf <<EOF
{
    "cniVersion": "0.3.1",
    "name": "mynet",
    "type": "macvlan",
     "master": "enp0s0f0",
        "ipam": {
                "type": "host-local",
                "subnet": "10.56.217.0/24",
                "rangeStart": "10.56.217.171",
                "rangeEnd": "10.56.217.181",
                "routes": [
                        { "dst": "0.0.0.0/0" }
                ],
                "gateway": "10.56.217.1"
        }
}

EOF
```

**2.** Follow the steps in the previous section to deploy the device plugin

**3.** Deploy RDMA pod application

```
kubectl create -f <rdma-app.yaml>
```