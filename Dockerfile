FROM node:26-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/McConfig/telemt-admin/internal/version.Version=${VERSION}" -o /out/tgadmin ./cmd/tgadmin

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/tgadmin /tgadmin
USER nonroot:nonroot
EXPOSE 8090
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD ["/tgadmin", "healthcheck"]
ENTRYPOINT ["/tgadmin"]
CMD ["serve"]
