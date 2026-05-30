FROM golang:1.22-alpine AS builder

RUN apk add --no-cache build-base

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o agentdisk .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/agentdisk .
COPY config.yaml .

EXPOSE 8080
ENTRYPOINT ["./agentdisk"]
CMD ["--config", "config.yaml"]
