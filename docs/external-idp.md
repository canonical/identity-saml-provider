# Connecting to an External Identity Provider

To connect your local deployment to an external IDP such as one of the
Prodstack IAM instances, you can follow these steps:

1. In `k8s/20-apps/identity-saml-provider.yaml`:
    - update the value for `SAML_PROVIDER_HYDRA_PUBLIC_URL` to point to
      the deployment URL, and
    - update the value for `SAML_PROVIDER_OIDC_CLIENT_ID` to your client
      id
    - if Hydra uses a custom CA chain, set
      `SAML_PROVIDER_HYDRA_CA_CERT_PATH` to the path of a PEM file
      containing the CA certificate(s) and mount the file into the
      container (e.g., via a Kubernetes Secret volume)
2. In `k8s/10-ory/kustomization.yaml`:
    - update the value for `client-secret` in `hydra-credentials` to your
      client secret
