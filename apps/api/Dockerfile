# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build

WORKDIR /workspace/apps/api

RUN apk add --no-cache ca-certificates
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

COPY apps/api/cmd ./cmd
COPY apps/api/internal ./internal
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/keepup-api ./cmd/api

FROM scratch AS runtime

ENV APP_ENV=production \
    APP_PORT=8080

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/keepup-api /keepup-api

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/keepup-api"]
