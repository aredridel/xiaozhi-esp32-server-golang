FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# Copy binary from build stage
COPY --from=builder /app/main .
COPY --from=builder /app/config ./config

# Set timezone
ENV TZ=Asia/Shanghai

EXPOSE 8080

CMD ["./main"]