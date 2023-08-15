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
	mfc "github.com/manifestival/controller-runtime-client"
	mf "github.com/manifestival/manifestival"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Manifest(cl client.Client, templatePath string, context interface{}) (mf.Manifest, error) {
	m, err := mf.ManifestFrom(PathTemplateSource(templatePath, context))
	if err != nil {
		return mf.Manifest{}, err
	}
	m.Client = mfc.NewClient(cl)

	return m, err
}

func Apply(cl client.Client, owner metav1.Object, templatePath string, context interface{}, fns ...mf.Transformer) error {
	m, err := mf.ManifestFrom(PathTemplateSource(templatePath, context))
	if err != nil {
		return err
	}
	m.Client = mfc.NewClient(cl)

	if owner != nil {
		asMfOwner := owner.(mf.Owner)
		fns = append(fns, mf.InjectOwner(asMfOwner))
		fns = append(fns, mf.InjectNamespace(asMfOwner.GetNamespace()))
	}
	m, err = m.Transform(fns...)
	if err != nil {
		return err
	}
	err = m.Apply()
	if err != nil {
		return err
	}

	return nil
}

func Delete(cl client.Client, owner metav1.Object, templatePath string, context interface{}, namespace string, fns ...mf.Transformer) error {
	m, err := mf.ManifestFrom(PathTemplateSource(templatePath, context))
	if err != nil {
		return err
	}
	m.Client = mfc.NewClient(cl)

	if owner != nil {
		asMfOwner := owner.(mf.Owner)
		fns = append(fns, mf.InjectOwner(asMfOwner))
		fns = append(fns, mf.InjectNamespace(namespace))
	}
	m, err = m.Transform(fns...)
	if err != nil {
		return err
	}
	err = m.Delete()
	if err != nil {
		return err
	}

	return nil
}

// func updateDeployment(resource *unstructured.Unstructured) error {
// 	if resource.GetKind() != "Deployment" {
// 		return nil
// 	}
// 	// Either manipulate the Unstructured resource directly or...
// 	// convert it to a structured type...
// 	var deployment = &appsv1.Deployment{}
// 	if err := scheme.Scheme.Convert(resource, deployment, nil); err != nil {
// 		return err
// 	}

// 	// Now update the deployment!

// 	// If you converted it, convert it back, otherwise return nil
// 	return scheme.Scheme.Convert(deployment, resource, nil)
// }

// func loadManifest(client client.Client) {
// 	m, err := mf.ManifestFrom(PathTemplateSource("../../../config/internal/model-mesh/deployment.yaml", nil))
// 	m, err = m.Transform(mf.InjectNamespace("default"))
// 	m.Client = mfc.NewClient(client)
// 	fmt.Println(m.Client, err)

// 	fmt.Println("Client ", m.Client)

// 	err = m.Apply()

// 	//err = m.Apply()
// 	fmt.Println(err)
// }
