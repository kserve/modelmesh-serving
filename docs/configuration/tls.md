# TLS Configuration

TLS can be configured via the `tls.secretName` and `tls.clientAuth` parameters of the [global ConfigMap](./README.md). `tls.secretName` should be set to the name of a Kubernetes secret containing a PEM encoded key pair and optionally a CA certificate. The private key must be encoded with PKCS8.

When TLS is enabled for the external inferencing interface, all of the ModelMesh Serving internal (intra-Pod) communication will be secured using the same certificates. The internal links will use mutual TLS regardless of whether client authentication is required for the external connections.

There are various ways to generate TLS certificates, below are steps on how to do this using OpenSSL or CertManager.

## Generating TLS Certificates for Dev/Test using OpenSSL

To create a SAN key/cert for TLS, use command:

```shell
openssl req -x509 -newkey rsa:4096 -sha256 -days 3560 -nodes -keyout example.key -out example.crt -subj '/CN=modelmesh-serving' -extensions san -config openssl-san.config
```

Where the contents of `openssl-san.config` look like:

```
[ req ]
distinguished_name = req
[ san ]
subjectAltName = DNS:modelmesh-serving.${NAMESPACE},DNS:localhost,IP:0.0.0.0
```

With the generated key/cert, create a kube secret with contents like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ${SECRET_NAME}
type: kubernetes.io/tls
stringData:
  tls.crt: <contents-of-example.crt>
  tls.key: <contents-of-example.key>
  ca.crt: <contents-of-example.crt>
```

For basic TLS, only the fields `tls.crt` and `tls.key` are needed in the kube secret. For mutual TLS, add `ca.crt` in the kube secret and set the configuration `tls.clientAuth` to `require` in the ConfigMap `model-serving-config`.

## Creating TLS Certificates using CertManager

1.  If necessary, install `cert-manager` in the cluster - follow the steps here: https://cert-manager.io/docs/installation/.

2.  Create an `Issuer` CR

        kubectl apply -f - <<EOF
        apiVersion: cert-manager.io/v1
        kind: Issuer
        metadata:
          name: modelmesh-serving-selfsigned-issuer
        spec:
          selfSigned: {}
        EOF

3.  Create a `Certificate` CR

        kubectl apply -f - <<EOF
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

    Where `${NAMESPACE}` is the namespace where the ModelMesh Serving Service resides, and `modelmesh-serving` is the name of that service (configured via the `inferenceServiceName` global ConfigMap parameter).

    Replace `modelmesh-serving-selfsigned-issuer` by the name of the issuer that you're using if needed (see previous step).

    `${HOSTNAME}` is optional but should be set as follows when configuring an external Kubernetes Ingress or OpenShift route as described [here](./README.md#exposing-an-external-endpoint-using-an-openshift-route):

        HOSTNAME=`oc get route modelmesh-serving -o jsonpath='{.spec.host}'`

    If the certificate request is successful, a TLS secret with the PEM-encoded certs will be created as `${SECRET_NAME}`.

4.  Wait for the certificate to be successfully issued

        kubectl get certificate/${SECRET_NAME} --watch

    Once you see `Ready` as `True`, proceed to the next step.

        NAME                     READY   SECRET                        AGE
        modelmesh-serving-cert   True    ${SECRET_NAME}                21h

5.  Enable TLS in ModelMesh Serving

    As explained before, TLS is enabled through adding a value for `tls.secretName` in the user's ConfigMap that points to an existing kube secret with TLS key/cert details.

    So in this case, it would be `${SECRET_NAME}`, which gets created once the `certificate` is `ready`.

    **Example:**

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

6.  Retrieve the `ca.crt` (to be used in clients)

        kubectl get secret ${SECRET_NAME} -o jsonpath="{.data.ca\.crt}" > ca.crt
