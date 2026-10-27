FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git sed

WORKDIR /build

# ortam bağımlılığı
COPY ortam/ ./ortam/

# Modül bağımlılıkları
COPY tutum/go.mod ./tutum/
RUN cd tutum && sed -i 's|replace otukenmatris/ortam => ../ortam|replace otukenmatris/ortam => /build/ortam|' go.mod
RUN cd tutum && go mod download

# Kaynak kodu
COPY tutum/ ./tutum/
RUN cd tutum && CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/tutum ./cmd/tutum

# Çalıştırma
FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /bin/tutum /bin/tutum

EXPOSE 9111

ENTRYPOINT ["/bin/tutum"]
