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
  echo "Loading acouvreur/sablier:local into k3s..."
  docker save acouvreur/sablier:local | docker exec -i ${DOCKER_COMPOSE_PROJECT_NAME}-server-1 ctr images import -
  echo "Loading succeeded."
}

destroy_kubernetes() {
  docker compose -f $DOCKER_COMPOSE_FILE -p $DOCKER_COMPOSE_PROJECT_NAME down --volumes
}

prepare_traefik() {
  helm repo add traefik https://traefik.github.io/charts
  helm repo update
  helm install traefik --version 28.3.0 traefik/traefik -f values.yaml --namespace kube-system
}

prepare_sablier() {
  helm install sablier ../../../../helm --set image.tag=local --namespace kube-system
}

prepare_deployment() {
  kubectl apply -f ./manifests/sablier.yml
  kubectl apply -f ./manifests/deployment.yml
}

destroy_deployment() {
  kubectl delete -f ./manifests/deployment.yml
}

destroy_sablier() {
  helm uninstall --namespace kube-system
}

run_kubernetes_deployment_test() {
  echo "---- Running Kubernetes Test: $1 ----"
  prepare_sablier
  prepare_deployment
  sleep 10
  go clean -testcache
  if ! go test -count=1 -tags e2e -timeout 30s -run ^${1}$ github.com/acouvreur/sablier/e2e; then
    errors=1
    kubectl -n kube-system logs deployments/sablier-deployment
    kubectl -n kube-system logs deployments/traefik
  fi

  destroy_deployment
  destroy_sablier
}

trap destroy_kubernetes EXIT

prepare_kubernetes
prepare_traefik
run_kubernetes_deployment_test Test_Dynamic
run_kubernetes_deployment_test Test_Blocking
run_kubernetes_deployment_test Test_Multiple
run_kubernetes_deployment_test Test_Healthy
run_kubernetes_deployment_test Test_Group

exit $errors
