FROM golang:1.18-alpine
ADD . /app
WORKDIR /app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/otel-span-bigquery

FROM alpine:latest
WORKDIR /app
COPY --from=0 /app/otel-span-bigquery /app/otel-span-bigquery
CMD ["/app/otel-span-bigquery"]