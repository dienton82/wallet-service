package domain

import "time"

// Estructura que contiene la información de una transferencia
// realizada entre dos cuentas.
type Transfer struct {
	ID            string    `json:"id"`
	FromAccountID string    `json:"from_account_id"`
	ToAccountID   string    `json:"to_account_id"`
	Amount        int64     `json:"amount"`
	CreatedAt     time.Time `json:"created_at"`
}
