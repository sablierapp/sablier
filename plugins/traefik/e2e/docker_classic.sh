#!/bin/bash

TRAEFIK_VERSION=2.9.4
DOCKER_COMPOSE_FILE=docker-compose.yml
DOCKER_COMPOSE_PROJECT_NAME=docker_classic_e2e

echo "Using Traefik version ${TRAEFIK_VERSION}"
echo "Using Docker version:"
docker version

prepare_docker_classic() {
  docker compose -f $DOCKER_COMPOSE_FILE -p $DOCKER_COMPOSE_PROJECT_NAME up -d
  docker compose -f $DOCKER_COMPOSE_FILE -p $DOCKER_COMPOSE_PROJECT_NAME stop whoami nginx
}

destroy_docker_classic() {
  docker compose -f $DOCKER_COMPOSE_FILE -p $DOCKER_COMPOSE_PROJECT_NAME down --remove-orphans
}

run_docker_classic_test() {
  echo "Running Docker Classic Test: $1"
  prepare_docker_classic
  sleep 2
  go clean -testcache
  go test -count=1 -tags e2e -timeout 30s -run ^${1}$ github.com/acouvreur/sablier/e2e || docker compose -f ${DOCKER_COMPOSE_FILE} -p ${DOCKER_COMPOSE_PROJECT_NAME}"" logs sablier traefik
  destroy_docker_classic
}

run_docker_classic_test Test_Dynamic
run_docker_classic_test Test_Blocking
run_docker_classic_test Test_Multiple
run_docker_classic_test Test_Healthy