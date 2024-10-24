build:
	go generate
	tinygo build -o sablierproxywasm.wasm -scheduler=none -target=wasi ./

docker:
	docker build -t sablierapp/sablier-proxy-wasm:latest .