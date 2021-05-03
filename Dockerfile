# Build source code using latest version of golang
FROM golang:alpine AS build
RUN mkdir /opt/aio-bot

COPY *.go go.* /opt/aio-bot/

RUN \
    cd /opt/aio-bot && \
    go mod tidy && \
    go build -o ./aio-bot .

# Build production image based on UBI8
FROM alpine:latest
RUN \
    apk add tzdata
COPY --from=build /opt/aio-bot/aio-bot /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/aio-bot"]