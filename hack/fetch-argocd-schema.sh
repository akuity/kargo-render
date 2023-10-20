#!/usr/bin/env bash

SCRIPT_PATH=$( cd "$(dirname "$0")" && pwd )
ARGOCD_VERSION=$(cat "$SCRIPT_PATH"/../go.mod | grep github.com/argoproj/argo-cd  | cut -d' ' -f2-)
curl https://raw.githubusercontent.com/argoproj/argo-cd/$ARGOCD_VERSION/manifests/crds/application-crd.yaml | yq -o=json | \
  jq '{"$schema": "http://json-schema.org/draft-07/schema#", "$id": "argocd-schema.json", "definitions": {helm: .spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.source.properties.helm, kustomize: .spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.source.properties.kustomize, plugin: .spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.source.properties.plugin}} ' > \
  "$SCRIPT_PATH"/../argocd-schema.json