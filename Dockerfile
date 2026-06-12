# Stage 1: Build the Go binary
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Menerima nama service dari docker-compose
ARG SERVICE
# Build binary dan letakkan di root /app
RUN go build -o main ./${SERVICE}/cmd/...

# Stage 2: Create a minimal production image
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main .
EXPOSE 8080 50051 50052
CMD ["./main"]