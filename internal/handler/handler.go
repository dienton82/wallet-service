package handler

import (
	"net/http"
	"path"

	"github.com/dienton82/wallet-service/internal/service"
)

// Handler recibe las solicitudes HTTP y las comunica
// con el servicio de la billetera digital.
type Handler struct {
	wallet *service.WalletService
}

// Datos necesarios para crear una cuenta.
type createAccountRequest struct {
	Owner          string `json:"owner"`
	InitialBalance *int64 `json:"initial_balance"`
}

// Datos necesarios para realizar una transferencia.
type createTransferRequest struct {
	FromAccountID string `json:"from_account_id"`
	ToAccountID   string `json:"to_account_id"`
	Amount        int64  `json:"amount"`
}

// NewHandler inicializa el controlador de la API.
func NewHandler(wallet *service.WalletService) *Handler {
	return &Handler{
		wallet: wallet,
	}
}

// Routes registra las rutas disponibles en el servicio.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /accounts", h.createAccount)
	mux.HandleFunc("GET /accounts/{id}", h.getAccount)
	mux.HandleFunc("POST /transfers", h.createTransfer)
	mux.HandleFunc("GET /accounts/{id}/transfers", h.getAccountTransfers)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Evitamos redirecciones automáticas que generen
		// respuestas fuera del formato JSON.
		if path.Clean(r.URL.EscapedPath()) != r.URL.EscapedPath() {
			writeError(
				w,
				http.StatusNotFound,
				"ruta no encontrada",
			)
			return
		}

		_, pattern := mux.Handler(r)

		// Controlamos las rutas y métodos no permitidos
		// para mantener las respuestas en formato JSON.
		if pattern == "" {
			for _, method := range []string{
				http.MethodGet,
				http.MethodPost,
			} {
				request := r.Clone(r.Context())
				request.Method = method

				if _, allowed := mux.Handler(request); allowed != "" {
					w.Header().Set("Allow", method)

					writeError(
						w,
						http.StatusMethodNotAllowed,
						"método HTTP no permitido",
					)
					return
				}
			}

			writeError(
				w,
				http.StatusNotFound,
				"ruta no encontrada",
			)
			return
		}

		mux.ServeHTTP(w, r)
	})
}

// createAccount recibe los datos y registra una nueva cuenta.
func (h *Handler) createAccount(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request createAccountRequest

	if err := decodeJSON(w, r, &request); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"el cuerpo de la solicitud contiene un JSON inválido",
		)
		return
	}

	// Validamos que el saldo inicial haya sido enviado.
	// El valor cero es válido para crear una cuenta.
	if request.InitialBalance == nil {
		writeError(
			w,
			http.StatusBadRequest,
			"el saldo inicial es obligatorio",
		)
		return
	}

	account, err := h.wallet.CreateAccount(
		request.Owner,
		*request.InitialBalance,
	)

	if err != nil {
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, account)
}

// getAccount consulta la información y el saldo de una cuenta.
func (h *Handler) getAccount(
	w http.ResponseWriter,
	r *http.Request,
) {
	accountID := r.PathValue("id")

	account, err := h.wallet.GetAccount(accountID)

	if err != nil {
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, account)
}

// createTransfer procesa una transferencia entre dos cuentas.
func (h *Handler) createTransfer(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request createTransferRequest

	if err := decodeJSON(w, r, &request); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"el cuerpo de la solicitud contiene un JSON inválido",
		)
		return
	}

	// Validamos que se hayan enviado las cuentas de la transferencia.
	if request.FromAccountID == "" || request.ToAccountID == "" {
		writeError(
			w,
			http.StatusBadRequest,
			"las cuentas de origen y destino son obligatorias",
		)
		return
	}

	// La clave es opcional y permite controlar solicitudes repetidas.
	idempotencyKey := r.Header.Get("Idempotency-Key")

	transfer, err := h.wallet.CreateTransfer(
		request.FromAccountID,
		request.ToAccountID,
		request.Amount,
		idempotencyKey,
	)

	if err != nil {
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, transfer)
}

// getAccountTransfers consulta los movimientos enviados
// y recibidos por una cuenta.
func (h *Handler) getAccountTransfers(
	w http.ResponseWriter,
	r *http.Request,
) {
	accountID := r.PathValue("id")

	transfers, err := h.wallet.GetAccountTransfers(accountID)

	if err != nil {
		handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, transfers)
}
