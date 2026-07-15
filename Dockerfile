FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the bot
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/ai-tarologist .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary and miniapp
COPY --from=builder /app/ai-tarologist .
COPY --from=builder /app/miniapp ./miniapp

# Expose port for Mini App
EXPOSE 8080

# Run the bot
CMD ["./ai-tarologist"]
