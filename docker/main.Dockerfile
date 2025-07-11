# 构建 Go 主程序 alisten
FROM golang:1.24 AS builder
WORKDIR /app
COPY . .
RUN go build -o alisten

# 生产环境镜像
FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/alisten /app/alisten
EXPOSE 8080
CMD ["/app/alisten"]
