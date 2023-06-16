FROM --platform=$BUILDPLATFORM golang:alpine AS build-env

RUN apk add --update --no-cache make git musl-dev gcc binutils-gold cargo

ARG BUILDPLATFORM=arm64
ARG TARGETPLATFORM=arm64
ARG COSMWASM_VERSION=1.2.3

RUN wget https://github.com/CosmWasm/wasmvm/releases/download/v${COSMWASM_VERSION}/libwasmvm_muslc.aarch64.a -O /usr/lib/libwasmvm.aarch64.a && \
    wget https://github.com/CosmWasm/wasmvm/releases/download/v${COSMWASM_VERSION}/libwasmvm_muslc.x86_64.a -O /usr/lib/libwasmvm.x86_64.a

COPY . .

RUN LDFLAGS='-linkmode external -extldflags "-static"' make install

RUN if [ -d "/go/bin/linux_${TARGETPLATFORM}" ]; then mv /go/bin/linux_${TARGETPLATFORM}/* /go/bin/; fi

# Use minimal busybox from infra-toolkit image for final scratch image
FROM --platform=$BUILDPLATFORM ghcr.io/strangelove-ventures/infra-toolkit:v0.0.6 AS busybox-min
RUN addgroup --gid 1000 -S relayer && adduser --uid 100 -S relayer -G relayer

# Use ln and rm from full featured busybox for assembling final image
FROM --platform=$BUILDPLATFORM busybox:musl AS busybox-full

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
COPY --from=build-env /bin/rly /bin

# Install trusted CA certificates
COPY --from=busybox-min /etc/ssl/cert.pem /etc/ssl/cert.pem

# Install relayer user
COPY --from=busybox-min /etc/passwd /etc/passwd
COPY --from=busybox-min --chown=100:1000 /home/relayer /home/relayer

USER relayer

WORKDIR /home/relayer

COPY ./env/godWallet.json ./keys/godwallet.json

CMD ["/bin/rly"]
