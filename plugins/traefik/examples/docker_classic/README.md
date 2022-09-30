# Docker classic

## Run the demo

1. `git clone git@github.com:acouvreur/sablier.git`
2. `cd sablier/plugins/traefik/examples/docker_classic`
3. `export TRAEFIK_PILOT_TOKEN=...`
4. `docker-compose up`
   
   The log: `level=error msg="middleware \"ondemand@docker\" does not exist" entryPointName=http routerName=whoami@file` is expected because the file provider is parsed before the docker containers. However this should appear only once and not cause any issue.
5. `docker stop docker_classic_whoami_1`
6. Load `http://localhost/whoami`
7. Wait 1 minute
8. Container is stopped
  
## Limitations

### Cannot use service labels

Cannot use labels because as soon as the container is stopped, the labels are not treated by Traefik.

The route doesn't exist anymore, so we use dynamic-config.yml file instead.