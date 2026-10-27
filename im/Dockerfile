# İM — ÖTÜKEN-MATRİS Dijital Kimlik & İmza
# Çok aşamalı Docker derlemesi
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# ortam bağımlılığı
COPY ortam/ /build/ortam/

COPY im/go.mod im/go.sum* ./
RUN sed -i "s|=> ../ortam|=> /build/ortam|" go.mod && go mod download

COPY im/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app ./cmd/im/main.go

# ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app /usr/local/bin/im

ENV OTUKEN_ENV=prod
EXPOSE 9105

ENTRYPOINT ["im"]
