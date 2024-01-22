FROM golang:1.21 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app ./cmd/worker

FROM alpine:latest AS prod
WORKDIR /
CMD apk update
CMD apk add rsync
COPY --from=build /app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
