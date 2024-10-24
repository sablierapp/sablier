#!/bin/bash

export DOCKER_COMPOSE_FILE=compose.yaml
export DOCKER_COMPOSE_PROJECT_NAME=kubernetes_e2e

errors=0

export KUBECONFIG=./kubeconfig.yaml

echo "Using Docker version:"
docker version

prepare_kubernetes() {
  docker compose -f $DOCKER_COMPOSE_FILE -p $DOCKER_COMPOSE_PROJECT_NAME up -d
  until kubectl get nodes | grep " Ready "; do sleep 1; done
  echo "Loading sablierapp/sablier:local into k3s..."
  docker save sablierapp/sablier:local | docker exec -i ${DOCKER_COMPOSE_PROJECT_NAME}-server-1 ctr images import -
  echo "Loading succeeded."
}

destroy_kubernetes() {
  docker compose -f $DOCKER_COMPOSE_FILE -p $DOCKER_COMPOSE_PROJECT_NAME down --volumes
}

prepare_istio() {
  helm repo add istio https://istio-release.storage.googleapis.com/charts
  helm repo update
  kubectl create namespace istio-system
  helm install istio-base istio/base -n istio-system --wait
  helm install istiod istio/istiod -n istio-system --wait
  kubectl label namespace istio-system istio-injection=enabled
  kubectl label namespace default istio-injection=enabled
  kubectl create configmap -n istio-system sablier-wasm-plugin --from-file ../../../sablierproxywasm.wasm
  helm install istio-ingressgateway istio/gateway --values ./istio-gateway-values.yaml -n istio-system --wait
}

prepare_manifests() {
  kubectl apply -f ./manifests
}

destroy_manifests() {
  kubectl delete -f ./manifests
}

run_kubernetes_test() {
  echo "---- Running Kubernetes Test: $1 ----"
  prepare_manifests
  sleep 10
  go clean -testcache
  if ! go test -count=1 -tags e2e -timeout 30s -run ^${1}$ github.com/sablierapp/sablier/e2e; then
    errors=1
    kubectl -n kube-system logs deployments/sablier-deployment
    # kubectl -n kube-system logs deployments/traefik TODO: Log istio
  fi

  destroy_manifests
}

# trap destroy_kubernetes EXIT

prepare_kubernetes
prepare_istio
# run_kubernetes_test Test_Dynamic
# run_kubernetes_test Test_Blocking
# run_kubernetes_test Test_Multiple
# run_kubernetes_test Test_Healthy

# exit $errors
