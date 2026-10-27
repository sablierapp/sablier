# Kept at one allocation when idle, and scaled to two for a session.
#   make run JOB=scale-mode.nomad.hcl
job "warm-api" {
  group "web" {
    count = 1

    # region sablier
    meta = {
      "sablier.enable"          = "true"
      "sablier.group"           = "warm-api"
      "sablier.idle.replicas"   = "1"
      "sablier.active.replicas" = "2"
    }
    # endregion

    network {
      port "http" {}
    }

    service {
      name     = "warm-api"
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
      # The busybox web server of the Nomad image. Use your own driver and image.
      driver = "raw_exec"

      config {
        command = "/bin/busybox"
        args    = ["httpd", "-f", "-p", "${NOMAD_PORT_http}", "-h", "${NOMAD_TASK_DIR}"]
      }

      template {
        destination = "local/index.html"
        data        = "Hello from {{ env \"NOMAD_JOB_NAME\" }}/{{ env \"NOMAD_GROUP_NAME\" }}.\n"
      }

      resources {
        cpu    = 50
        memory = 32
      }
    }
  }
}
