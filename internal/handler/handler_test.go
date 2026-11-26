package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dienton82/wallet-service/internal/domain"
	"github.com/dienton82/wallet-service/internal/service"
	"github.com/dienton82/wallet-service/internal/store"
)

// Inicializamos la API con un almacenamiento independiente.
func newTestHandler() http.Handler {
	memoryStore := store.NewMemoryStore()
	walletService := service.NewWalletService(memoryStore)

	return NewHandler(walletService).Routes()
}

// Función de apoyo para ejecutar solicitudes HTTP durante las pruebas.
func executeRequest(
	api http.Handler,
	method string,
	path string,
	body string,
	idempotencyKey string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)

	request.Header.Set("Content-Type", "application/json")

	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}

	response := httptest.NewRecorder()

	api.ServeHTTP(response, request)

	return response
}

// Validamos el código HTTP y el formato de la respuesta.
func assertResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
	expectedStatus int,
) {
	t.Helper()

	if response.Code != expectedStatus {
		t.Fatalf(
			"código esperado %d, obtenido %d. Respuesta: %s",
			expectedStatus,
			response.Code,
			response.Body.String(),
		)
	}

	if response.Header().Get("Content-Type") != "application/json" {
		t.Error("la respuesta debe tener formato application/json")
	}

	if !json.Valid(response.Body.Bytes()) {
		t.Error("la respuesta no contiene un JSON válido")
	}
}

// Validamos que los errores tengan el formato solicitado.
func assertErrorResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
	expectedStatus int,
) {
	t.Helper()

	assertResponse(t, response, expectedStatus)

	var result errorResponse

	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Error == "" {
		t.Error("se esperaba un mensaje de error descriptivo")
	}
}

// Creamos cuentas de apoyo para las pruebas HTTP.
func createHTTPAccount(
	t *testing.T,
	api http.Handler,
	body string,
) domain.Account {
	t.Helper()

	response := executeRequest(
		api,
		http.MethodPost,
		"/accounts",
		body,
		"",
	)

	assertResponse(t, response, http.StatusCreated)

	var account domain.Account

	if err := json.Unmarshal(response.Body.Bytes(), &account); err != nil {
		t.Fatal(err)
	}

	return account
}

// Consultamos el saldo de una cuenta desde la API.
func assertHTTPBalance(
	t *testing.T,
	api http.Handler,
	accountID string,
	expected int64,
) {
	t.Helper()

	response := executeRequest(
		api,
		http.MethodGet,
		"/accounts/"+accountID,
		"",
		"",
	)

	assertResponse(t, response, http.StatusOK)

	var account domain.Account

	if err := json.Unmarshal(response.Body.Bytes(), &account); err != nil {
		t.Fatal(err)
	}

	if account.Balance != expected {
		t.Errorf(
			"saldo esperado %d, obtenido %d",
			expected,
			account.Balance,
		)
	}
}

// Validamos la creación de una cuenta desde la API.
func TestCreateAccountHTTP(t *testing.T) {
	api := newTestHandler()

	account := createHTTPAccount(
		t,
		api,
		`{"owner":"Ana","initial_balance":10000}`,
	)

	if account.ID != "acc_1" ||
		account.Owner != "Ana" ||
		account.Balance != 10000 {
		t.Errorf("datos incorrectos: %+v", account)
	}
}

// Validamos los diferentes errores al crear una cuenta.
func TestCreateAccountErrorsHTTP(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			"JSON mal formado",
			`{"owner":`,
		},
		{
			"titular vacío",
			`{"owner":"","initial_balance":10000}`,
		},
		{
			"titular con espacios",
			`{"owner":"   ","initial_balance":10000}`,
		},
		{
			"saldo negativo",
			`{"owner":"Ana","initial_balance":-100}`,
		},
		{
			"saldo decimal",
			`{"owner":"Ana","initial_balance":100.50}`,
		},
		{
			"varios objetos JSON",
			`{"owner":"Ana","initial_balance":100} {}`,
		},
		{
			"cuerpo vacío",
			"",
		},
		{
			"tipo de dato incorrecto",
			`{"owner":"Ana","initial_balance":"100"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newTestHandler()

			response := executeRequest(
				api,
				http.MethodPost,
				"/accounts",
				tt.body,
				"",
			)

			assertErrorResponse(
				t,
				response,
				http.StatusBadRequest,
			)
		})
	}
}

// Consultamos una cuenta existente.
func TestGetAccountHTTP(t *testing.T) {
	api := newTestHandler()

	account := createHTTPAccount(
		t,
		api,
		`{"owner":"Ana","initial_balance":10000}`,
	)

	assertHTTPBalance(t, api, account.ID, 10000)
}

// Consultamos una cuenta que no existe.
func TestGetAccountNotFoundHTTP(t *testing.T) {
	api := newTestHandler()

	response := executeRequest(
		api,
		http.MethodGet,
		"/accounts/acc_99",
		"",
		"",
	)

	assertErrorResponse(t, response, http.StatusNotFound)
}

// Validamos una transferencia y los saldos de ambas cuentas.
func TestCreateTransferHTTP(t *testing.T) {
	api := newTestHandler()

	createHTTPAccount(
		t, api,
		`{"owner":"Ana","initial_balance":10000}`,
	)

	createHTTPAccount(
		t, api,
		`{"owner":"Carlos","initial_balance":5000}`,
	)

	response := executeRequest(
		api,
		http.MethodPost,
		"/transfers",
		`{"from_account_id":"acc_1","to_account_id":"acc_2","amount":2500}`,
		"",
	)

	assertResponse(t, response, http.StatusCreated)

	var transfer domain.Transfer

	if err := json.Unmarshal(response.Body.Bytes(), &transfer); err != nil {
		t.Fatal(err)
	}

	if transfer.ID != "trf_1" ||
		transfer.FromAccountID != "acc_1" ||
		transfer.ToAccountID != "acc_2" ||
		transfer.Amount != 2500 ||
		transfer.CreatedAt.IsZero() {
		t.Errorf("datos incorrectos: %+v", transfer)
	}

	assertHTTPBalance(t, api, "acc_1", 7500)
	assertHTTPBalance(t, api, "acc_2", 7500)
}

// Validamos los códigos HTTP de las transferencias fallidas.
func TestTransferErrorsHTTP(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
	}{
		{
			"JSON mal formado",
			`{"amount":`,
			http.StatusBadRequest,
		},
		{
			"monto en cero",
			`{"from_account_id":"acc_1","to_account_id":"acc_2","amount":0}`,
			http.StatusBadRequest,
		},
		{
			"monto negativo",
			`{"from_account_id":"acc_1","to_account_id":"acc_2","amount":-100}`,
			http.StatusBadRequest,
		},
		{
			"monto decimal",
			`{"from_account_id":"acc_1","to_account_id":"acc_2","amount":10.50}`,
			http.StatusBadRequest,
		},
		{
			"misma cuenta",
			`{"from_account_id":"acc_1","to_account_id":"acc_1","amount":100}`,
			http.StatusBadRequest,
		},
		{
			"origen inexistente",
			`{"from_account_id":"acc_99","to_account_id":"acc_2","amount":100}`,
			http.StatusNotFound,
		},
		{
			"destino inexistente",
			`{"from_account_id":"acc_1","to_account_id":"acc_99","amount":100}`,
			http.StatusNotFound,
		},
		{
			"fondos insuficientes",
			`{"from_account_id":"acc_1","to_account_id":"acc_2","amount":20000}`,
			http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newTestHandler()

			createHTTPAccount(
				t, api,
				`{"owner":"Ana","initial_balance":10000}`,
			)

			createHTTPAccount(
				t, api,
				`{"owner":"Carlos","initial_balance":5000}`,
			)

			response := executeRequest(
				api,
				http.MethodPost,
				"/transfers",
				tt.body,
				"",
			)

			assertErrorResponse(t, response, tt.status)

			// Verificamos que los saldos no cambien.
			assertHTTPBalance(t, api, "acc_1", 10000)
			assertHTTPBalance(t, api, "acc_2", 5000)
		})
	}
}

// Validamos el historial de movimientos enviados y recibidos.
func TestGetAccountTransfersHTTP(t *testing.T) {
	api := newTestHandler()

	createHTTPAccount(
		t, api,
		`{"owner":"Ana","initial_balance":10000}`,
	)

	createHTTPAccount(
		t, api,
		`{"owner":"Carlos","initial_balance":5000}`,
	)

	bodies := []string{
		`{"from_account_id":"acc_1","to_account_id":"acc_2","amount":2500}`,
		`{"from_account_id":"acc_2","to_account_id":"acc_1","amount":1000}`,
	}

	for _, body := range bodies {
		response := executeRequest(
			api,
			http.MethodPost,
			"/transfers",
			body,
			"",
		)

		assertResponse(t, response, http.StatusCreated)
	}

	for _, id := range []string{"acc_1", "acc_2"} {
		response := executeRequest(
			api,
			http.MethodGet,
			"/accounts/"+id+"/transfers",
			"",
			"",
		)

		assertResponse(t, response, http.StatusOK)

		var transfers []domain.Transfer

		if err := json.Unmarshal(response.Body.Bytes(), &transfers); err != nil {
			t.Fatal(err)
		}

		if len(transfers) != 2 {
			t.Fatalf("se esperaban dos transferencias")
		}

		if transfers[0].ID != "trf_1" ||
			transfers[1].ID != "trf_2" {
			t.Error("los movimientos no coinciden con el historial")
		}
	}
}

// Una cuenta sin movimientos debe retornar una lista vacía.
func TestEmptyTransferHistoryHTTP(t *testing.T) {
	api := newTestHandler()

	createHTTPAccount(
		t, api,
		`{"owner":"Ana","initial_balance":10000}`,
	)

	response := executeRequest(
		api,
		http.MethodGet,
		"/accounts/acc_1/transfers",
		"",
		"",
	)

	assertResponse(t, response, http.StatusOK)

	if strings.TrimSpace(response.Body.String()) != "[]" {
		t.Errorf(
			"se esperaba una lista vacía, obtenido: %s",
			response.Body.String(),
		)
	}
}

// Consultamos el historial de una cuenta inexistente.
func TestTransferHistoryNotFoundHTTP(t *testing.T) {
	api := newTestHandler()

	response := executeRequest(
		api,
		http.MethodGet,
		"/accounts/acc_99/transfers",
		"",
		"",
	)

	assertErrorResponse(t, response, http.StatusNotFound)
}

// Validamos la idempotencia utilizando el encabezado HTTP.
func TestIdempotencyHTTP(t *testing.T) {
	api := newTestHandler()

	createHTTPAccount(
		t, api,
		`{"owner":"Ana","initial_balance":10000}`,
	)

	createHTTPAccount(
		t, api,
		`{"owner":"Carlos","initial_balance":0}`,
	)

	body := `{"from_account_id":"acc_1","to_account_id":"acc_2","amount":2500}`

	first := executeRequest(
		api,
		http.MethodPost,
		"/transfers",
		body,
		"operation-001",
	)

	assertResponse(t, first, http.StatusCreated)

	second := executeRequest(
		api,
		http.MethodPost,
		"/transfers",
		body,
		"operation-001",
	)

	assertResponse(t, second, http.StatusCreated)

	if first.Body.String() != second.Body.String() {
		t.Error("las respuestas de la misma operación deben coincidir")
	}

	conflict := executeRequest(
		api,
		http.MethodPost,
		"/transfers",
		`{"from_account_id":"acc_1","to_account_id":"acc_2","amount":3000}`,
		"operation-001",
	)

	assertErrorResponse(t, conflict, http.StatusConflict)

	assertHTTPBalance(t, api, "acc_1", 7500)
	assertHTTPBalance(t, api, "acc_2", 2500)
}

// Validamos las rutas que no existen.
func TestUnknownRouteHTTP(t *testing.T) {
	api := newTestHandler()

	response := executeRequest(
		api,
		http.MethodGet,
		"/unknown",
		"",
		"",
	)

	assertErrorResponse(t, response, http.StatusNotFound)
}

// Validamos los métodos HTTP no permitidos.
func TestMethodNotAllowedHTTP(t *testing.T) {
	api := newTestHandler()

	response := executeRequest(
		api,
		http.MethodDelete,
		"/accounts",
		"",
		"",
	)

	assertErrorResponse(
		t,
		response,
		http.StatusMethodNotAllowed,
	)

	if response.Header().Get("Allow") != http.MethodPost {
		t.Error("el método permitido debe ser POST")
	}
}

// Validamos el límite del tamaño de las solicitudes.
func TestRequestBodyLimitHTTP(t *testing.T) {
	api := newTestHandler()

	body := `{"owner":"` +
		strings.Repeat("a", 1<<20) +
		`","initial_balance":100}`

	response := executeRequest(
		api,
		http.MethodPost,
		"/accounts",
		body,
		"",
	)

	assertErrorResponse(t, response, http.StatusBadRequest)
}

// Validamos todos los errores de negocio y su respuesta HTTP.
// También verificamos que los errores internos no expongan
// información sensible al usuario.
func TestHandleErrorHTTP(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{
			"titular inválido",
			domain.ErrInvalidOwner,
			http.StatusBadRequest,
			domain.ErrInvalidOwner.Error(),
		},
		{
			"saldo inicial negativo",
			domain.ErrInvalidInitialBalance,
			http.StatusBadRequest,
			domain.ErrInvalidInitialBalance.Error(),
		},
		{
			"monto inválido",
			domain.ErrInvalidAmount,
			http.StatusBadRequest,
			domain.ErrInvalidAmount.Error(),
		},
		{
			"misma cuenta",
			domain.ErrSameAccount,
			http.StatusBadRequest,
			domain.ErrSameAccount.Error(),
		},
		{
			"desbordamiento de saldo",
			domain.ErrBalanceOverflow,
			http.StatusBadRequest,
			domain.ErrBalanceOverflow.Error(),
		},
		{
			"cuenta inexistente",
			domain.ErrAccountNotFound,
			http.StatusNotFound,
			domain.ErrAccountNotFound.Error(),
		},
		{
			"fondos insuficientes",
			domain.ErrInsufficientFunds,
			http.StatusUnprocessableEntity,
			domain.ErrInsufficientFunds.Error(),
		},
		{
			"conflicto de idempotencia",
			domain.ErrIdempotencyConflict,
			http.StatusConflict,
			domain.ErrIdempotencyConflict.Error(),
		},
		{
			"error interno inesperado",
			errors.New("detalle interno del sistema"),
			http.StatusInternalServerError,
			"error interno del servidor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()

			handleError(response, tt.err)

			assertErrorResponse(t, response, tt.status)

			var result errorResponse

			if err := json.Unmarshal(
				response.Body.Bytes(),
				&result,
			); err != nil {
				t.Fatal(err)
			}

			if result.Error != tt.message {
				t.Errorf(
					"mensaje esperado %q, obtenido %q",
					tt.message,
					result.Error,
				)
			}
		})
	}
}
