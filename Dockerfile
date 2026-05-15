FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o odk .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/odk .
EXPOSE 40444
VOLUME ["/data"]
ENV ODK_DB=/data/odk.db
ENV ODK_READER_DB=/data/odk-reader.db
ENV ODK_LOG=/data/odk-agent.log
ENTRYPOINT ["/app/odk"]
CMD ["agent", "--api", ":40444", "--db", "/data/odk.db", "--reader-db", "/data/odk-reader.db", "--log", "/data/odk-agent.log"]
