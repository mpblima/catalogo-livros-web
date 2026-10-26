FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /catalogo-api ./cmd/api
FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 catalogo
COPY --from=build /catalogo-api /usr/local/bin/catalogo-api
WORKDIR /app
COPY --from=build /src/certs/ ./certs/
USER catalogo
EXPOSE 8080
ENTRYPOINT ["catalogo-api"]
