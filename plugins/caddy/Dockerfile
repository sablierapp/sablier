ARG CADDY_VERSION=2.8.4
FROM caddy:${CADDY_VERSION}-builder AS builder

COPY . .

RUN xcaddy build \
    --with github.com/acouvreur/sablier/plugins/caddy=.

FROM caddy:${CADDY_VERSION}

COPY --from=builder /usr/bin/caddy /usr/bin/caddy