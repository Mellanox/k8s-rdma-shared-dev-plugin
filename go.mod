module github.com/Mellanox/k8s-rdma-shared-dev-plugin

go 1.13

require (
	github.com/Mellanox/rdmamap v1.0.0
	github.com/fsnotify/fsnotify v1.4.7
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/jaypipes/ghw v0.6.1
	github.com/jaypipes/pcidb v0.5.0
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/stretchr/objx v0.1.1 // indirect
	github.com/stretchr/testify v1.4.0
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	golang.org/x/sys v0.0.0 // indirect
	golang.org/x/text v0.3.3 // indirect
	golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543 // indirect
	google.golang.org/genproto v0.0.0 // indirect
	google.golang.org/grpc v1.26.0
	gopkg.in/yaml.v2 v2.2.7 // indirect
	k8s.io/kubelet v0.17.2
)

replace (
	golang.org/x/sys v0.0.0 => github.com/golang/sys v0.0.0-20190813064441-fde4db37ae7a
	google.golang.org/genproto v0.0.0 => github.com/googleapis/go-genproto v0.0.0-20200117163144-32f20d992d24
)
