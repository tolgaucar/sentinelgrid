FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
COPY ../ortam /ortam
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 go build -o koz ./cmd/koz

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/koz /usr/local/bin/koz
EXPOSE 9114
CMD ["koz"]
