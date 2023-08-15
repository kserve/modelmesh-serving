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

package modelmesh

const (
	ModelMeshContainerName = "mm"
	RESTProxyContainerName = "rest-proxy"

	GrpcPortEnvVar         = "INTERNAL_GRPC_PORT"
	ServeGrpcPortEnvVar    = "INTERNAL_SERVING_GRPC_PORT"
	GrpcUdsPathEnvVar      = "INTERNAL_GRPC_SOCKET_PATH"
	ServeGrpcUdsPathEnvVar = "INTERNAL_SERVING_GRPC_SOCKET_PATH"

	EtcdSecretKey = "etcd_connection"
	EtcdVolume    = "etcd-config"

	ModelsDirVolume = "models-dir"
	SocketVolume    = "domain-socket"

	ConfigStorageMount = "storage-config"

	//The name of the puller container
	PullerContainerName = "puller"

	//The env variable puller uses to configure it's own listen port
	PullerEnvListenPort = "PORT"

	//The env variable puller uses to configure the target model serving port
	PullerEnvModelServerEndpoint = "MODEL_SERVER_ENDPOINT"

	//The env variable puller uses to configure the models dir
	PullerEnvModelDir = "ROOT_MODEL_DIR"

	//The env variable puller uses to configure the config dir (secrets)
	PullerEnvStorageConfigDir = "STORAGE_CONFIG_DIR"

	//The env variable puller uses to configure the pvc dir (secrets)
	PullerEnvPVCDir = "PVC_MOUNTS_DIR"

	//The puller default port number
	PullerPortNumber = 8086

	//The puller model mount path
	PullerModelPath = "/models"

	//The puller model PVC path
	DefaultPVCMountsDir = "/pvc_mounts"

	//The puller model config path
	PullerConfigPath = "/storage-config"

	InternalConfigMapName = "tc-config"

	// uds path volume mount name for puller containers
	udsVolMountName = "domain-socket"
)
