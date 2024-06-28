#!/bin/bash

DOCKER_COMPOSE_FILE=docker-compose.yml
DOCKER_COMPOSE_PROJECT_NAME=docker_classic_e2e

errors=0

echo "Using Docker version:"
docker version

prepare_docker_classic() {
  docker compose -f $DOCKER_COMPOSE_FILE -p $DOCKER_COMPOSE_PROJECT_NAME up -d
  docker compose -f $DOCKER_COMPOSE_FILE -p $DOCKER_COMPOSE_PROJECT_NAME stop whoami nginx
}

destroy_docker_classic() {
  docker compose -f $DOCKER_COMPOSE_FILE -p $DOCKER_COMPOSE_PROJECT_NAME down --remove-orphans || true
}

run_docker_classic_test() {
  echo "Running Docker Classic Test: $1"
  TIMEOUT=${2:-30s}
  echo "TimeOut set to ${TIMEOUT}"
  prepare_docker_classic
  sleep 2
  go clean -testcache
  if ! go test -count=1 -tags e2e -timeout ${TIMEOUT} -run ^${1}$ github.com/acouvreur/sablier/e2e; then
    errors=1
    docker compose -f ${DOCKER_COMPOSE_FILE} -p ${DOCKER_COMPOSE_PROJECT_NAME} logs sablier traefik
  fi
  destroy_docker_classic
}

trap destroy_docker_classic EXIT

run_docker_classic_test Test_Dynamic
run_docker_classic_test Test_Blocking
run_docker_classic_test Test_Multiple
run_docker_classic_test Test_Healthy
run_docker_classic_test Test_Blocking_WebSocket 3m

exit $errors
