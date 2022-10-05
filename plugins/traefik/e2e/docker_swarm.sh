#!/bin/bash

TRAEFIK_VERSION=2.9.4
DOCKER_STACK_FILE=docker-stack.yml
DOCKER_STACK_NAME=DOCKER_SWARM_E2E

echo "Using Traefik version ${TRAEFIK_VERSION}"
echo "Using Docker version:"
docker version

prepare_docker_swarm() {
  docker swarm init
}

prepare_docker_stack() {
  docker stack deploy --compose-file $DOCKER_STACK_FILE ${DOCKER_STACK_NAME}
}

destroy_docker_stack() {
  docker stack rm ${DOCKER_STACK_NAME}
}

destroy_docker_swarm() {
  docker swarm leave -f
}

run_docker_swarm_test() {
  echo "Running Docker Swarm Test: $1"
  prepare_docker_stack
  sleep 10
  go clean -testcache
  go test -count=1 -timeout 30s -run ^${1}$ github.com/acouvreur/sablier/e2e || docker service logs ${DOCKER_STACK_NAME}_sablier && docker service logs ${DOCKER_STACK_NAME}_traefik
  destroy_docker_stack
}

prepare_docker_swarm
run_docker_swarm_test Test_Dynamic
run_docker_swarm_test Test_Blocking
run_docker_swarm_test Test_Multiple
run_docker_swarm_test Test_Healthy
destroy_docker_swarm