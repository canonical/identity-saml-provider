#!/bin/sh

# The script requires:
# - rockcraft
# - skopeo with sudo privilege
# - yq
# - docker

set -e

# rockcraft clean
rockcraft pack -v


echo "$IMAGE built"

if [ "${PUSH_IMAGE}" = "true" ]; then
  skopeo --insecure-policy copy oci-archive:identity-saml-provider_$(yq -r '.version' rockcraft.yaml)_amd64.rock docker://$IMAGE --dest-tls-verify=false
  echo "$IMAGE pushed"
fi
