# Nomad disables the raw_exec driver by default because it does not isolate tasks.
# Enable it only for local tests like this example.
plugin "raw_exec" {
  config {
    enabled = true
  }
}
