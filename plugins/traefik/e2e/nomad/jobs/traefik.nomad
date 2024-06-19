job "traefik" {
  datacenters = ["dc1"]
  type        = "service"

  group "traefik" {

    task "traefik" {
      driver = "docker"

      template {
          data        = file(abspath("dynamic-config.yml"))
          destination = "local/dynamic-config.yml"
      }
      config {
        image        = "traefik:v3.0"
        network_mode = "host"

        args = [
          "--api.dashboard=true",
          "--api.insecure=true",
          "--experimental.localPlugins.sablier.moduleName=github.com/acouvreur/sablier",
          "--entryPoints.http.address=:80",
          "--providers.file.filename=/etc/traefik/dynamic-config.yml",
          "--providers.nomad.refreshInterval=30s",
          "--providers.nomad.prefix=traefik",
          "--providers.nomad.endpoint.address=http://${attr.unique.hostname}:4646",
          "--providers.nomad.exposedByDefault=true",
        ]

        volumes = [
          "${abspath("../../../..")}:/plugins-local/src/github.com/acouvreur/sablier",
          "local/dynamic-config.yml:/etc/traefik/dynamic-config.yml",
        ]

      }

      resources {
        memory = 200
      }
    }
  }
}
