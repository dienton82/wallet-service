
# Primera etapa: compilamos el servicio.
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copiamos la configuración del módulo.
COPY go.mod ./

# Copiamos el código fuente de la aplicación.
COPY cmd ./cmd
COPY internal ./internal

# Compilamos el servicio sin dependencias externas de C.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /wallet-service ./cmd/api

# Segunda etapa: utilizamos una imagen mínima.
FROM scratch

# Copiamos únicamente el ejecutable generado.
COPY --from=builder /wallet-service /wallet-service

# Ejecutamos el servicio con un usuario sin privilegios.
USER 65532:65532

# Puerto utilizado por la API.
EXPOSE 8080

# Iniciamos el servicio.
ENTRYPOINT ["/wallet-service"]
