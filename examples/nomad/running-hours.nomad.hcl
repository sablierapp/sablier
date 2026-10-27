# Kept running on weekdays from 09:00 to 18:00, in Sablier's local time.
#   make run JOB=running-hours.nomad.hcl
job "office-app" {
  group "web" {
    count = 0

    # region sablier
    meta = {
      "sablier.enable"        = "true"
      "sablier.group"         = "office-app"
      "sablier.running-hours" = "09:00-18:00"
      "sablier.running-days"  = "Mon,Tue,Wed,Thu,Fri"
    }
    # endregion

    network {
      port "http" {}
    }

    service {
      name     = "office-app"
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
