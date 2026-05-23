FROM golang:1.23 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations

RUN CGO_ENABLED=0 GOOS=linux go build -o /conduit ./cmd/server

FROM gcr.io/distroless/static-debian12

COPY --from=build /conduit /conduit

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/conduit"]
