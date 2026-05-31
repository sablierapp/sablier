job "sablier" {
  datacenters = ["dc1"]
  type        = "service"

  group "sablier" {
    count = 1

    network {
      mode = "bridge"
      port "http" {
        static = 10000
        to     = 10000
      }
    }

    service {
      name = "sablier"
      port = "http"
      
      check {
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "sablier" {
      driver = "docker"

      config {
        image = "${SABLIER_IMAGE}"
        ports = ["http"]
        
        # Mount Nomad API socket if needed (for local dev)
        # In production, use NOMAD_ADDR environment variable
        volumes = [
          "/opt/nomad/data:/opt/nomad/data:ro"
        ]
      }

      env {
        NOMAD_ADDR = "http://${attr.unique.network.ip-address}:4646"
        PROVIDER_NAME = "nomad"
        PROVIDER_NOMAD_ADDRESS = "http://${attr.unique.network.ip-address}:4646"
        PROVIDER_NOMAD_NAMESPACE = "default"
        SERVER_PORT = "10000"
        SESSIONS_DEFAULT_DURATION = "1m"
        SESSIONS_EXPIRATION_INTERVAL = "10s"
        LOGGING_LEVEL = "debug"
        STRATEGY_DYNAMIC_DEFAULT_THEME = "hacker-terminal"
        STRATEGY_DYNAMIC_SHOW_DETAILS_BY_DEFAULT = "true"
        STRATEGY_BLOCKING_DEFAULT_TIMEOUT = "1m"
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}
