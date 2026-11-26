package domain

// Estructura que representa una cuenta de la billetera digital.
// El saldo se maneja en centavos para evitar errores de precisión.
type Account struct {
	ID      string `json:"id"`
	Owner   string `json:"owner"`
	Balance int64  `json:"balance"`
}
