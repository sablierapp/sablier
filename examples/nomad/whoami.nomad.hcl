job "whoami" {
  datacenters = ["dc1"]
  type        = "service"

  group "web" {
    # Sablier scales the task group up on request and back to 0 on expiry.
    count = 0

    # A map, because HCL does not accept quoted names in a meta block.
    meta = {
      "sablier.enable" = "true"
      "sablier.group"  = "demo"
    }

    network {
      port "http" {
        static = 8080
      }
    }

    # Sablier reports the instance as ready when the deployment marks the allocation healthy.
    update {
      health_check     = "checks"
      min_healthy_time = "5s"
    }

    service {
      name     = "whoami"
      port     = "http"
      provider = "nomad"

      check {
        type     = "http"
        path     = "/"
        interval = "2s"
        timeout  = "1s"
      }
    }

    task "server" {
      # The busybox web server is part of the Nomad image.
      driver = "raw_exec"

      config {
        command = "/bin/busybox"
        args    = ["httpd", "-f", "-p", "${NOMAD_PORT_http}", "-h", "${NOMAD_TASK_DIR}/www"]
      }

      template {
        destination = "local/www/index.html"
        data        = <<-EOT
          Hello from the Nomad task group {{ env "NOMAD_JOB_NAME" }}/{{ env "NOMAD_GROUP_NAME" }}.
          Allocation {{ env "NOMAD_ALLOC_ID" }} was started on demand by Sablier.
        EOT
      }

      resources {
        cpu    = 50
        memory = 32
      }
    }
  }
}
