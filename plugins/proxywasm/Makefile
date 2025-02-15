build:
	go generate
	env GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o sablierproxywasm.wasm .

docker:
	docker build -t sablierapp/sablier-proxy-wasm:latest .