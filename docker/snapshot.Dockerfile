FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/snapshot/ cmd/snapshot/
RUN CGO_ENABLED=0 go build -o alisten-snapshot ./cmd/snapshot/

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /build/alisten-snapshot .
EXPOSE 9090
CMD ["./alisten-snapshot"]
