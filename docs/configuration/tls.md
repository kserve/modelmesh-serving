# TLS Configuration

TLS can be configured via the `tls.secretName` and `tls.clientAuth` parameters of the [global ConfigMap](./README.md). `tls.secretName` should be set to the name of a Kubernetes secret containing a PEM encoded key pair and optionally a CA certificate. The private key must be encoded with PKCS8.

When TLS is enabled for the external inferencing interface, all of the ModelMesh Serving internal (intra-Pod) communication will be secured using the same certificates. The internal links will use mutual TLS regardless of whether client authentication is required for the external connections.

There are various ways to generate TLS certificates. Below are steps on how to do this using OpenSSL or CertManager.

## Generating TLS Certificates for Dev/Test using OpenSSL

To create a SAN key/cert for TLS, use the following command:

```shell
openssl req -x509 -newkey rsa:4096 -sha256 -days 3560 -nodes -keyout example.key -out example.crt -subj '/CN=modelmesh-serving' -extensions san -config openssl-san.config
```

Where the contents of `openssl-san.config` include:

```
[ req ]
distinguished_name = req
[ san ]
subjectAltName = DNS:modelmesh-serving.${NAMESPACE},DNS:localhost,IP:0.0.0.0
```

`${NAMESPACE}` is the namespace where the ModelMesh Serving Service is deployed.

From there, you can create a secret using the generated certificate and key:

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

For basic TLS, only the fields `tls.crt` and `tls.key` are required. For mutual TLS, `ca.crt` should be included and `tls.clientAuth` should be set to `require` in the [`model-serving-config` ConfigMap](./README.md).

You can also create this secret imperatively using:

```
kubectl create secret tls <secret-name> --cert <cert-file> --key <key-file>
```

## Creating TLS Certificates using CertManager

1.  [Install `cert-manager`](https://cert-manager.io/docs/installation/) in the cluster.

2.  Create an `Issuer` CR, modifying its name if needed:

        kubectl apply -f - <<EOF
        apiVersion: cert-manager.io/v1
        kind: Issuer
        metadata:
          name: modelmesh-serving-selfsigned-issuer
        spec:
          selfSigned: {}
        EOF

3.  Create a `Certificate` CR:

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

     Above, `${NAMESPACE}` is the namespace where the ModelMesh Serving Service resides, and `modelmesh-serving` is the name of that service (configured via the `inferenceServiceName` [global ConfigMap](./README.md). parameter). You can also replace `issuerRef.name` to match the name of the issuer used above if necessary. 

    `${HOSTNAME}` is optional, but should be set when configuring an external Kubernetes Ingress or OpenShift route as described [here](./README.md#exposing-an-external-endpoint-using-an-openshift-route):

           HOSTNAME=`oc get route modelmesh-serving -o jsonpath='{.spec.host}'`

    If the certificate request is successful, a TLS secret with the PEM-encoded certs will be created as `modelmesh-serving-cert`, unless changed above.

4.  Wait for the certificate to be successfully issued

        kubectl get certificate/modelmesh-serving-cert --watch

    Once you see `Ready` as `True`, proceed to the next step.

        NAME                     READY   SECRET                        AGE
        modelmesh-serving-cert   True    ${SECRET_NAME}                21h

5.  Enable TLS in ModelMesh Serving

    As explained before, TLS is enabled by adding a value for `tls.secretName` in the ConfigMap, pointing to an existing secret with the TLS key/cert details.

    In this case, it would be `${SECRET_NAME}`, which gets created once the `certificate` is `ready`. For example:

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
