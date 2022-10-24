#!/usr/bin/env bash

set -euo pipefail

source example/kind/lib.sh

export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-dash}"
export DATA_FILE=$(realpath example/kind)

echodate "Installing kind"

kind delete cluster --name "$KIND_CLUSTER_NAME"
kind create cluster --name "$KIND_CLUSTER_NAME" --config "$DATA_FILE"/cluster.yaml

echodate "Installing metallb"
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.7/config/manifests/metallb-native.yaml
echodate "Waiting for load balancer to be ready..."
retry 10 check_all_deployments_ready metallb-system
echodate "Load balancer is ready."
kubectl apply -f "$DATA_FILE"/metallb-config.yaml
echodate "Deploy CRD"
kubectl create -f config/crd/bases/
echodate "Deploy ingress"
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s