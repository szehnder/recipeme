FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o recipeme .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/recipeme /usr/local/bin/recipeme
ENTRYPOINT ["recipeme"]
