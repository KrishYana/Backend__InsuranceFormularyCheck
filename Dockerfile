FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY . .
RUN go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /api ./cmd/api
RUN CGO_ENABLED=0 go build -o /ingest ./cmd/ingest
RUN CGO_ENABLED=0 go build -o /articles ./cmd/articles

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /api /api
COPY --from=builder /ingest /ingest
COPY --from=builder /articles /articles
EXPOSE 8080
CMD ["/api"]
