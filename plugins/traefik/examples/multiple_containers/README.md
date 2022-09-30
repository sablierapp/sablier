# Docker swarm

## Run the demo

1. `git clone git@github.com:acouvreur/sablier.git`
2. `cd sablier/plugins/traefik/examples/multiple_containers`
3. `docker swarm init`
4. `export TRAEFIK_PILOT_TOKEN=...`
5.  `docker stack deploy -c docker-stack.yml DOCKER_SWARM`
6.  Load `http://localhost/nginx`
7.  Load `http://localhost/whoami`
8.  After 1 minute whoami is scaled to 0/0
9.  After 5 minutes nginx is scaled to 0/0
10. `docker stack rm DOCKER_SWARM`

## Limitations

### Define a middleware per service/container

Due to Traefik plugin, the interface is to provide a config and a `ServeHTTP` request.

This function has no access to the Traefik configuration, thus no way to determine the container/service associated to the request.

See https://github.com/acouvreur/sablier/issues/8#issuecomment-931940533.