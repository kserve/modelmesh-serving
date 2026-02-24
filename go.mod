module github.com/kserve/modelmesh-serving

go 1.25.7

require (
	github.com/dereklstinson/cifar v0.0.0-20200421171932-5722a3b6a0c7
	github.com/go-logr/logr v1.4.1
	github.com/golang/protobuf v1.5.4
	github.com/google/go-cmp v0.6.0
	github.com/kserve/kserve v0.12.0
	github.com/manifestival/controller-runtime-client v0.4.0
	github.com/manifestival/manifestival v0.7.1
	github.com/moverest/mnist v0.0.0-20160628192128-ec5d9d203b59
	github.com/onsi/ginkgo/v2 v2.15.0
	github.com/onsi/gomega v1.31.0
	github.com/operator-framework/operator-lib v0.10.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.55.0
	github.com/spf13/viper v1.11.0
	github.com/stretchr/testify v1.8.4
	github.com/tommy351/goldga v0.5.0
	go.etcd.io/etcd/api/v3 v3.5.9
	go.etcd.io/etcd/client/v3 v3.5.9
	go.uber.org/atomic v1.11.0
	google.golang.org/grpc v1.59.0
	google.golang.org/protobuf v1.33.0
	k8s.io/api v0.28.4
	k8s.io/apimachinery v0.30.13
	k8s.io/client-go v0.28.4
	knative.dev/pkg v0.0.0-20231115001034-97c7258e3a98
	sigs.k8s.io/controller-runtime v0.16.3
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	golang.org/x/exp v0.0.0-20231110203233-9a3e6036ecaa // indirect
	golang.org/x/sync v0.19.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231106174013-bbf56f31fb17 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231106174013-bbf56f31fb17 // indirect
)

// when adding or removing to the replace-ments below, remove the following block of
// indirect dependencies and run `go mod tidy -compat=1.21` (see go version above)
require (
	cloud.google.com/go v0.110.10 // indirect
	cloud.google.com/go/compute v1.23.3 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/iam v1.1.5 // indirect
	cloud.google.com/go/storage v1.35.1 // indirect
	github.com/BurntSushi/toml v1.0.0 // indirect
	github.com/andreyvit/diff v0.0.0-20170406064948-c7f18ee00883 // indirect
	github.com/aws/aws-sdk-go v1.48.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blendle/zapdriver v1.3.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch v5.7.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.7.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-containerregistry v0.16.1 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20210720184732-4bb14d4b1be1 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/google/uuid v1.4.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.0 // indirect
	github.com/googleapis/google-cloud-go-testing v0.0.0-20210719221736-1c9a4c676720 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/logrusorgru/aurora/v3 v3.0.0 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pelletier/go-toml/v2 v2.0.0-beta.8 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.17.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/spf13/afero v1.8.2 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.9 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.26.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/oauth2 v0.14.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/term v0.27.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.4.0 // indirect
	golang.org/x/tools v0.42.0 // indirect
	golang.org/x/xerrors v0.0.0-20231012003039-104605ab7028 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/api v0.151.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto v0.0.0-20231106174013-bbf56f31fb17 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.66.4 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.28.4 // indirect
	k8s.io/component-base v0.28.4 // indirect
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b // indirect
	knative.dev/networking v0.0.0-20231115015815-3af9769712cd // indirect
	knative.dev/serving v0.39.3 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
)

replace (
	// Fixes CVE-2022-21698 and CVE-2023-45142
	// this dependency comes from k8s.io/component-base@v0.26.4 and k8s.io/apiextensions-apiserver@v0.26.4
	// before removing it make sure that the next version of the related k8s dependencies contains the fix
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp => go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.44.0

	// Fixes CVE-2024-45338
	golang.org/x/net => golang.org/x/net v0.33.0
)
