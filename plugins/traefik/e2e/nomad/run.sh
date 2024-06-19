#!/bin/bash 

set +m

# ensure nomad is installed
NOMAD_BIN=$(which nomad)
errors=0

echo "Using Docker version:"
docker version

prepare_nomad() {
  if [ ! -f nomad.pid ]; then
    # ensure nomad is running as root (https://github.com/hashicorp/nomad/issues/13669)
    sudo $NOMAD_BIN agent -bind "0.0.0.0" -config=nomad.conf -dev &>nomad.log &
    echo $! > nomad.pid
    sleep 15 # TODO: loop until nomad is online
    set +m

  fi
}

destroy_nomad() {
  sudo kill -9 $(cat nomad.pid)
  rm nomad.pid nomad.log
}

prepare_traefik() {
  nomad run jobs/traefik.nomad
}

prepare_deployment() {
  nomad run jobs/nginx.nomad
  nomad run jobs/whoami.nomad
  nomad run jobs/sablier.nomad
}

destroy_deployment() {
  nomad job stop whoami
  nomad job stop sablier
  nomad job stop nginx
}

get_logs() {
  ALLOC=$(nomad job allocs -no-color -json $1 | jq -r '.[] | select(.ClientStatus=="running") | .ID' | shuf -n 1)
  nomad logs $ALLOC | tail -15
}

run_nomad_deployment_test() {
  echo "---- Running Nomad Test: $1 ----"
  prepare_deployment
  sleep 10
  go clean -testcache
  if ! go test -count=1 -tags e2e -timeout 30s -run ^${1}$ github.com/acouvreur/sablier/e2e; then
    errors=1
    get_logs sablier
    get_logs traefik
  fi

  destroy_deployment
}

trap destroy_nomad EXIT

prepare_nomad
prepare_traefik
run_nomad_deployment_test Test_Dynamic
run_nomad_deployment_test Test_Blocking
run_nomad_deployment_test Test_Multiple
run_nomad_deployment_test Test_Healthy

exit $errors