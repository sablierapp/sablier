job "whoami" {
  datacenters = ["dc1"]
  type        = "service"

  group "whoami" {
    count = 0  # Start at 0, Sablier will scale it up on demand

    # Required: Sablier meta tags
    meta {
      "sablier.enable" = "true"
      "sablier.group"  = "whoami"
    }

    network {
      mode = "bridge"
      port "http" {
        to = 8080
      }
    }

    service {
      name = "whoami"
      port = "http"
      
      check {
        type     = "http"
        path     = "/"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "whoami" {
      driver = "docker"

      config {
        image = "acouvreur/whoami:v1.10.2"
        ports = ["http"]
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
