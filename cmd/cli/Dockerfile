FROM golang:1.23 AS builder
WORKDIR /app
COPY . .
ENV GOPROXY=https://goproxy.io,direct
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main cmd/cli/main.go

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/main /main
ENV GIN_MODE=release
ENTRYPOINT ["/main"]
CMD ["start", "--config", "/config/config.json"]