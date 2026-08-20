FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /ledgerd ./cmd/ledgerd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 65532 nonroot
COPY --from=build /ledgerd /ledgerd
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/ledgerd"]
