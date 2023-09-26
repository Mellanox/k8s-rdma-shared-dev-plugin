[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mellanox/k8s-rdma-shared-dev-plugin)](https://goreportcard.com/report/github.com/Mellanox/rdma-cni)
[![Build Status](https://travis-ci.com/Mellanox/k8s-rdma-shared-dev-plugin.svg?branch=master)](https://travis-ci.com/Mellanox/k8s-rdma-shared-dev-plugin)
[![Coverage Status](https://coveralls.io/repos/github/Mellanox/k8s-rdma-shared-dev-plugin/badge.svg)](https://coveralls.io/github/Mellanox/k8s-rdma-shared-dev-plugin)

# k8s-rdma-shared-dev-plugin

(https://hub.docker.com/r/mellanox/k8s-rdma-shared-dev-plugin)

This is simple rdma device plugin that support IB and RoCE HCA. This plugin runs as daemonset. Its container image is
available at mellanox/k8s-rdma-shared-dev-plugin.

# How to use device plugin

**1.** Use CNI plugin such as Contiv, Calico, Cluster

Make sure to configure ib0 or appropriate IPoIB netdevice as the parent netdevice for creating overlay/virtual
netdevices.

**2.** Create ConfigMap and deploy Device Plugin

Deploy device plugin and create config map to describe mode as "hca" mode. This is per node configuration.

```
cd deployment/k8s/base
kubectl apply -k .
```

**3.** Create Test pod

Create test pod which requests 1 vhca resource.

```
kubectl create -f example/test-hca-pod.yaml
```

### Deploy the device plugin with CDI support
To use the device plugin with [CDI](https://github.com/cncf-tags/container-device-interface) support, do the following:
```
cd deployment/k8s/base/overlay
kubectl apply -k .
```
# How to use device plugin for RDMA

The device plugin can be used with macvlan for RDMA, to do the following steps:

**1.** use macvlan cni

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

# RDMA Shared Device Plugin Configurations

The plugin has several configuration fields, this section explains each field usage

```json
{
  "periodicUpdateInterval": 300,
  "configList": [{
      "resourceName": "hca_shared_devices_a",
      "resourcePrefix": "example_prefix",
      "rdmaHcaMax": 1000,
      "devices": ["ib0", "ib1"]
    },
    {
      "resourceName": "hca_shared_devices_b",
      "rdmaHcaMax": 500,
      "selectors": {
        "vendors": ["15b3"],
        "deviceIDs": ["1017"],
        "ifNames": ["ib3", "ib4"]
      }
    }
  ]
}
```

`periodicUpdateInterval` is the time interval in seconds to update the resources according to host devices in case of
changes. Notes:

- if `periodicUpdateInterval` is 0 then periodic update for host devices will be disabled.
- if `periodicUpdateInterval` is not set then default periodic update interval of 60 seconds will be used.

`"configList"` should contain a list of config objects. Each config object may consist of following fields:

| Field            | Required | Description                                                                                                                       |     Type         | Default value | Example                                                 |
|------------------|----------|-----------------------------------------------------------------------------------------------------------------------------------|------------------|---------------|---------------------------------------------------------|
| "resourceName"   | Y        | Endpoint resource name. Should not contain special characters, must be unique in the scope of the resource prefix                 | string           | -             | "hca_shared_devices_a"                                  |
| "resourcePrefix" | N        | Endpoint resource prefix. Should not contain special characters                                                                   | string           | "rdma"        | "example_prefix"                                        |
| "rdmaHcaMax"     | Y        | Maximum number of RDMA resources that can be provided by the device plugin resource                                               | Integer          | -             | 1000                                                    |
| "selectors"      | N        | A map of device selectors for filtering the devices. refer to [Device Selectors](#devices-selectors) section for more information | json object      | -             | selectors": {"vendors": ["15b3"],"deviceIDs": ["1017"]} |
| "devices"        | N        | A list of devices names to be selected, same as "ifNames" selector                                                                | `string` list    | -             | ["ib0", "ib1"]                                          |

Note: Either `selectors` or `devices` must be specified for a given resource, "selectors" is recommended.

## Devices Selectors

The following selectors are used for filtering the desired devices.

|    Field    |                          Description                           |     Type      |         Example          |
|-------------|----------------------------------------------------------------|---------------|--------------------------|
| "vendors"   | Target device's vendor Hex code as string                      | `string` list | "vendors": ["15b3"]      |
| "deviceIDs" | Target device's device Hex code as string                      | `string` list | "devices": ["1017"]      |
| "drivers"   | Target device driver names as string                           | `string` list | "drivers": ["mlx5_core"] |
| "ifNames"   | Target device name                                             | `string` list | "ifNames": ["enp2s2f0"]  |
| "linkTypes" | The link type of the net device associated with the PCI device | `string` list | "linkTypes": ["ether"]   |

[//]: # (The tables above generated using: https://ozh.github.io/ascii-tables/)

## Selectors Matching Process

The device plugin filters the host devices based on the provided selectors, if there are any missing selectors, the
device plugin ignores them. Device plugin performs logical OR between elements of a specific selector and logical AND is
performed between selectors.

# RDMA shared device plugin deployment with node labels

RDMA shared device plugin should be deployed on nodes that:

1. Have RDMA capable hardware
2. RDMA kernel stack is loaded

To allow proper node selection [Node Feature Discovery (NFD)](https://github.com/kubernetes-sigs/node-feature-discovery)
can be used to discover the node capabilities, and expose them as node labels.

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

RDMA device plugin can then be deployed on nodes with `feature.node.kubernetes.io/custom-rdma.available=true`, which
indicates that the node is RDMA capable and RDMA modules are loaded.

# Docker image

RDMA shared device plugin uses `alpine` base image by default. To build RDMA shared device plugin with another base
image you need to pass `BASE_IMAGE` argument:

```
docker build -t k8s-rdma-shared-dev-plugin \
--build-arg BASE_IMAGE=registry.access.redhat.com/ubi8/ubi-minimal:latest \
.
```

> __Note:__ Building image with alpine v3.14.x requires Docker 20.10.0 or newer. for more information refer to
[Alpine 3.14.0 Release Notes](https://wiki.alpinelinux.org/wiki/Release_Notes_for_Alpine_3.14.0#faccessat2)

