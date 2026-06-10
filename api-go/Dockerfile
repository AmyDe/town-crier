# Town Crier API (Go)
FROM golang:1.26-alpine AS build
WORKDIR /src

# go.* matches go.mod now and go.sum once dependencies arrive.
COPY go.* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app

COPY --from=build /app/api .

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["./api"]
