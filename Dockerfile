# ---- Build stage: compile a static Go binary ----
FROM golang:1.27 AS build
WORKDIR /src

# Download dependencies first so this layer is cached between code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/url-shortener .

# ---- Runtime stage: tiny image with no shell, running as non-root ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/url-shortener /url-shortener
EXPOSE 8080
# Numeric UID (distroless "nonroot" = 65532): Kubernetes' runAsNonRoot can only
# verify a numeric user, and refuses to start the container otherwise.
USER 65532:65532
ENTRYPOINT ["/url-shortener"]
