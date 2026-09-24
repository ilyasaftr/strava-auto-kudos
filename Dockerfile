# syntax=docker/dockerfile:1
FROM golang:1.27-trixie AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/kudos ./cmd/kudos

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=build /out/kudos /app/kudos
ENV DATA_DIR=/data
VOLUME ["/data"]
ENTRYPOINT ["/app/kudos"]
CMD ["run"]