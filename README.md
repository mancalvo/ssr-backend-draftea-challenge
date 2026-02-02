# SSR Backend Draftea Challenge

Challenge de sistema de pagos en Go con arquitectura limpia y diseño orientado a dominios.

## Dominios

| Dominio | Responsabilidad |
|---------|-----------------|
| **Users** | Sistema de usuarios |
| **Wallets** | Saldos en cuenta y sus operaciones |
| **Payments** | Transacciones de compra y depósito, con ledger de operaciones |
| **Offerings** | Catálogo de servicios y asignaciones a usuarios |

## Stack

Go 1.25.5+ · PostgreSQL 16 · Docker

## Arquitectura

```
cmd/api/              # Entry point
internal/
├── domains/          # Dominios de negocio
│   ├── users/        # Usuarios
│   ├── wallets/      # Saldos
│   ├── payments/     # Transacciones
│   └── offerings/    # Catálogo y entitlements
├── infrastructure/   # Adaptadores externos
└── routes/           # Handlers HTTP
pkg/                  # Utilidades compartidas
```

## Inicio Rápido

```bash
docker-compose up --build
# API en http://localhost:8080
```

## API

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| `GET` | `/wallets/{user_id}/balance` | Consultar saldo |
| `POST` | `/payments/deposit` | Depositar con tarjeta |
| `POST` | `/payments/purchase` | Comprar offering |
| `POST` | `/payments/refund` | Reembolsar compra |
| `GET` | `/payments/history/{user_id}` | Historial de transacciones |
| `GET` | `/users/{user_id}/entitlements` | Entitlements activos |

## Ejemplos

```bash
# Consultar saldo
curl http://localhost:8080/wallets/11111111-1111-1111-1111-111111111111/balance

# Depositar
curl -X POST http://localhost:8080/payments/deposit \
  -H "Content-Type: application/json" \
  -d '{"user_id": "11111111-1111-1111-1111-111111111111", "amount": 50000, "card_token": "tok_test"}'

# Contratar servicio
curl -X POST http://localhost:8080/payments/purchase \
  -H "Content-Type: application/json" \
  -d '{"user_id": "11111111-1111-1111-1111-111111111111", "offering_id": "cccccccc-cccc-cccc-cccc-cccccccccccc"}'

# Reembolsar servicio
curl -X POST http://localhost:8080/payments/refund \
  -H "Content-Type: application/json" \
  -d '{"user_id": "11111111-1111-1111-1111-111111111111", "offering_id": "cccccccc-cccc-cccc-cccc-cccccccccccc"}'

# Historial de transacciones
curl "http://localhost:8080/payments/history/11111111-1111-1111-1111-111111111111?page=1&page_size=10"

# Servicios pagador por usuario
curl http://localhost:8080/users/11111111-1111-1111-1111-111111111111/entitlements
```

## Tests

```bash
go test ./... -v
```

## Configuración

Variables de entorno en `.env.example`:

| Variable | Default | Descripción |
|----------|---------|-------------|
| `SERVER_PORT` | 8080 | Puerto del servidor |
| `DB_HOST` | localhost | Host PostgreSQL |
| `DB_NAME` | ssr_challenge | Base de datos |
