FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY ortam/ ortam/
COPY sozgec/ sozgec/
WORKDIR /build/sozgec
RUN go build -o /sozgec ./cmd/sozgec

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /sozgec /usr/local/bin/sozgec
EXPOSE 9124 9125
CMD ["sozgec"]
