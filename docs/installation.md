# Install Sablier

You can install Sablier using one of the following methods:

- Use the Docker image
- Use the binary distribution
- Compile from source

## Use the Docker Image

- **Docker Hub**: [sablierapp/sablier](https://hub.docker.com/r/sablierapp/sablier)
- **GitHub Container Registry**: [ghcr.io/sablierapp/sablier](https://github.com/sablierapp/sablier/pkgs/container/sablier)
  
Choose one of the Docker images and run it with a sample configuration file:

- [sablier.yaml](https://raw.githubusercontent.com/sablierapp/sablier/main/sablier.sample.yaml)

<!-- x-release-please-start-version -->
```bash
docker run -d -p 10000:10000 \
    -v $PWD/sablier.yaml:/etc/sablier/sablier.yaml sablierapp/sablier:1.10.1
```
<!-- x-release-please-end -->

## Use the Binary Distribution

Download the latest binary from the [releases](https://github.com/sablierapp/sablier/releases) page and run it:

```bash
./sablier --help
```

## Compile from Source

```bash
git clone git@github.com:sablierapp/sablier.git
cd sablier
make
# Output will vary depending on your platform
./sablier_draft_linux-amd64
```
