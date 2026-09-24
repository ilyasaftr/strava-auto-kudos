# syntax=docker/dockerfile:1
FROM golang:1.27-trixie AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN mkdir -p /tmp/kudos-data && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/kudos ./cmd/kudos

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=build /out/kudos /app/kudos
COPY --from=build --chown=nonroot:nonroot --chmod=700 /tmp/kudos-data /data
ENV DATA_DIR=/data
VOLUME ["/data"]
ENTRYPOINT ["/app/kudos"]
CMD ["run"]