# Settings in the job meta apply to every task group. A task group meta
# overrides them: web is in the shop group, admin in the back-office group.
#   make run JOB=job-meta.nomad.hcl
job "shop" {
  # region job
  meta = {
    "sablier.enable" = "true"
    "sablier.group"  = "shop"
  }
  # endregion

  group "web" {
    count = 0

    network {
      port "http" {}
    }

    service {
      name     = "shop-web"
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

  group "admin" {
    count = 0

    # region group
    meta = {
      "sablier.group" = "back-office"
    }
    # endregion

    network {
      port "http" {}
    }

    service {
      name     = "shop-admin"
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
