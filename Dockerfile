# Build gribble/cmd/...
FROM golang:1-alpine AS build
RUN apk add \
    ca-certificates \
    gcc \
    git \
    musl-dev

WORKDIR /build/workspace
COPY . .

ENV GOPATH=/go
ENV GOBIN=/build/workspace/bin
ENV GOCACHE=/go/_gocache

RUN go mod verify
RUN go install -v ./cmd/...

# Final image
FROM alpine:3.9

VOLUME /var/lib/gribble

RUN apk add --no-cache runit

RUN addgroup -S _gribble
RUN adduser -H -S -s/bin/false -h /var/lib/gribble -G _gribble _gribble
RUN install -o _gribble -g _gribble -m700 -d /var/lib/gribble
COPY --from=build /build/workspace/bin /usr/local/bin

WORKDIR /var/lib/gribble
EXPOSE 4077

ENTRYPOINT ["chpst", "-u", "_gribble", "/usr/local/bin/gribblesv", "-L=0.0.0.0:4077"]
