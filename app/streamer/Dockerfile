FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for Go module fetching
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o server .

# Final minimal image
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]