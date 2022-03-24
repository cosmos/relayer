FROM golang:1.16-alpine AS build-env
ARG VERSION

# Set up dependencies
ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev

RUN apk add --no-cache $PACKAGES

WORKDIR /go/src/github.com/ovrclk

RUN git clone https://github.com/ovrclk/akash.git

WORKDIR /go/src/github.com/ovrclk/akash

RUN git checkout ${VERSION} && make install

FROM alpine:edge

RUN apk add --no-cache ca-certificates
WORKDIR /root

# Copy over binaries from the build-env
COPY --from=build-env /go/bin/akash /usr/bin/akash

WORKDIR /akash

COPY ./_test/setup/akash-setup.sh .

COPY ./_test/setup/valkeys ./setup/valkeys

USER root

RUN chmod -R 777 ./setup

EXPOSE 26657

ENTRYPOINT [ "./akash-setup.sh" ]
# NOTE: to run this image, docker run -d -p 26657:26657 ./akash-setup.sh {{chain_id}} {{genesis_account}} {{seeds}} {{priv_validator_key_path}}
