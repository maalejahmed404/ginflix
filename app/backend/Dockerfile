# Use a minimal Go image
FROM golang:1.24.2-alpine

# Set working directory
WORKDIR /app

# Install ffmpeg
RUN apk add --no-cache ffmpeg

# Copy Go modules first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the app
RUN go build -o app .

# Expose port
EXPOSE 8080

# Run the app
CMD ["./app"]
