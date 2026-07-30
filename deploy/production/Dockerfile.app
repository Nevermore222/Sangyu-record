FROM golang:1.26 AS app-build
ENV GOPROXY=https://proxy.golang.org,direct
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -o /out/worker ./cmd/worker

FROM golang:1.26 AS migrate-build
ENV GOPROXY=https://proxy.golang.org,direct
RUN CGO_ENABLED=0 GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@v3.27.3

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S sangyu \
    && adduser -S -G sangyu sangyu
USER sangyu

FROM runtime AS api
COPY --from=app-build /out/api /usr/local/bin/api
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=10 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz
ENTRYPOINT ["/usr/local/bin/api"]

FROM runtime AS worker
COPY --from=app-build /out/worker /usr/local/bin/worker
ENTRYPOINT ["/usr/local/bin/worker"]

FROM runtime AS migrate
COPY --from=migrate-build /out/goose /usr/local/bin/goose
COPY migrations /migrations
ENTRYPOINT ["/usr/local/bin/goose", "-dir", "/migrations"]
CMD ["up"]
