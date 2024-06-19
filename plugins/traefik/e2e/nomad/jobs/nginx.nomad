job "nginx" {
  datacenters = ["dc1"]
  type        = "service"

  group "nginx" {

    network {
      port "web" {
        static = 8081
        to = 80
      }
    }

    scaling {
      min = 0
      max = 3
    }

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.25.2"
        ports = ["web"]
      }

      service {
        provider = "nomad"
        name     = "nginx"
        port     = "web"

        check {
          name     = "probe"
          type     = "http"
          path     = "/"
          interval = "10s"
          timeout  = "1s"
        }
      }

      resources {
        memory = 200
      }
    }
  }
}
