# syntax=docker/dockerfile:1
# Builds one Gotra Go binary. Select which with --build-arg SERVICE=api|gateway|agent.
#   docker build -t gotra-api      --build-arg SERVICE=api      ./services
#   docker build -t gotra-gateway  --build-arg SERVICE=gateway  ./services

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
# Ports: api 8080, gateway 8081/443. Migrations are embedded in the api binary.
EXPOSE 8080
ENTRYPOINT ["/app"]
