# Connecting to an External Identity Provider

To connect your local deployment to an external IDP such as one of the Prodstack IAM instances, you can follow these steps:

1. In `k8s/deployment.yaml`:
    - update the value for `SAML_PROVIDER_HYDRA_PUBLIC_URL` to point to the deployment URL, and
    - update the value for `SAML_PROVIDER_OIDC_CLIENT_ID` to your client id
2. In `k8s/kustomization.yaml`:
    - update the value for `client-secret` in `hydra-credentials` to your client secret
