FROM golang:1.22 AS build

ADD https://github.com/tinygo-org/tinygo/releases/download/v0.31.2/tinygo_0.31.2_amd64.deb tinygo_amd64.deb
RUN dpkg -i tinygo_amd64.deb

WORKDIR /go/src/sablier/plugins/proxywasm

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . /go/src/sablier/plugins/proxywasm

RUN make

FROM scratch

COPY --from=build /go/src/sablier/plugins/proxywasm/sablierproxywasm.wasm ./plugin.wasm