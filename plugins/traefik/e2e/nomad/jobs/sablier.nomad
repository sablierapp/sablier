job "sablier" {
  datacenters = ["dc1"]
  type        = "service"

  group "sablier" {

    network {
      port "web" {
        to = 10000
      }
    }

    task "sablier" {
      driver = "docker"

      config {
        image = "acouvreur/sablier:local"
        ports = ["web"]
        args = [
          "start",
          "--provider.name=nomad",
          "--server.base-path=/sablier",
        ]

      }

      env {
        NOMAD_ADDR = "http://${attr.unique.hostname}:4646"
      }

      service {
        provider = "nomad"
        name     = "sablier"
        port     = "web"

        check {
          name     = "probe"
          type     = "http"
          path     = "/sablier/health"
          interval = "10s"
          timeout  = "1s"
        }


        tags = [

          "traefik.enable=true",
          "traefik.http.routers.sablier.rule=PathPrefix(`/sablier`)",

          # Dynamic Middleware
          "traefik.http.middlewares.dynamic.plugin.sablier.names=default_whoami_whoami_1",
          "traefik.http.middlewares.dynamic.plugin.sablier.sablierUrl=http://${attr.unique.hostname}:80/sablier",
          "traefik.http.middlewares.dynamic.plugin.sablier.sessionDuration=1m",
          "traefik.http.middlewares.dynamic.plugin.sablier.dynamic.DisplayName=Dynamic Whoami",
          "traefik.http.middlewares.dynamic.plugin.sablier.dynamic.theme=hacker-terminal",
          
          # Blocking Middleware
          "traefik.http.middlewares.blocking.plugin.sablier.names=default_whoami_whoami_1",
          "traefik.http.middlewares.blocking.plugin.sablier.sablierUrl=http://${attr.unique.hostname}:80/sablier",
          "traefik.http.middlewares.blocking.plugin.sablier.sessionDuration=1m",
          "traefik.http.middlewares.blocking.plugin.sablier.blocking.timeout=30s",

          # Multiple Dynamic Middleware
          "traefik.http.middlewares.multiple.plugin.sablier.names=default_whoami_whoami_1,default_nginx_nginx_1",
          "traefik.http.middlewares.multiple.plugin.sablier.sablierUrl=http://${attr.unique.hostname}:80/sablier",
          "traefik.http.middlewares.multiple.plugin.sablier.sessionDuration=1m",
          "traefik.http.middlewares.multiple.plugin.sablier.dynamic.displayName=Multiple Whoami",

          # Healthy Middleware
          "traefik.http.middlewares.healthy.plugin.sablier.names=default_nginx_nginx_1",
          "traefik.http.middlewares.healthy.plugin.sablier.sablierUrl=http://${attr.unique.hostname}:80/sablier",
          "traefik.http.middlewares.healthy.plugin.sablier.sessionDuration=1m",
          "traefik.http.middlewares.healthy.plugin.sablier.dynamic.DisplayName=Healthy Nginx",
          "traefik.http.middlewares.healthy.plugin.sablier.dynamic.theme=hacker-terminal",

        ]
      }

      resources {
        memory = 200
      }
    }
  }
}
