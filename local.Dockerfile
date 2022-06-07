FROM golang:1.18-alpine as BUILD

WORKDIR /relayer

# Copy the files from host
COPY . .

ENV PACKAGES make git install build-base

# Update and install needed deps prioir to installing the binary.
RUN apk update && apk add --no-cache $PACKAGES

FROM alpine:latest

ENV RELAYER /relayer

RUN addgroup rlyuser && adduser -S -G rlyuser rlyuser -h "$RELAYER"

USER rlyuser

# Define working directory
WORKDIR $RELAYER

# Copy binary from BUILD
COPY --from=BUILD /go/bin/rly /usr/bin/rly

ENTRYPOINT ["/usr/bin/rly"]
