FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY . .
RUN apk add --no-cache gcc musl-dev
RUN go mod download
RUN CGO_ENABLED=1 go build -o bot

FROM alpine:latest

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app
COPY --from=builder /app/bot .

# Create a directory for the SQLite database
RUN mkdir -p /app/data && chmod 777 /app/data

# Environment variables

ENV LOG_LEVEL="info"

# Volume for persistent database storage
VOLUME ["/app/data"]

CMD ["./bot"]
