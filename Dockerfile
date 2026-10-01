# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build a static binary (CGO_ENABLED=0 → no libc in the final image).
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/pdf-main ./cmd/pdf-main

# ---- run stage ----
FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/pdf-main /usr/local/bin/pdf-main
EXPOSE 8000
ENTRYPOINT ["pdf-main"]
