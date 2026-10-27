FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
COPY ../ortam /ortam
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 go build -o izlek ./cmd/izlek

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/izlek /usr/local/bin/izlek
EXPOSE 9126
CMD ["izlek"]
