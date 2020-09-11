
FROM golang:1.13 AS builder
WORKDIR /build
COPY . /build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo -ldflags '-w -extldflags "-static"'

FROM alpine
COPY --from=builder /build/alignfootbot /bot/alignfootbot
ENTRYPOINT ["/bot/alignfootbot"]

