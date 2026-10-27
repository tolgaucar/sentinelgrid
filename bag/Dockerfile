# BAG — OTUKEN-MATRIS
# Cok asamali Docker derlemesi
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# ortam bagimliligi
COPY ortam/ /build/ortam/

COPY bag/go.mod bag/go.sum* ./
RUN sed -i "s|=> ../ortam|=> /build/ortam|" go.mod && go mod download

COPY bag/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app ./cmd/bag/main.go

# ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app /usr/local/bin/bag

ENV OTUKEN_ENV=prod
EXPOSE 9091

ENTRYPOINT ["bag"]
