docker compose build
docker compose up

watch -n 1 docker compose ps -a

export SESSIONS_DEFAULT_DURATION=10s
export SESSIONS_EXPIRATION_INTERVAL=1s

make run

curl http://localhost:8080/web/
