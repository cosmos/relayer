FROM golang:alpine as BUILD

WORKDIR /relayer

# Copy the files from host
COPY . .

# Update and install needed deps prioir to installing the binary.
RUN apk update && \
    apk --no-cache add make=4.2.1-r2 git=2.24.3-r0 && \
    make install

FROM alpine:edge

ENV RELAYER /relayer

RUN addgroup rlyuser && \
    adduser -S -G rlyuser rlyuser -h "$RELAYER"

USER rlyuser

# Define working directory
WORKDIR $RELAYER

# Copy binary from BUILD
COPY --from=BUILD /go/bin/rly /usr/bin/rly

ENTRYPOINT ["/usr/bin/rly"]

# Make config available ofr mutaitons
VOLUME [ $RELAYER ]
