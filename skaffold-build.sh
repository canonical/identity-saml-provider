#!/bin/sh

# This script builds the OCI image for the identity-saml-provider plugin and optionally pushes it to a container registry.
# It is meant to be used by Skaffold.

# The script requires:
# - rockcraft
# - skopeo
# - yq
# - an OCI container registry

set -e

# rockcraft clean
rockcraft pack -v


echo "$IMAGE built"

if [ "${PUSH_IMAGE}" = "true" ]; then
  skopeo --insecure-policy copy "oci-archive:identity-saml-provider_$(yq -r '.version' rockcraft.yaml)_amd64.rock" "docker://$IMAGE" --dest-tls-verify=false
  echo "$IMAGE pushed"
fi
