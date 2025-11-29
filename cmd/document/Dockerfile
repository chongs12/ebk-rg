FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app ./cmd/document

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/app /app/app
RUN mkdir -p /app/uploads
VOLUME ["/app/uploads"]
ENV GIN_MODE=release
ENV EKB_SERVICE_NAME=document
CMD ["/app/app"]
