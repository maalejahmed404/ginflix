# build stage
FROM golang:1.24.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o app .

# final image: only the binary and ffmpeg
FROM alpine:3.21

RUN apk add --no-cache ffmpeg

WORKDIR /app
COPY --from=builder /app/app .

EXPOSE 8080

CMD ["./app"]
