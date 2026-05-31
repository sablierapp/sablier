job "nginx" {
  datacenters = ["dc1"]
  type        = "service"

  group "nginx" {
    count = 1

    network {
      mode = "bridge"
      port "http" {
        static = 8080
        to     = 80
      }
    }

    service {
      name = "nginx"
      port = "http"
      
      check {
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:alpine"
        ports = ["http"]
        
        volumes = [
          "local/nginx.conf:/etc/nginx/nginx.conf:ro"
        ]
      }

      template {
        data = <<EOF
events {
    worker_connections 1024;
}

http {
    # Upstream for Sablier
    upstream sablier {
        server {{ env "NOMAD_IP_sablier_http" }}:{{ env "NOMAD_PORT_sablier_http" }};
    }

    # Upstream for the whoami service
    upstream whoami {
        server {{ range service "whoami" }}{{ .Address }}:{{ .Port }}{{ end }};
    }

    server {
        listen 80;
        server_name localhost;

        # Dynamic strategy: Show loading page while task group starts
        location /whoami {
            proxy_pass http://sablier/api/strategies/dynamic?session_duration=1m&names=whoami/whoami&show_details=true;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            
            # Required for dynamic strategy
            proxy_set_header X-Sablier-Upstream http://whoami;
        }

        # Blocking strategy: Wait for task group to start
        location /blocking {
            proxy_pass http://sablier/api/strategies/blocking?session_duration=1m&names=whoami/whoami&timeout=60s;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            
            # Required for blocking strategy
            proxy_set_header X-Sablier-Upstream http://whoami;
        }

        # Health check endpoint
        location /health {
            proxy_pass http://sablier/health;
        }
    }
}
EOF
        destination = "local/nginx.conf"
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
