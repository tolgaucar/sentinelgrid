FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY ortam/ ortam/
COPY duyum/ duyum/
WORKDIR /build/duyum
RUN go build -o /duyum ./cmd/duyum

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /duyum /usr/local/bin/duyum
EXPOSE 9120 9121
CMD ["duyum"]
