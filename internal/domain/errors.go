package domain

import "errors"

// Errores definidos para controlar las validaciones
// y las reglas de negocio de la billetera digital.
var (
	ErrInvalidOwner = errors.New(
		"el nombre del titular no puede estar vacío",
	)

	ErrInvalidInitialBalance = errors.New(
		"el saldo inicial no puede ser negativo",
	)

	ErrInvalidAmount = errors.New(
		"el monto de la transferencia debe ser mayor a cero",
	)

	ErrSameAccount = errors.New(
		"la cuenta de origen y destino no pueden ser iguales",
	)

	ErrAccountNotFound = errors.New(
		"la cuenta no existe",
	)

	ErrInsufficientFunds = errors.New(
		"la cuenta de origen no tiene fondos suficientes",
	)

	ErrBalanceOverflow = errors.New(
		"el saldo de la cuenta destino supera el límite permitido",
	)

	ErrIdempotencyConflict = errors.New(
		"la clave de idempotencia ya fue utilizada con otros datos",
	)
)
