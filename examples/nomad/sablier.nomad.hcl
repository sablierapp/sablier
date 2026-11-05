# Sablier as a Nomad job, for a Nomad cluster with the Docker driver and ACLs.
# Store the ACL token in a Nomad variable first:
#   nomad var put nomad/jobs/sablier token=<secret ID>
#   nomad job run sablier.nomad.hcl
job "sablier" {
  group "sablier" {
    network {
      port "http" {
        static = 10000
      }
    }

    service {
      name     = "sablier"
      port     = "http"
      provider = "nomad"

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
        image = "sablierapp/sablier:1.20.0" # x-release-please-version
        args  = ["start", "--provider.name=nomad"]
        ports = ["http"]
      }

      # region env
      env {
        NOMAD_ADDR      = "http://${attr.unique.network.ip-address}:4646"
        NOMAD_NAMESPACE = "default"
        # sablier.running-hours windows use this time zone.
        TZ = "Europe/Paris"
      }
      # endregion

      # region token
      # The task reads its own variable, so the token never appears in the job.
      template {
        data        = "NOMAD_TOKEN={{ with nomadVar \"nomad/jobs/sablier\" }}{{ .token }}{{ end }}"
        destination = "secrets/env"
        env         = true
      }
      # endregion

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
