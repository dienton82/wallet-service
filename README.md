# Wallet Service

[![Wallet Service CI](https://github.com/dienton82/wallet-service/actions/workflows/ci.yml/badge.svg)](https://github.com/dienton82/wallet-service/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8)
![Cobertura local](https://img.shields.io/badge/Cobertura%20local-100%25-brightgreen)

## Descripción

Para esta prueba desarrollé un servicio en Go que permite crear cuentas, consultar saldos, realizar transferencias entre cuentas y consultar el historial de movimientos.

Durante el desarrollo tuve en cuenta las validaciones solicitadas y principalmente el manejo de las transferencias cuando se reciben varias solicitudes al mismo tiempo, para evitar inconsistencias en los saldos.

También implementé los puntos opcionales de la prueba y agregué GitHub Actions para automatizar las pruebas y la validación de Docker.

## Tecnologías utilizadas

- Go 1.22 o superior.
- Librerías estándar de Go.
- Docker.
- Git y GitHub Actions.

Decidí trabajar con las librerías propias de Go porque para lo solicitado no era necesario agregar frameworks ni dependencias externas.

## Estructura del proyecto

```text
wallet-service/
├── .github/
│   └── workflows/
│       └── ci.yml
├── cmd/
│   └── api/
│       ├── main.go
│       └── main_test.go
├── internal/
│   ├── domain/
│   │   ├── account.go
│   │   ├── transfer.go
│   │   └── errors.go
│   ├── handler/
│   │   ├── handler.go
│   │   ├── response.go
│   │   └── handler_test.go
│   ├── service/
│   │   ├── wallet.go
│   │   └── wallet_test.go
│   └── store/
│       └── memory.go
├── .dockerignore
├── .gitignore
├── Dockerfile
├── go.mod
└── README.md
```

Separé el código de la siguiente manera:

- **domain:** modelos de las cuentas, transferencias y errores.
- **handler:** manejo de las solicitudes y respuestas HTTP.
- **service:** lógica y validaciones del negocio.
- **store:** almacenamiento en memoria y manejo de las transferencias.
- **cmd/api:** configuración, inicio y cierre del servidor.

La idea de manejar esta estructura es tener el código organizado y que sea más fácil realizar ajustes o agregar nuevas funcionalidades más adelante.

## Cómo ejecutar el proyecto

Clonar el repositorio:

```bash
git clone https://github.com/dienton82/wallet-service.git
cd wallet-service
```

Compilar:

```bash
go build ./...
```

Ejecutar:

```bash
go run ./cmd/api
```

La información se almacena en memoria, por lo que se pierde al reiniciar el servicio.

El servicio queda disponible en:

```text
http://localhost:8080
```

Para detenerlo se utiliza Ctrl+C.

## Endpoints

| Método | Ruta | Descripción | Estado |
|---|---|---|---|
| POST | /accounts | Crear una cuenta | 201 |
| GET | /accounts/{id} | Consultar una cuenta | 200 |
| POST | /transfers | Realizar una transferencia | 201 |
| GET | /accounts/{id}/transfers | Consultar movimientos | 200 |

### Crear cuenta

POST /accounts

Solicitud:

```json
{
  "owner": "Ana",
  "initial_balance": 10000
}
```

Respuesta:

```json
{
  "id": "acc_1",
  "owner": "Ana",
  "balance": 10000
}
```

### Consultar cuenta

GET /accounts/acc_1

Respuesta:

```json
{
  "id": "acc_1",
  "owner": "Ana",
  "balance": 10000
}
```

### Realizar transferencia

POST /transfers

Para este ejemplo se deben crear previamente las dos cuentas. Después de crear la cuenta de Ana, se puede crear otra con `POST /accounts` y el cuerpo `{"owner":"Carlos","initial_balance":5000}`. Los identificadores se obtienen de las respuestas.

Solicitud:

```json
{
  "from_account_id": "acc_1",
  "to_account_id": "acc_2",
  "amount": 2500
}
```

Respuesta:

```json
{
  "id": "trf_1",
  "from_account_id": "acc_1",
  "to_account_id": "acc_2",
  "amount": 2500,
  "created_at": "2026-11-25T15:04:05Z"
}
```

### Consultar movimientos

GET /accounts/acc_1/transfers

Respuesta:

```json
[
  {
    "id": "trf_1",
    "from_account_id": "acc_1",
    "to_account_id": "acc_2",
    "amount": 2500,
    "created_at": "2026-11-25T15:04:05Z"
  }
]
```

Esta consulta muestra las transferencias enviadas y recibidas de la cuenta. Si no tiene movimientos, retorna un arreglo vacío.

## Validaciones

Para la creación de cuentas y transferencias tuve en cuenta los siguientes casos:

- Nombre del titular vacío.
- Saldo inicial negativo, omitido o nulo. El saldo cero sí es válido.
- Identificadores de origen o destino omitidos, vacíos o nulos.
- Montos menores o iguales a cero.
- Cuentas que no existen.
- Transferencias entre la misma cuenta.
- Fondos insuficientes.
- Datos JSON incorrectos o solicitudes con más de un objeto JSON.
- Rutas no normalizadas, que se rechazan con un 404 en JSON sin redirección.
- Saldos que superen el límite permitido.

Los errores se retornan en formato JSON.

Por ejemplo:

```json
{
  "error": "la cuenta de origen no tiene fondos suficientes"
}
```

| Código | Descripción |
|---|---|
| 400 | Datos incorrectos |
| 404 | Cuenta o ruta no encontrada |
| 405 | Método no permitido |
| 409 | Conflicto de idempotencia |
| 422 | Fondos insuficientes |
| 500 | Error interno del servidor |

## Tres decisiones técnicas que tomé durante el desarrollo

### 1. Manejo de los saldos

Decidí manejar los valores en centavos utilizando `int64` para evitar problemas de precisión con los decimales.

También agregué una validación para evitar que el saldo de una cuenta supere el valor máximo permitido.

### 2. Manejo de las transferencias concurrentes

Para este caso utilicé `sync.RWMutex`.

La idea es que mientras se está realizando una transferencia, otra solicitud no pueda modificar los saldos de las cuentas.

Primero se realizan todas las validaciones y, si todo está correcto, se descuenta el dinero de una cuenta, se suma a la otra y se registra la transferencia.

Si alguna validación falla, no se modifican los saldos.

Para las consultas utilicé `RLock`, permitiendo que se puedan realizar varias lecturas al mismo tiempo.

Por el alcance de la prueba decidí manejar un solo bloqueo para las operaciones de escritura, aunque más adelante se podría mejorar este proceso para manejar un mayor volumen de transferencias.

### 3. Organización del código

Preferí separar los modelos, las solicitudes HTTP, la lógica del negocio y el almacenamiento.

De esta manera no queda toda la lógica dentro de los endpoints y es más fácil realizar pruebas o identificar en qué parte del código se debe hacer un ajuste.

## Funcionalidades adicionales

### Idempotencia

Agregué el encabezado opcional Idempotency-Key para evitar que una misma transferencia se realice dos veces.

```http
Idempotency-Key: transfer-001
```

Si se vuelve a enviar la misma solicitud con la misma clave, se retorna la transferencia que ya se había registrado sin afectar nuevamente los saldos.

Si se utiliza la misma clave con datos diferentes, se retorna un error 409. Las claves se guardan en memoria y se pierden al reiniciar el servicio.

### Logs

Utilicé log/slog para generar los logs en formato JSON y poder identificar los eventos de inicio, errores y cierre del servicio.

### Apagado del servidor

Implementé Graceful Shutdown para que al detener el servidor se permita finalizar las solicitudes que se están ejecutando, con un tiempo máximo de espera de 10 segundos.

### Docker

Configuré el Dockerfile en dos etapas. En la primera se compila el proyecto y en la segunda se copia únicamente el ejecutable.

Con esto se busca tener una imagen más liviana y ejecutar el servicio como usuario sin privilegios de root.

## Pruebas realizadas

Durante el desarrollo fui realizando pruebas de las diferentes funcionalidades, tanto de los casos exitosos como de los posibles errores.

También realicé pruebas de concurrencia con 500 transferencias simultáneas, verificando que los saldos fueran correctos al finalizar el proceso.

Adicionalmente, realicé pruebas de fondos insuficientes, idempotencia concurrente, historial de movimientos y manejo del servidor.

Ejecutar todas las pruebas:

```bash
go test -v -count=1 ./...
```

Validar condiciones de carrera:

```bash
go test -race -count=1 ./...
```

Validar el código:

```bash
go vet ./...
```

Validar la compilación:

```bash
go build ./...
```

### Cobertura

Para generar la cobertura global del proyecto:

```bash
go test "-coverpkg=./..." "-coverprofile=coverage.out" -count=1 ./...
```

Consultar el resultado:

```bash
go tool cover "-func=coverage.out"
```

Generar el reporte HTML:

```bash
go tool cover "-html=coverage.out" -o coverage.html
```

Después de los últimos ajustes ejecuté nuevamente las pruebas y obtuve una cobertura global de sentencias del 100%. También pasaron las pruebas de rutas no normalizadas, campos obligatorios y creación de cuentas con saldo cero.

```text
total: (statements) 100.0%
```

También ejecuté las pruebas con race detector y no se reportaron condiciones de carrera.

Para ejecutar race detector se requiere CGO habilitado y un compilador de C compatible.

## Ejecución con Docker

Construir la imagen:

```bash
docker build -t wallet-service:1.0.0 .
```

Ejecutar el contenedor:

```bash
docker run -d --name wallet-service -p 8080:8080 wallet-service:1.0.0
```

Consultar los logs:

```bash
docker logs wallet-service
```

Detener y eliminar el contenedor:

```bash
docker stop wallet-service
docker rm wallet-service
```

## GitHub Actions

Como algo adicional a lo solicitado, configuré GitHub Actions para que al subir cambios al repositorio se realicen automáticamente las siguientes validaciones:

- Formato y análisis del código.
- Compilación.
- Pruebas unitarias.
- Race detector.
- Cobertura del código.
- Construcción y ejecución de Docker.
- Validación de los endpoints.
- Idempotencia y apagado del servidor.

La primera ejecución de GitHub Actions terminó correctamente, tanto en las validaciones de Go como en la construcción y ejecución del contenedor Docker. Allí comprobé los endpoints, la idempotencia, los saldos y el apagado ordenado.

Los resultados de cada ejecución y el reporte de cobertura (`coverage-report`) se pueden consultar en [GitHub Actions](https://github.com/dienton82/wallet-service/actions).

## Qué mejoraría si tuviera más tiempo

Lo primero que implementaría sería una base de datos como PostgreSQL, ya que actualmente toda la información se maneja en memoria y se pierde al reiniciar el servicio.

También realizaría los siguientes ajustes:

- Manejar las transferencias mediante transacciones de base de datos.
- Mejorar el manejo de concurrencia para no bloquear todas las cuentas durante una transferencia.
- Optimizar las consultas del historial mediante índices y paginación.
- Guardar las claves de idempotencia en base de datos y manejar su expiración.
- Implementar autenticación y permisos de acceso.
- Documentar los endpoints con Swagger.
- Realizar pruebas de carga para revisar el comportamiento del servicio con una mayor cantidad de solicitudes.
- Agregar herramientas de monitoreo y seguimiento de errores.

## Autor

Yeison Eduardo Álvarez García

Ingeniero de Sistemas

Prueba técnica - Periferia IT Group.
