# The Go API server.
#
# Prompts are embedded with go:embed, so the runtime image needs nothing from
# this repository but the binary — no prompt directory to mount, no template
# path to configure, and no way for the running server to disagree with the
# code it was built from. That is the property the prompt blob-hash scheme
# depends on.

FROM golang:1.26-alpine AS build
WORKDIR /src

# Dependencies first, so a source-only change does not re-download the module
# cache. go.sum is copied with go.mod or the download is unverified.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO off produces a static binary that runs on the scratch-like base below.
# -trimpath keeps build-host paths out of panics; the ldflags strip debug
# symbols, which is worth ~30% of the binary and nothing at runtime.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /out/server ./cmd/server

FROM alpine:3.21
# ca-certificates is not optional: the server calls the Anthropic API over
# TLS, and without a trust store every model call fails certificate
# verification. tzdata so timestamps render in the operator's zone rather
# than always UTC.
RUN apk add --no-cache ca-certificates tzdata

# An unprivileged user. Nothing here writes to disk — rendered documents are
# streamed back to the caller rather than persisted — so the filesystem can
# stay owned by root and unwritable.
RUN adduser -D -u 10001 rolemodel
USER rolemodel

COPY --from=build /out/server /usr/local/bin/server

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
