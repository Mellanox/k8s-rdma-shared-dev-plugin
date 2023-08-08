module github.com/Mellanox/k8s-rdma-shared-dev-plugin

go 1.20

require (
	github.com/Mellanox/rdmamap v1.1.0
	github.com/jaypipes/ghw v0.6.1
	github.com/jaypipes/pcidb v1.0.0
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.8.4
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/net v0.12.0
	google.golang.org/grpc v1.56.2
	k8s.io/kubelet v0.27.3
)

require (
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/hpcloud/tail v1.0.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	golang.org/x/sys v0.10.0 // indirect
	golang.org/x/text v0.11.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	howett.net/plist v1.0.0 // indirect
)

replace (
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	golang.org/x/sys v0.0.0 => github.com/golang/sys v0.0.0-20190813064441-fde4db37ae7a
	google.golang.org/genproto v0.0.0 => github.com/googleapis/go-genproto v0.0.0-20200117163144-32f20d992d24
)
