// Copyright 2021 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package config

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	kserveapi "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	EnvEtcdSecretName     = "ETCD_SECRET_NAME"
	DefaultEtcdSecretName = "model-serving-etcd"

	ConfigType        = "yaml"
	MountLocation     = "/etc/model-serving/config/default"
	ViperKeyDelimiter = "::"
)

var (
	defaultConfig *viper.Viper
	configLog     = ctrl.Log.WithName("config")
)

// Config holds process global configuration information
type Config struct {
	// System config
	EtcdSecretName    string // DEPRECATED - should be removed in the future
	ModelMeshEndpoint string // For dev use only
	AllowAnyPVC       bool

	// Service config
	InferenceServiceName    string
	InferenceServicePort    uint16
	TLS                     TLSConfig
	HeadlessService         bool
	GrpcMaxMessageSizeBytes int

	// Runtimes config
	ModelMeshImage         ImageConfig
	ModelMeshResources     ResourceRequirements
	RESTProxy              RESTProxyConfig
	StorageHelperImage     ImageConfig
	StorageHelperResources ResourceRequirements
	PodsPerRuntime         uint16
	StorageSecretName      string
	EnableAccessLogging    bool
	BuiltInServerTypes     []string
	PayloadProcessors      []string

	ServiceAccountName string

	Metrics     PrometheusConfig
	ScaleToZero ScaleToZeroConfig

	RuntimePodLabels      map[string]string
	RuntimePodAnnotations map[string]string

	ImagePullSecrets []corev1.LocalObjectReference

	// For internal use only
	InternalModelMeshEnvVars EnvVarList
}

type EnvVarList []EnvVar

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (evl EnvVarList) ToKubernetesType() []corev1.EnvVar {
	env := make([]corev1.EnvVar, len(evl))
	for idx, e := range evl {
		env[idx] = corev1.EnvVar{
			Name:  e.Name,
			Value: e.Value,
		}
	}
	return env
}

type PrometheusConfig struct {
	Enabled                          bool
	Port                             uint16
	Scheme                           string
	DisablePrometheusOperatorSupport bool
}

type ScaleToZeroConfig struct {
	Enabled bool
	// how long to wait after the last predictor assigned to a runtime is deleted
	// before scaling to zero
	GracePeriodSeconds uint16
}

type TLSConfig struct {
	// TLS disabled if omitted
	SecretName string
	// Mutual TLS disabled if omitted
	ClientAuth string
}

type RESTProxyConfig struct {
	Enabled   bool
	Port      uint16
	Image     ImageConfig
	Resources ResourceRequirements
}

func (c *Config) GetEtcdSecretName() string {
	secretName, found := os.LookupEnv(EnvEtcdSecretName)
	if !found {
		secretName = DefaultEtcdSecretName
	}

	// for backward compatability with the old configmap - should be removed in the future
	if secretName == DefaultEtcdSecretName && c.EtcdSecretName != "" {
		secretName = c.EtcdSecretName
	}

	return secretName
}

// ConfigProvider provides immutable snapshots of current config
type ConfigProvider struct {
	config                unsafe.Pointer
	c                     sync.Cond
	isReloading           bool
	loadedResourceVersion string
}

func NewConfigProvider(ctx context.Context, cl client.Client, name types.NamespacedName) (*ConfigProvider, error) {
	// Perform initial load of the default configuration
	config, err := NewMergedConfigFromString("")
	if err != nil {
		return nil, err
	}

	cp := &ConfigProvider{c: sync.Cond{L: &sync.Mutex{}}, config: (unsafe.Pointer)(config)}
	return cp, cp.ReloadConfigMap(ctx, cl, name)
}

func (cp *ConfigProvider) GetConfig() *Config {
	return (*Config)(atomic.LoadPointer(&cp.config))
}

// NewConfigProviderForTest is only for tests
func NewConfigProviderForTest() *ConfigProvider {
	return &ConfigProvider{c: sync.Cond{L: &sync.Mutex{}}}
}

// SetConfigForTest is only for tests
func SetConfigForTest(cp *ConfigProvider, cfg *Config) {
	atomic.StorePointer(&cp.config, (unsafe.Pointer)(cfg))
}

func (cp *ConfigProvider) ReloadConfigMap(ctx context.Context, c client.Client, name types.NamespacedName) error {
	cp.c.L.Lock()
	cp.isReloading = true
	defer func() {
		cp.c.L.Unlock()
		cp.isReloading = false
	}()

	// get the user ConfigMap
	configmap := corev1.ConfigMap{}
	err := c.Get(ctx, name, &configmap)
	// any error other than NotFound is unexpected
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// only reload if the resource version changed from what we last loaded successfully
	//  if the user config does not exist, configmap.ResourceVersion is the empty string
	if configmap.ResourceVersion != cp.loadedResourceVersion {
		var err2 error
		var newConfig *Config
		//  if the configmap was deleted but had been previously loaded
		if configmap.ResourceVersion == "" {
			configLog.Info("User configmap deleted, reverting to defaults", "ConfigMap", name)
			// load from empty string to by-pass validation of the configmap
			newConfig, err2 = NewMergedConfigFromString("")
		} else {
			configLog.Info("Reloading user config", "ConfigMap", name)
			newConfig, err2 = NewMergedConfigFromConfigMap(configmap)
		}
		if err2 != nil {
			return err2
		}
		// update the stored resource version to track changes
		atomic.StorePointer(&cp.config, (unsafe.Pointer)(newConfig))
		cp.loadedResourceVersion = configmap.ResourceVersion
	}

	cp.c.Broadcast()

	return nil
}

// Handler used by controllers which depend on the user configuration
func ConfigWatchHandler(configMapName types.NamespacedName, f func() []reconcile.Request,
	cp *ConfigProvider, kclient *client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
		// Ignore ConfigMaps we don't care about
		if o.GetName() == configMapName.Name && o.GetNamespace() == configMapName.Namespace {
			err := cp.ReloadConfigMap(context.TODO(), *kclient, configMapName)
			if err != nil {
				configLog.Error(err, "Unable to reload user configuration")
			}
			return f()
		}
		return []reconcile.Request{}
	})
}

func (cp *ConfigProvider) IsReloading() bool {
	return cp.isReloading
}

func (cp *ConfigProvider) AwaitReload() {
	cp.c.L.Lock()
	defer cp.c.L.Unlock()

	if cp.isReloading {
		cp.c.Wait()
	}
}

type ImageConfig struct {
	Name    string
	Tag     string
	Digest  string
	Command []string
}

func (m ImageConfig) TaggedImage() string {
	// Use only the digest if it is provided and ignore the tag.
	// It is possible to reference an image by both tag and digest, but this is considered ambiguous and the digest would be used to pull the image anyway.
	// See also https://github.com/cri-o/cri-o/pull/3060
	if m.Digest != "" {
		return fmt.Sprintf("%s@%s", m.Name, m.Digest)
	} else if m.Tag != "" {
		return fmt.Sprintf("%s:%s", m.Name, m.Tag)
	}
	return m.Name
}

type ResourceRequirements struct {
	Requests ResourceQuantities
	Limits   ResourceQuantities
	// used to cache the parsed resources from parseAndValidate()
	parsedKubeResourceRequirements *corev1.ResourceRequirements
}
type ResourceQuantities struct {
	CPU    string
	Memory string
}

func (rr ResourceRequirements) ToKubernetesType() *corev1.ResourceRequirements {
	// assert that parseAndValidate has already been called
	if rr.parsedKubeResourceRequirements == nil {
		panic("ResourceRequirements: Must call parseAndValidate() before ToKubernetesType()")
	}

	return rr.parsedKubeResourceRequirements
}

func (rr *ResourceRequirements) parseAndValidate() error {
	var err error
	var limitsCPU, requestsCPU, limitsMemory, requestsMemory resource.Quantity

	if limitsCPU, err = resource.ParseQuantity(rr.Limits.CPU); err != nil {
		return fmt.Errorf("Cannot parse 'Limits.CPU' as a quantity: %w", err)
	}
	if requestsCPU, err = resource.ParseQuantity(rr.Requests.CPU); err != nil {
		return fmt.Errorf("Cannot parse 'Requests.CPU' a quantity: %w", err)
	}

	if limitsMemory, err = resource.ParseQuantity(rr.Limits.Memory); err != nil {
		return fmt.Errorf("Cannot parse 'Limits.Memory' as a quantity: %w", err)
	}
	if requestsMemory, err = resource.ParseQuantity(rr.Requests.Memory); err != nil {
		return fmt.Errorf("Cannot parse 'Requests.Memory' a quantity: %w", err)
	}

	rr.parsedKubeResourceRequirements = &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    limitsCPU,
			corev1.ResourceMemory: limitsMemory,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    requestsCPU,
			corev1.ResourceMemory: requestsMemory,
		},
	}

	return nil
}

// Sets the defaults prior to config file parsing
// Note: see also config/default/defaults.yaml for the shipped default config
func defaults(v *viper.Viper) {
	v.SetDefault("InferenceServiceName", "modelmesh-serving")
	v.SetDefault("InferenceServicePort", 8033)
	v.SetDefault("PodsPerRuntime", 2)
	v.SetDefault("StorageSecretName", "storage-config")
	v.SetDefault("ServiceAccountName", "")
	v.SetDefault("PayloadProcessors", []string{})
	v.SetDefault(concatStringsWithDelimiter([]string{"Metrics", "Port"}), 2112)
	v.SetDefault(concatStringsWithDelimiter([]string{"Metrics", "Scheme"}), "https")
	v.SetDefault(concatStringsWithDelimiter([]string{"ScaleToZero", "Enabled"}), true)
	v.SetDefault(concatStringsWithDelimiter([]string{"ScaleToZero", "GracePeriodSeconds"}), 60)
	// default size 16MiB in bytes
	v.SetDefault("GrpcMaxMessageSizeBytes", 16777216)
	v.SetDefault("BuiltInServerTypes", []string{
		string(kserveapi.MLServer), string(kserveapi.Triton), string(kserveapi.OVMS), "torchserve",
	})
}

func concatStringsWithDelimiter(elems []string) string {
	return strings.Join(elems, ViperKeyDelimiter)
}

func init() {
	defaultConfig = viper.NewWithOptions(viper.KeyDelimiter(ViperKeyDelimiter))

	defaults(defaultConfig)

	defaultConfig.SetConfigName("config-defaults")
	defaultConfig.SetConfigType(ConfigType)
	defaultConfig.AddConfigPath(MountLocation)

	//For dev env and tests, must get the config filename using cwd relative path
	if _, filename, _, ok := runtime.Caller(0); !ok {
		panic("Unable to get the caller")
	} else {
		filepath := path.Join(path.Dir(filename), "../../config/default")
		defaultConfig.AddConfigPath(filepath)
	}

	if err := defaultConfig.ReadInConfig(); err != nil {
		configLog.Error(err, "Unable to read the default configuration", "path", MountLocation)
	}
}

func NewMergedConfigFromConfigMap(m corev1.ConfigMap) (*Config, error) {
	configYaml, ok := m.Data["config.yaml"]
	if !ok {
		return nil, fmt.Errorf("User ConfigMap must contain a key named config.yaml")
	}

	return NewMergedConfigFromString(configYaml)
}

func NewMergedConfigFromString(configYaml string) (*Config, error) {
	var err error

	v := viper.NewWithOptions(viper.KeyDelimiter(ViperKeyDelimiter))
	v.SetConfigType(ConfigType)
	for _, key := range defaultConfig.AllKeys() {
		v.SetDefault(key, defaultConfig.Get(key))
	}

	configYamlReader := strings.NewReader(configYaml)
	if err = v.ReadConfig(configYamlReader); err != nil {
		return nil, err
	}

	// Even if the default config has an image digest, a user should be able to
	// override it with a tag (ignoring the default digest)
	// HACK: There should be a better way to do this...
	clearDigestIfTagsDiffer(v, "modelMeshImage")
	clearDigestIfTagsDiffer(v, "storageHelperImage")
	clearDigestIfTagsDiffer(v, "restProxy.image")

	// unmarshal the config into a Config struct
	var config Config
	if err = v.Unmarshal(&config); err != nil {
		return nil, err
	}

	configLog.Info("Updated model serving config", "mergedConfig", config)

	// extra validations on parsed config
	if err = config.ModelMeshResources.parseAndValidate(); err != nil {
		return nil, fmt.Errorf("Invalid config for 'ModelMeshResources': %s", err)
	}
	if err = config.RESTProxy.Resources.parseAndValidate(); err != nil {
		return nil, fmt.Errorf("Invalid config for 'RESTProxy.Resources': %s", err)
	}
	if err = config.StorageHelperResources.parseAndValidate(); err != nil {
		return nil, fmt.Errorf("Invalid config for 'StorageHelperResources': %s", err)
	}

	// check that none of the payload processors contains a space
	for _, processor := range config.PayloadProcessors {
		if strings.Contains(processor, " ") {
			return nil, fmt.Errorf("Error parsing payload processor '%s': endpoint must not contain spaces.", processor)
		}
	}
	return &config, nil
}

func clearDigestIfTagsDiffer(v *viper.Viper, imageConfigField string) {
	tag, digest := imageConfigField+".tag", imageConfigField+".digest"
	if v.GetString(tag) != defaultConfig.GetString(tag) && v.GetString(digest) == defaultConfig.GetString(digest) {
		v.Set(digest, "")
	}
}
