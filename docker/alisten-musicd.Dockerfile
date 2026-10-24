FROM golang:1.27-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o alisten-musicd ./cmd/alisten-musicd

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1001 -S alisten && \
    adduser -u 1001 -S alisten -G alisten
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder --chown=alisten:alisten /app/alisten-musicd .
USER alisten
EXPOSE 9090 8081
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8081/healthz || exit 1
CMD ["./alisten-musicd", "-listen", ":9090", "-http", ":8081"]
