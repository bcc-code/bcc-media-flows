FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app ./cmd/worker

FROM linuxserver/ffmpeg AS runtime
RUN apt update && apt install libass9 libfreetype6-dev -y
WORKDIR /
COPY --from=build /app /worker/bin
ENTRYPOINT ["/worker/bin"]
