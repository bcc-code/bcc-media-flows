FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app ./cmd/httpin

FROM gcr.io/distroless/static AS prod
WORKDIR /
COPY --from=build /app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
