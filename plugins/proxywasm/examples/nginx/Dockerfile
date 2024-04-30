FROM ubuntu:22.04

RUN apt update && apt install libatomic1

ADD https://github.com/Kong/ngx_wasm_module/releases/download/nightly/wasmx-nightly-20240422-v8-x86_64-ubuntu22.04.tar.gz wasmx.tar.gz

RUN mkdir /etc/nginx
RUN tar -xvf wasmx.tar.gz
RUN mv /wasmx-nightly-20240422-v8-x86_64-ubuntu22.04/* /etc/nginx/

WORKDIR /etc/nginx

CMD [ "./nginx", "-g", "daemon off;" ]