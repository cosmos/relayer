cd proj
apk add --no-cache musl-dev
CARGO_TARGET_DIR=target_docker cargo build -r -p go-export
