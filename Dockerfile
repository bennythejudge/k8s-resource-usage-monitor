FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o k8s-efficiency-auditor .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/k8s-efficiency-auditor /
ENTRYPOINT ["/k8s-efficiency-auditor"]