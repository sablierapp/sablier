FROM golang:1.18 AS build

ENV PORT 10000

WORKDIR /go/src/sablier

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . /go/src/sablier

ARG BUILDTIME
ARG VERSION
ARG REVISION
ARG TARGETOS
ARG TARGETARCH
RUN make BUILDTIME=${BUILDTIME} VERSION=${VERSION} GIT_REVISION=${REVISION} ${TARGETOS}/${TARGETARCH}

FROM alpine

COPY --from=build /go/src/sablier/sablier* /etc/sablier/sablier

EXPOSE 10000

ENTRYPOINT [ "/etc/sablier/sablier" ]
CMD [ "start", "--provider.name=docker"]