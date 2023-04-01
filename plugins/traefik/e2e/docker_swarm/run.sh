#!/bin/bash

DOCKER_STACK_FILE=docker-stack.yml
DOCKER_STACK_NAME=DOCKER_SWARM_E2E

errors=0

echo "Using Docker version:"
docker version

prepare_docker_swarm() {
  docker swarm init
}

prepare_docker_stack() {
  docker stack deploy --compose-file $DOCKER_STACK_FILE ${DOCKER_STACK_NAME}
  docker run --rm -it -v /var/run/docker.sock:/var/run/docker.sock sudobmitch/docker-stack-wait -t 60 ${DOCKER_STACK_NAME}
}

destroy_docker_stack() {
  docker stack rm ${DOCKER_STACK_NAME}
  # Sometimes, the network is not well cleaned up, see https://github.com/moby/moby/issues/30942#issuecomment-540699206
  until [ -z "$(docker stack ps ${DOCKER_STACK_NAME} -q)" ]; do sleep 1; done
}

destroy_docker_swarm() {
  docker swarm leave -f || true
}

run_docker_swarm_test() {
  echo "Running Docker Swarm Test: $1"
  prepare_docker_stack
  sleep 10
  go clean -testcache
  if ! go test -count=1 -tags e2e -timeout 30s -run ^${1}$ github.com/acouvreur/sablier/e2e; then
    errors=1
    docker service logs ${DOCKER_STACK_NAME}_sablier
    docker service logs ${DOCKER_STACK_NAME}_traefik
  fi
  destroy_docker_stack
}

trap destroy_docker_swarm EXIT

prepare_docker_swarm
run_docker_swarm_test Test_Dynamic
run_docker_swarm_test Test_Blocking
run_docker_swarm_test Test_Multiple
run_docker_swarm_test Test_Healthy

exit $errors