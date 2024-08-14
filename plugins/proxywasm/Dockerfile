FROM golang:1.23 AS build

ADD https://github.com/tinygo-org/tinygo/releases/download/v0.32.0/tinygo_0.32.0_amd64.deb tinygo_amd64.deb
RUN dpkg -i tinygo_amd64.deb

WORKDIR /go/src/sablier/plugins/proxywasm

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . /go/src/sablier/plugins/proxywasm

RUN make

FROM scratch

COPY --from=build /go/src/sablier/plugins/proxywasm/sablierproxywasm.wasm ./plugin.wasm