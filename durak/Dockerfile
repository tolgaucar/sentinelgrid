# DURAK — ÖTÜKEN-MATRİS Veri Paketi Önceliklendirme (QoS)
# Çok aşamalı Docker derlemesi
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# ortam bağımlılığı
COPY ortam/ /build/ortam/

COPY durak/go.mod durak/go.sum* ./
RUN sed -i "s|=> ../ortam|=> /build/ortam|" go.mod && go mod download

COPY durak/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app ./cmd/durak/main.go

# ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app /usr/local/bin/durak

ENV OTUKEN_ENV=prod
EXPOSE 9109

ENTRYPOINT ["durak"]
