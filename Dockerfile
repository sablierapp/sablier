FROM golang:1.24 AS build

RUN mkdir -p /etc/sablier/themes
WORKDIR /src
RUN go env -w GOMODCACHE=/root/.cache/go-build

# See https://docs.docker.com/build/guide/mounts/#add-bind-mounts for cached builds
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download

COPY . /src
ARG BUILDTIME
ARG VERSION
ARG REVISION
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    make BUILDTIME=${BUILDTIME} VERSION=${VERSION} GIT_REVISION=${REVISION} ${TARGETOS}/${TARGETARCH}

FROM scratch

EXPOSE 10000
COPY --from=build /etc/sablier/themes /etc/sablier/themes
COPY --from=build /src/sablier* /bin/sablier
COPY docker/sablier.yaml /etc/sablier/sablier.yaml

ENTRYPOINT [ "sablier" ]
CMD [ "--configFile=/etc/sablier/sablier.yaml", "start" ]