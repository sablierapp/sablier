FROM golang:1.18-alpine AS build

ENV CGO_ENABLED=0
ENV PORT 10000

COPY . /go/src/sablier
WORKDIR /go/src/sablier

ARG TARGETOS
ARG TARGETARCH
RUN GIN_MODE=release GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -buildvcs=false -o /go/bin/sablier

FROM alpine
EXPOSE 10000
COPY --from=build /go/bin/sablier /go/bin/sablier

ENTRYPOINT [ "/go/bin/sablier" ]
CMD [ "--swarmMode=true" ]