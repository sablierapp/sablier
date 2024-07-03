
# Install Sablier on its own

You can install Sablier with the following flavors:

- Use the Docker image
- Use the binary distribution
- Compile your binary from the sources

## Use the Docker image

- **Docker Hub**: [acouvreur/sablier](https://hub.docker.com/r/acouvreur/sablier)
- **Github Container Registry**: [ghcr.io/acouvreur/sablier](https://github.com/acouvreur/sablier/pkgs/container/sablier)
  
Choose one of the Docker images and run it with one sample configuration file:

- [sablier.yaml](https://raw.githubusercontent.com/acouvreur/sablier/main/sablier.sample.yaml)

```bash
docker run -d -p 10000:10000 \
    -v $PWD/sablier.yml:/etc/sablier/sablier.yml acouvreur/sablier:1.8.0-beta.5
```

## Use the binary distribution

Grab the latest binary from the [releases](https://github.com/acouvreur/sablier/releases) page.

And run it:

```bash
./sablier --help
```

## Compile your binary from the sources

```bash
git clone git@github.com:acouvreur/sablier.git
cd sablier
make
# Output will change depending on your distro
./sablier_draft_linux-amd64
```