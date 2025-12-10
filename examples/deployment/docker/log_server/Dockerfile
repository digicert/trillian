FROM golang:1.25.4-bookworm@sha256:e17419604b6d1f9bc245694425f0ec9b1b53685c80850900a376fb10cb0f70cb as build

WORKDIR /trillian

ARG GOFLAGS=""
ENV GOFLAGS=$GOFLAGS

# We need to use the token for the private repo ctutils
# ARG GITHUB_TOKEN - Removed in favor of Docker secrets
ENV GOPRIVATE=github.com/digicert/ctutils
ENV GONOSUMDB=github.com/digicert/ctutils

# Download dependencies first - this should be cacheable.
COPY go.mod go.sum ./
# We use a configurable username to support both GitHub App tokens (x-access-token) and PATs (custom or default)
ARG GITHUB_USER=x-access-token
# Use secrets mount to avoid leaking token in image history
RUN --mount=type=secret,id=github_token \
    git config --global url."https://${GITHUB_USER}:$(cat /run/secrets/github_token)@github.com/".insteadOf "https://github.com/" && \
    go mod download && \
    rm -f ~/.gitconfig

# Now add the local Trillian repo, which typically isn't cacheable.
COPY . .

# Build the server.
RUN go install ./cmd/trillian_log_server
# Run the licensing tool and save licenses, copyright notices, etc.
RUN go run github.com/google/go-licenses/v2 save ./cmd/trillian_log_server --save_path /THIRD_PARTY_NOTICES

# Make a minimal image.
FROM gcr.io/distroless/base-debian12@sha256:9e9b50d2048db3741f86a48d939b4e4cc775f5889b3496439343301ff54cdba8

COPY --from=build /go/bin/trillian_log_server /
COPY --from=build /THIRD_PARTY_NOTICES /THIRD_PARTY_NOTICES

ENTRYPOINT ["/trillian_log_server"]
