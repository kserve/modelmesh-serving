Prerequisites:  
* The ODH operator is already installed
* If you want to use the minio installation in these scripts, 
you'll need openssl installed which is used to generate the secret key

Run the script at `quickstart.sh`

```bash
❯ ./quickstart.sh
Error from server (AlreadyExists): project.project.openshift.io "opendatahub" already exists
kfdef.kfdef.apps.kubeflow.org/odh-modelmesh created
Error from server (AlreadyExists): project.project.openshift.io "mesh-test" already exists
service/minio created
Warning: would violate PodSecurity "restricted:v1.24": allowPrivilegeEscalation != false (container "minio" must set securityContext.allowPrivilegeEscalation=false), unrestricted capabilities (container "minio" must set securityContext.capabilities.drop=["ALL"]), runAsNonRoot != true (pod or container "minio" must set securityContext.runAsNonRoot=true), seccompProfile (pod or container "minio" must set securityContext.seccompProfile.type to "RuntimeDefault" or "Localhost")
pod/minio created
secret/storage-config created
inferenceservice.serving.kserve.io/example-onnx-mnist created
serviceaccount/user-one created
Warning: resource rolebindings/user-one-view is missing the kubectl.kubernetes.io/last-applied-configuration annotation which is required by oc apply. oc apply should only be used on resources created declaratively by either oc create --save-config or oc apply. The missing annotation will be patched automatically.
rolebinding.rbac.authorization.k8s.io/user-one-view configured
```

You can specify custom namespaces for the script as per the following
` ./quickstart.sh <modelmesh_kfdef_namespace> <inference_service_namespace>`

To test the inference when authentication is enabled. You may need to update the specified namespace depending on where you deployed
1. `ROUTE=$(oc get routes -n mesh-test example-onnx-mnist --template={{.spec.host}}{{.spec.path}})`
2. `TOKEN=$(oc sa new-token user-one -n mesh-test)`
2. `curl -k https://$ROUTE/infer -d @input.json -H "Authorization: Bearer $TOKEN" -i`  # will use the sample input for this model

You should see a result that looks like this...
`{"model_name":"example-onnx-mnist__isvc-82e2bf7ea4","model_version":"1","outputs":[{"name":"Plus214_Output_0","datatype":"FP32","shape":[1,10],"data":[-8.233052,-7.749704,-3.4236808,12.363028,-12.079106,17.26659,-10.570972,0.7130786,3.3217115,1.3621225]}]}`