FROM golang:1.26.5 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o client .

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/client .
CMD ["./client"]