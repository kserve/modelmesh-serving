// Copyright 2021 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package fvt

import (
	"github.com/go-logr/logr"
)

var Log logr.Logger
var FVTClientInstance *FVTClient

var DefaultTimeout = int64(120) // absolute timeout for watcher event channels
var NameSpaceScopeMode = false

var DefaultConfig = map[string]interface{}{
	"podsPerRuntime": 1,
	"restProxy": map[string]interface{}{
		"enabled": true,
	},
	"scaleToZero": map[string]interface{}{
		"enabled": false,
	},
	"internalModelMeshEnvVars": []map[string]interface{}{
		{
			"name":  "BOOTSTRAP_CLEARANCE_PERIOD_MS",
			"value": "0",
		},
	},
}

var StorageConfigDataMinio = map[string]interface{}{
	"localMinIO": map[string]string{
		"type":              "s3",
		"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
		"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"endpoint_url":      "http://minio:9000",
		"default_bucket":    "modelmesh-example-models",
		"region":            "us-south",
	},
}

var StorageConfigDataPVC = map[string]interface{}{
	"pvc1": map[string]string{
		"type": "pvc",
		"name": "models-pvc-1",
	},
	"pvc2": map[string]string{
		"type": "pvc",
		"name": "models-pvc-2",
	},
}

var BasicTLSConfig = map[string]interface{}{
	"tls": map[string]interface{}{
		"secretName": TLSSecretName,
		"clientAuth": "optional",
		// Avoid port-forwarding DNS complications
		"headlessService": false,
	},
}

var MutualTLSConfig = map[string]interface{}{
	"tls": map[string]interface{}{
		"secretName": TLSSecretName,
		"clientAuth": "require",
		// Avoid port-forwarding DNS complications
		"headlessService": false,
	},
}

const (
	ServingRuntimeKind         = "ServingRuntime"
	PredictorKind              = "Predictor"
	IsvcKind                   = "InferenceService"
	ConfigMapKind              = "ConfigMap"
	SecretKind                 = "Secret"
	DefaultTestNamespace       = "modelmesh-serving"
	DefaultTestServiceName     = "modelmesh-serving"
	DefaultControllerNamespace = "modelmesh-serving"
	UserConfigMapName          = "model-serving-config"
	SamplesPath                = "predictors/"
	IsvcSamplesPath            = "isvcs/"
	RuntimeSamplesPath         = "runtimes/"
	TLSSecretName              = "fvt-tls-secret"
	StorageConfigSecretName    = "storage-config"
)
