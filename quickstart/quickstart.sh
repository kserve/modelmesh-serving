MODELMESH_PROJECT=${1:-"opendatahub"}
INFERENCE_SERVICE_PROJECT=${2:-"mesh-test"}

oc new-project $MODELMESH_PROJECT
oc apply -f ../manifests/kfdef.yaml -n $MODELMESH_PROJECT

oc new-project $INFERENCE_SERVICE_PROJECT

SECRETKEY=$(openssl rand -hex 32)
sed "s/<secretkey>/$SECRETKEY/g" sample-minio.yaml > minio.yaml

oc apply -f minio.yaml -n $INFERENCE_SERVICE_PROJECT
oc apply -f triton.yaml -n $INFERENCE_SERVICE_PROJECT

rm minio.yaml

oc apply -f service_account.yaml -n $INFERENCE_SERVICE_PROJECT
