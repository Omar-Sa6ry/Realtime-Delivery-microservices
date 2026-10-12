FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/alertmanager-discord .

FROM alpine:3.20
RUN adduser -D -u 10001 relay
USER relay
COPY --from=build /out/alertmanager-discord /usr/local/bin/alertmanager-discord
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/alertmanager-discord"]
