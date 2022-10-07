#!/bin/bash

export DOCKER_COMPOSE_FILE=docker-kubernetes.yml
export DOCKER_COMPOSE_PROJECT_NAME=kubernetes_e2e

errors=0

export KUBECONFIG=./kubeconfig.yaml

echo "Using Docker version:"
docker version

prepare_kubernetes() {
  docker compose -f $DOCKER_COMPOSE_FILE -p $DOCKER_COMPOSE_PROJECT_NAME up -d
  until kubectl get nodes | grep " Ready "; do sleep 1; done
  echo "Loading ghcr.io/acouvreur/sablier:local into k3s..."
  docker save ghcr.io/acouvreur/sablier:local | docker exec -i ${DOCKER_COMPOSE_PROJECT_NAME}-server-1 ctr images import -
  echo "Loading succeeded."
}

destroy_kubernetes() {
  docker compose -f $DOCKER_COMPOSE_FILE -p $DOCKER_COMPOSE_PROJECT_NAME down --volumes
}

prepare_traefik_and_sablier() {
  helm repo add traefik https://helm.traefik.io/traefik
  helm repo update
  helm install traefik traefik/traefik -f values.yaml --namespace kube-system
  kubectl apply -f ./manifests/sablier.yml
}

prepare_deployment() {
  kubectl apply -f ./manifests/deployment.yml
}

destroy_deployment() {
  kubectl delete -f ./manifests/deployment.yml
}

prepare_stateful_set() {
  kubectl apply -f ./manifests/statefulset.yml
}

destroy_stateful_set() {
  kubectl delete -f ./manifests/statefulset.yml
}

run_kubernetes_deployment_test() {
  echo "Running Kubernetes Test: $1"
  prepare_deployment
  sleep 10
  go clean -testcache
  if ! go test -count=1 -tags e2e -timeout 30s -run ^${1}$ github.com/acouvreur/sablier/e2e; then
    errors=1
    kubectl -n kube-system logs deployments/sablier-deployment
    kubectl -n kube-system logs deployments/traefik
  fi

  destroy_deployment
}

trap destroy_kubernetes EXIT

prepare_kubernetes
prepare_traefik_and_sablier
run_kubernetes_deployment_test Test_Dynamic
run_kubernetes_deployment_test Test_Blocking
run_kubernetes_deployment_test Test_Multiple
run_kubernetes_deployment_test Test_Healthy

exit $errors