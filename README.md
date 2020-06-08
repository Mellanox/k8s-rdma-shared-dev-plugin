[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mellanox/k8s-rdma-shared-dev-plugin)](https://goreportcard.com/report/github.com/Mellanox/rdma-cni)
[![Build Status](https://travis-ci.com/Mellanox/k8s-rdma-shared-dev-plugin.svg?branch=master)](https://travis-ci.com/Mellanox/k8s-rdma-shared-dev-plugin)
[![Coverage Status](https://coveralls.io/repos/github/Mellanox/k8s-rdma-shared-dev-plugin/badge.svg)](https://coveralls.io/github/Mellanox/k8s-rdma-shared-dev-plugin)

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
kubectl create -f images/k8s-rdma-shared-dev-plugin-config-map.yaml
```

**3.** Deploy device plugin

```
kubectl create -f images/k8s-rdma-shared-dev-plugin-ds.yaml
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

# RDMA shared device plugin deployment with node labels

RDMA shared device plugin should be deployed on nodes that:

1. Have RDMA capable hardware
2. RDMA kernel stack is loaded

To allow proper node selection [Node Feature Discovery (NFD)](https://github.com/kubernetes-sigs/node-feature-discovery) can be used to discover the node capabilities, and expose them as node labels.

1. Deploy NFD, release `v0.6.0` or new newer

```
# export NFD_VERSION=v0.6.0
# kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/$NFD_VERSION/nfd-master.yaml.template
# kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/$NFD_VERSION/nfd-worker-daemonset.yaml.template
```

2. Check the new labels added to the node
```
# kubectl get nodes --show-labels
```

RDMA device plugin can then be deployed on nodes with `feature.node.kubernetes.io/custom-rdma.available=true`, which indicates that the node is RDMA capable and RDMA modules are loaded.
