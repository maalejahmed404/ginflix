#!/usr/bin/env bash
# Creates the Kubernetes secrets from the .env file.
# The secrets are not stored as YAML in the repo on purpose.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ ! -f .env ]; then
  cp .env.example .env
  echo "no .env file: created one from .env.example (demo values)"
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

for var in S3_ACCESS_KEY S3_SECRET_KEY KEYCLOAK_ADMIN_USER KEYCLOAK_ADMIN_PASSWORD; do
  if [ -z "${!var:-}" ]; then
    echo "$var is empty in .env" >&2
    exit 1
  fi
done

kubectl apply -f k8s/namespace.yaml

# "create --dry-run | apply" so the script can be run several times

# the application reads the S3 keys in variables named GARAGE_*
kubectl create secret generic s3-credentials -n ginflix \
  --from-literal=GARAGE_ACCESS_KEY="$S3_ACCESS_KEY" \
  --from-literal=GARAGE_SECRET_KEY="$S3_SECRET_KEY" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic keycloak-admin -n ginflix \
  --from-literal=username="$KEYCLOAK_ADMIN_USER" \
  --from-literal=password="$KEYCLOAK_ADMIN_PASSWORD" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "secrets created in namespace ginflix"
