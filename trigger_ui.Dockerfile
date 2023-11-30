FROM golang:1.21 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app ./cmd/trigger_ui

FROM gcr.io/distroless/static AS prod
WORKDIR /
COPY --from=build /app /app
COPY ./cmd/trigger_ui/templates /templates
USER nonroot:nonroot
ENTRYPOINT ["/app"]
