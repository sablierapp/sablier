FROM golang:1.21 AS build

WORKDIR /src

# See https://docs.docker.com/build/guide/mounts/#add-bind-mounts for cached builds
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download

COPY . /src
ARG BUILDTIME
ARG VERSION
ARG REVISION
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod/ \
    make BUILDTIME=${BUILDTIME} VERSION=${VERSION} GIT_REVISION=${REVISION} ${TARGETOS}/${TARGETARCH}

FROM alpine:3.19.1

COPY --from=build /src/sablier* /etc/sablier/sablier
COPY docker/sablier.yaml /etc/sablier/sablier.yaml

EXPOSE 10000

ENTRYPOINT [ "/etc/sablier/sablier" ]
CMD [ "--configFile=/etc/sablier/sablier.yaml", "start" ]