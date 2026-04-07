FROM golang:1.26.1-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/rcv-app ./cmd/server/main.go

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=build /out/rcv-app /app/rcv-app
COPY --chown=nonroot:nonroot db/migrations /app/db/migrations
COPY --chown=nonroot:nonroot ui /app/ui

ENV APP_ENV=production
EXPOSE 10000

CMD ["/app/rcv-app"]
