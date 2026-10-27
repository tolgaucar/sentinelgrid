FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /build

# Ortam bağımlılığı
COPY ortam/ ./ortam/

# Modül dosyaları
COPY tani/go.mod ./tani/
WORKDIR /build/tani

# Replace path düzelt
RUN sed -i 's|../ortam|/build/ortam|' go.mod

RUN go mod download
COPY tani/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /tani ./cmd/tani

# Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /tani /usr/local/bin/tani
EXPOSE 9113
ENTRYPOINT ["tani"]
