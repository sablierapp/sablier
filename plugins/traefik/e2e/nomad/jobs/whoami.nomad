job "whoami" {
  datacenters = ["dc1"]
  type        = "service"

  group "whoami" {

    network {
      port "web" {
        static = 8082
        to     = 80
      }
    }

    scaling {
      min = 0
      max = 2
    }

    task "whoami" {
      driver = "docker"

      config {
        image = "traefik/whoami:v1.10"
        ports = ["web"]
      }

      service {
        provider = "nomad"
        name     = "whoami"
        port     = "web"

        check {
          name     = "probe"
          type     = "http"
          path     = "/"
          interval = "10s"
          timeout  = "1s"
        }

        tags = [
          "traefik.enable=true",
          "traefik.http.routers.whoami.rule=PathPrefix(`/dynamic/whoami`)",
          "traefik.http.routers.whoami.entrypoints=http",
          "traefik.http.routers.whoami.middlewares=whoami@nomad",

          "traefik.http.middlewares.whoami.plugin.sablier.names=whoami@default/whoami",
          "traefik.http.middlewares.whoami.plugin.sablier.sablierUrl=http://dev-sandbox:10000",
          "traefik.http.middlewares.whoami.plugin.sablier.sessionDuration=10m",
          "traefik.http.middlewares.whoami.plugin.sablier.dynamic.DisplayName=Whoami",
          "traefik.http.middlewares.whoami.plugin.sablier.dynamic.theme=ghost",
        ]
      }

      resources {
        memory = 200
      }
    }
  }
}
