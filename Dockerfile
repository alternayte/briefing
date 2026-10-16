FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/briefing ./cmd/briefing

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/briefing /briefing
EXPOSE 8080
ENTRYPOINT ["/briefing"]
CMD ["all"]
