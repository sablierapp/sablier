# One task group in two groups: a session for team-a or team-b starts it.
#   make run JOB=group.nomad.hcl
job "team-api" {
  group "web" {
    count = 0

    # region sablier
    meta = {
      "sablier.enable" = "true"
      "sablier.group"  = "team-a,team-b"
    }
    # endregion

    network {
      port "http" {}
    }

    service {
      name     = "team-api"
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
