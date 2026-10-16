FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/briefing ./cmd/briefing

# Alpine, not distroless: Coolify health checks run wget through /bin/sh.
FROM alpine:3.22
RUN adduser -D -H -u 65532 nonroot
USER nonroot
COPY --from=build /out/briefing /briefing
EXPOSE 8080
ENTRYPOINT ["/briefing"]
CMD ["all"]
