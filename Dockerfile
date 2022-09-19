FROM --platform=$BUILDPLATFORM golang:1.18-alpine as BUILD

WORKDIR /relayer

# Update and install needed deps prioir to installing the binary.
RUN apk update && \
  apk --no-cache add make git build-base

# Copy go.mod and go.sum first and download for caching go modules
COPY go.mod go.sum ./

RUN go mod download

# Copy the files from host
COPY . .

ARG TARGETARCH TARGETOS
ENV GOOS=${TARGETOS} GOARCH=${TARGETARCH}
RUN make install

FROM alpine:latest

ENV RELAYER /relayer

RUN addgroup rlyuser && adduser -S -G rlyuser rlyuser -h "$RELAYER"

USER rlyuser

# Define working directory
WORKDIR $RELAYER

# Copy binary from BUILD
COPY --from=BUILD /go/bin/rly /usr/bin/rly

ENTRYPOINT ["/usr/bin/rly"]
