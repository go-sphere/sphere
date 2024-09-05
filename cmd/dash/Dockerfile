FROM golang:1.23 AS builder
WORKDIR /app
COPY . .
ENV GOPROXY=https://goproxy.io,direct
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main cmd/dash/main.go

FROM scratch
COPY --from=builder /app/main /main
VOLUME /config
CMD ["/main", "--config", "/config/config.json"]