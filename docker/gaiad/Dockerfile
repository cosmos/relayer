FROM golang:alpine AS build-env
ARG VERSION

ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev

RUN apk add --no-cache $PACKAGES

WORKDIR /go/src/github.com/cosmos

RUN git clone https://github.com/cosmos/gaia.git

WORKDIR /go/src/github.com/cosmos/gaia

RUN git checkout ${VERSION} && make build-linux

FROM alpine:edge

RUN apk add --no-cache ca-certificates
WORKDIR /root

COPY --from=build-env /go/src/github.com/cosmos/gaia/build/gaiad /usr/bin/gaiad

#USER gaia

WORKDIR /gaia

COPY ./_test/setup/gaia-setup.sh .

COPY ./_test/setup/valkeys ./setup/valkeys

USER root

RUN chmod -R 777 ./setup

#USER gaia

EXPOSE 26657

ENTRYPOINT [ "./gaia-setup.sh" ]
# NOTE: to run this image, docker run -d -p 26657:26657 ./gaia-setup.sh {{chain_id}} {{genesis_account}} {{seeds}} {{priv_validator_key_path}}