# Docker swarm

## Run the demo

1. `git clone git@github.com:acouvreur/sablier.git`
2. `cd sablier/plugins/traefik/examples/docker_swarm`
3. `docker swarm init`
4. `export TRAEFIK_PILOT_TOKEN=...`
5.  `docker stack deploy -c docker-stack.yml DOCKER_SWARM`
6.  Load `http://localhost/nginx`
7.  Wait 1 minute
8.  Service is scaled to 0/0