# YUZEY — OTUKEN-MATRIS
# Cok asamali Docker derlemesi
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# ortam bagimliligi
COPY ortam/ /build/ortam/

COPY yuzey/go.mod yuzey/go.sum* ./
RUN sed -i "s|=> ../ortam|=> /build/ortam|" go.mod && go mod download

COPY yuzey/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app ./cmd/yuzey/main.go

# ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app /usr/local/bin/yuzey

ENV OTUKEN_ENV=prod
EXPOSE 9103

ENTRYPOINT ["yuzey"]
