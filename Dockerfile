# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS build
WORKDIR /src

COPY go.mod ./
COPY main.go ./
COPY internal/ ./internal/

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bff .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bff /bff

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/bff"]
