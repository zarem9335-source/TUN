FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /golden ./cmd/golden

FROM alpine:3.20
RUN apk add --no-cache iproute2 wireguard-tools iptables bash
WORKDIR /opt/golden
COPY --from=builder /golden /usr/local/bin/golden
COPY web ./web
EXPOSE 2096
ENTRYPOINT ["/usr/local/bin/golden", "-port=2096", "-web=/opt/golden/web"]
