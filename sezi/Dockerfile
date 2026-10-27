FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
COPY ../ortam /ortam
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 go build -o sezi ./cmd/sezi

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/sezi /usr/local/bin/sezi
EXPOSE 9116
CMD ["sezi"]
