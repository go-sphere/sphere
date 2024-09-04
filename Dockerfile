FROM golang:1.23 AS builder
WORKDIR /app
COPY . .
RUN go mod download && make buildLinuxX86

FROM scratch
COPY --from=builder /app/build/linux_x86/go-base-api /go-base-api
VOLUME /config
EXPOSE 8800 8899
CMD ["/go-base-api", "start", "--config", "/config/config.json"]