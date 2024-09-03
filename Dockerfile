FROM --platform=$BUILDPLATFORM golang:1.22-alpine3.19 AS build-env

RUN apk add --update --no-cache curl make git libc-dev bash gcc linux-headers eudev-dev

ARG TARGETARCH
ARG BUILDARCH

WORKDIR /app

RUN if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
    wget -c https://musl.cc/aarch64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
    wget -c https://musl.cc/x86_64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
    fi

ADD . .

RUN if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
    export CC=aarch64-linux-musl-gcc CXX=aarch64-linux-musl-g++;\
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
    export CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++; \
    fi; \
    GOOS=linux GOARCH=$TARGETARCH CGO_ENABLED=1 LDFLAGS='-linkmode external -extldflags "-static"' make build

RUN if [ -d "/go/bin/linux_${TARGETARCH}" ]; then mv /go/bin/linux_${TARGETARCH}/* /go/bin/; fi

# Build final image from scratch
FROM alpine:3.19

# Install chain binaries
COPY --from=build-env /app/build/rly /bin/rly

WORKDIR /home

USER root