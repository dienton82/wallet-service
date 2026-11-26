package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/dienton82/wallet-service/internal/domain"
)

// Estructura utilizada para retornar los mensajes de error.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON retorna la respuesta con su respectivo código HTTP.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError retorna los errores en el formato definido para la API.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{
		Error: message,
	})
}

// decodeJSON valida y convierte el cuerpo de la solicitud.
// Se limita el tamaño para evitar solicitudes demasiado grandes.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	// Validamos que no se envíen varios objetos JSON en una solicitud.
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("el cuerpo debe contener un solo objeto JSON")
	}

	return nil
}

// handleError relaciona los errores de negocio con los códigos HTTP.
func handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidOwner),
		errors.Is(err, domain.ErrInvalidInitialBalance),
		errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrSameAccount),
		errors.Is(err, domain.ErrBalanceOverflow):

		writeError(w, http.StatusBadRequest, err.Error())

	case errors.Is(err, domain.ErrAccountNotFound):
		writeError(w, http.StatusNotFound, err.Error())

	case errors.Is(err, domain.ErrInsufficientFunds):
		writeError(w, http.StatusUnprocessableEntity, err.Error())

	case errors.Is(err, domain.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, err.Error())

	default:
		writeError(
			w,
			http.StatusInternalServerError,
			"error interno del servidor",
		)
	}
}
