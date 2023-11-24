# TLS Configuration

TLS can be configured via the `tls.secretName` and `tls.clientAuth` parameters of the [global ConfigMap](./README.md). `tls.secretName` should be set to the name of a Kubernetes secret containing a PEM encoded key pair and optionally a CA certificate. The private key must be encoded with PKCS8.

When TLS is enabled for the external inferencing interface, all of the ModelMesh Serving internal (intra-Pod) communication will be secured using the same certificates. The internal links will use mutual TLS regardless of whether client authentication is required for the external connections.

There are various ways to generate TLS certificates. Below are steps on how to do this using OpenSSL or CertManager.

## Generating TLS Certificates for Dev/Test using OpenSSL

First, define the variables that will be used in the commands below. Change the values to suit your environment:

```shell
NAMESPACE="modelmesh-serving"  # the controller namespace where ModelMesh Serving was deployed
SECRET_NAME="modelmesh-certificate"
```

Create an OpenSSL configuration file named `openssl-san.config`:

```shell
cat > openssl-san.config << EOF
[ req ]
distinguished_name = req
[ san ]
subjectAltName = DNS:modelmesh-serving.${NAMESPACE},DNS:localhost,IP:0.0.0.0
EOF
```

Use the following command to create a SAN key/cert:

```shell
openssl req -x509 -newkey rsa:4096 -sha256 -days 3560 -nodes \
    -keyout server.key \
    -out server.crt \
    -subj "/CN=${NAMESPACE}" \
    -extensions san \
    -config openssl-san.config
```

From there, you can create a secret using the generated certificate and key:

```shell
kubectl apply -f - <<EOF
---
apiVersion: v1
kind: Secret
metadata:
  namespace: ${NAMESPACE}
  name: ${SECRET_NAME}
type: kubernetes.io/tls
stringData:
  tls.crt: $(cat server.crt)
  tls.key: $(cat server.key)
  ca.crt: $(cat server.crt)
EOF
```

**Note:** For basic TLS, only the fields `tls.crt` and `tls.key` are required. For mutual TLS, `ca.crt` should be included and `tls.clientAuth` should be set to `require` in the [`model-serving-config` ConfigMap](./README.md).

Alternatively, you can create this secret imperatively using:

```
kubectl create secret tls ${SECRET_NAME} --cert "server.crt" --key "server.key"
```

## Creating TLS Certificates using CertManager

First, define the variables that will be used in the commands below and change the values as needed.

```shell
NAMESPACE="modelmesh-serving" # the controller namespace where ModelMesh Serving was deployed
SECRET_NAME="modelmesh-certificate"
HOSTNAME=localhost
```

1.  [Install `cert-manager`](https://cert-manager.io/docs/installation/) in the cluster.

2.  Create an `Issuer` CR, modifying its name if needed:

    ```shell
    kubectl apply -f - <<EOF
    ---
    apiVersion: cert-manager.io/v1
    kind: Issuer
    metadata:
      name: modelmesh-serving-selfsigned-issuer
    spec:
      selfSigned: {}
    EOF
    ```

3.  Create a `Certificate` CR:

    ```shell
    kubectl apply -f - <<EOF
    ---
    apiVersion: cert-manager.io/v1
    kind: Certificate
    metadata:
      name: modelmesh-serving-cert
    spec:
      secretName: ${SECRET_NAME}
      duration: 2160h0m0s # 90d
      renewBefore: 360h0m0s # 15d
      commonName: modelmesh-serving
      isCA: true
      privateKey:
        size: 4096
        algorithm: RSA
        encoding: PKCS8
      dnsNames:
      - ${HOSTNAME}
      - modelmesh-serving.${NAMESPACE}
      - modelmesh-serving
      issuerRef:
        name: modelmesh-serving-selfsigned-issuer
        kind: Issuer
    EOF
    ```

    **Note:** `${HOSTNAME}` is optional but should be set when configuring an external Kubernetes Ingress or OpenShift route as described [here](./README.md#exposing-an-external-endpoint-using-an-openshift-route).

    If the certificate request is successful, a TLS secret with the PEM-encoded certs will be created as `modelmesh-serving-cert`, assuming `metadata.name` wasn't modified.

4.  Wait for the certificate to be successfully issued:

    ```shell
    kubectl get certificate/modelmesh-serving-cert --watch
    ```

    Once you see `READY` as `True`, proceed to the next step.

    ```
    NAME                     READY   SECRET                        AGE
    modelmesh-serving-cert   True    modelmesh-certificate         21h
    ```

5.  Enable TLS in ModelMesh Serving by adding a value for `tls.secretName` in the ConfigMap, pointing to the secret created with the TLS key/cert details.

    ```shell
    kubectl create -f - <<EOF
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: model-serving-config
    data:
      config.yaml: |
        tls:
          secretName: ${SECRET_NAME}
    EOF
    ```

6.  Retrieve the `ca.crt` (to be used in clients):

    ```shell
    kubectl get secret ${SECRET_NAME} -o jsonpath="{.data.ca\.crt}" > ca.crt
    ```
