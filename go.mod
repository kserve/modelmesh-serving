module github.com/kserve/modelmesh-serving

go 1.16

require (
	github.com/dereklstinson/cifar v0.0.0-20200421171932-5722a3b6a0c7
	github.com/go-logr/logr v0.4.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/manifestival/controller-runtime-client v0.4.0
	github.com/manifestival/manifestival v0.7.0
	github.com/moverest/mnist v0.0.0-20160628192128-ec5d9d203b59
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	github.com/operator-framework/operator-lib v0.6.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.49.0
	github.com/prometheus/common v0.30.0 // indirect
	github.com/prometheus/procfs v0.7.1 // indirect
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/tommy351/goldga v0.3.0
	go.etcd.io/etcd/api/v3 v3.5.0
	go.etcd.io/etcd/client/v3 v3.5.0
	golang.org/x/net v0.0.0-20210726213435-c6fcb2dbf985 // indirect
	google.golang.org/genproto v0.0.0-20210728212813-7823e685a01f // indirect
	google.golang.org/grpc v1.39.0
	google.golang.org/protobuf v1.27.1
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/kube-openapi v0.0.0-20210305164622-f622666832c1 // indirect
	sigs.k8s.io/controller-runtime v0.9.5
	sigs.k8s.io/yaml v1.2.0
)

replace go.uber.org/atomic => github.com/uber-go/atomic v1.9.0
