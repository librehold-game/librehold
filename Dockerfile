# The server image: the Go binary plus both web builds.

FROM --platform=$BUILDPLATFORM node:26.12.0 AS web
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build:web

FROM --platform=$BUILDPLATFORM golang:1.27.3 AS server
ARG TARGETOS TARGETARCH
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=server /out/server /app/server
COPY --from=web /src/dist/account /app/web/account
COPY --from=web /src/dist/game /app/web/game
ENV LH_HTTP_ADDR=:8080 LH_WEB_DIR=/app/web
EXPOSE 8080
ENTRYPOINT ["/app/server"]
