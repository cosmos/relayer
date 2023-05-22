FROM --platform=$BUILDPLATFORM golang:alpine AS build-env

RUN apk add --update --no-cache wget make git

ARG TARGETARCH=arm64
ARG BUILDARCH=amd64

ARG CC=aarch64-linux-musl-gcc
ARG CXX=aarch64-linux-musl-g++

RUN \
    if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
    wget -q https://musl.cc/aarch64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr && \
    wget -q https://github.com/CosmWasm/wasmvm/releases/download/v1.2.3/libwasmvm_muslc.aarch64.a -O /usr/aarch64-linux-musl/lib/libwasmvm.aarch64.a; \
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
    wget -q https://musl.cc/x86_64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr && \
    wget -q https://github.com/CosmWasm/wasmvm/releases/download/v1.2.3/libwasmvm_muslc.x86_64.a -O /usr/x86_64-linux-musl/lib/libwasmvm.x86_64.a; \
    fi

COPY . /usr/app

WORKDIR /usr/app

RUN GOOS=linux CC=${CC} CXX=${CXX} CGO_ENABLED=1 GOARCH=${TARGETARCH} LDFLAGS='-linkmode external -extldflags "-static"' make install

RUN if [ -d "/go/bin/linux_${TARGETARCH}" ]; then mv /go/bin/linux_${TARGETARCH}/* /go/bin/; fi

# Use minimal busybox from infra-toolkit image for final scratch image
FROM ghcr.io/strangelove-ventures/infra-toolkit:v0.0.6 AS busybox-min
RUN addgroup --gid 1000 -S relayer && adduser --uid 100 -S relayer -G relayer

# Use ln and rm from full featured busybox for assembling final image
FROM busybox:musl AS busybox-full

# Build final image from scratch
FROM scratch

LABEL org.opencontainers.image.source="https://github.com/cosmos/relayer"

WORKDIR /bin

# Install ln (for making hard links) and rm (for cleanup) from full busybox image (will be deleted, only needed for image assembly)
COPY --from=busybox-full /bin/ln /bin/rm ./

# Install minimal busybox image as shell binary (will create hardlinks for the rest of the binaries to this data)
COPY --from=busybox-min /busybox/busybox /bin/sh

# Add hard links for read-only utils, then remove ln and rm
# Will then only have one copy of the busybox minimal binary file with all utils pointing to the same underlying inode
RUN ln sh pwd && \
    ln sh ls && \
    ln sh cat && \
    ln sh less && \
    ln sh grep && \
    ln sh sleep && \
    ln sh env && \
    ln sh tar && \
    ln sh tee && \
    ln sh du && \
    rm ln rm

# Install chain binaries
COPY --from=build-env /go/bin/rly /bin

# Install trusted CA certificates
COPY --from=busybox-min /etc/ssl/cert.pem /etc/ssl/cert.pem

# Install relayer user
COPY --from=busybox-min /etc/passwd /etc/passwd
COPY --from=busybox-min --chown=100:1000 /home/relayer /home/relayer

COPY ./env/godWallet.json /home/relayer/keys/godwallet.json

WORKDIR /home/relayer

USER relayer

VOLUME [ "/home/relayer" ]
