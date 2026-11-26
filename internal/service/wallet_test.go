package service

import (
	"errors"
	"math"
	"sync"
	"testing"

	"github.com/dienton82/wallet-service/internal/domain"
	"github.com/dienton82/wallet-service/internal/store"
)

// Inicializamos un servicio independiente para cada prueba.
func newTestWallet() *WalletService {
	return NewWalletService(store.NewMemoryStore())
}

// Función de apoyo para crear cuentas durante las pruebas.
func createTestAccount(
	t *testing.T,
	wallet *WalletService,
	owner string,
	balance int64,
) domain.Account {
	t.Helper()

	account, err := wallet.CreateAccount(owner, balance)

	if err != nil {
		t.Fatalf("error al crear la cuenta: %v", err)
	}

	return account
}

// Consultamos una cuenta y verificamos su saldo.
func assertBalance(
	t *testing.T,
	wallet *WalletService,
	accountID string,
	expected int64,
) {
	t.Helper()

	account, err := wallet.GetAccount(accountID)

	if err != nil {
		t.Fatalf("error al consultar la cuenta: %v", err)
	}

	if account.Balance != expected {
		t.Errorf(
			"saldo de %s: esperado %d, obtenido %d",
			accountID,
			expected,
			account.Balance,
		)
	}
}

// Validamos la creación de una cuenta.
func TestCreateAccount(t *testing.T) {
	wallet := newTestWallet()

	account := createTestAccount(t, wallet, "Ana", 10000)

	if account.ID != "acc_1" {
		t.Errorf("identificador inesperado: %s", account.ID)
	}

	if account.Owner != "Ana" {
		t.Errorf("titular inesperado: %s", account.Owner)
	}

	assertBalance(t, wallet, account.ID, 10000)

	// También debe permitir cuentas con saldo inicial en cero.
	emptyAccount := createTestAccount(t, wallet, "Carlos", 0)
	assertBalance(t, wallet, emptyAccount.ID, 0)
}

// Validamos los errores al crear una cuenta.
func TestCreateAccountErrors(t *testing.T) {
	tests := []struct {
		name    string
		owner   string
		balance int64
		wantErr error
	}{
		{"titular vacío", "", 1000, domain.ErrInvalidOwner},
		{"titular con espacios", "   ", 1000, domain.ErrInvalidOwner},
		{"saldo negativo", "Ana", -100, domain.ErrInvalidInitialBalance},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wallet := newTestWallet()

			_, err := wallet.CreateAccount(tt.owner, tt.balance)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf(
					"error esperado %v, obtenido %v",
					tt.wantErr,
					err,
				)
			}
		})
	}
}

// Verificamos una transferencia exitosa y sus saldos.
func TestCreateTransfer(t *testing.T) {
	wallet := newTestWallet()

	from := createTestAccount(t, wallet, "Ana", 10000)
	to := createTestAccount(t, wallet, "Carlos", 5000)

	transfer, err := wallet.CreateTransfer(
		from.ID,
		to.ID,
		2500,
		"",
	)

	if err != nil {
		t.Fatalf("error al realizar la transferencia: %v", err)
	}

	if transfer.ID != "trf_1" ||
		transfer.FromAccountID != from.ID ||
		transfer.ToAccountID != to.ID ||
		transfer.Amount != 2500 ||
		transfer.CreatedAt.IsZero() {
		t.Errorf("datos incorrectos: %+v", transfer)
	}

	assertBalance(t, wallet, from.ID, 7500)
	assertBalance(t, wallet, to.ID, 7500)

	// Verificamos el historial de ambas cuentas.
	for _, id := range []string{from.ID, to.ID} {
		transfers, err := wallet.GetAccountTransfers(id)

		if err != nil {
			t.Fatal(err)
		}

		if len(transfers) != 1 {
			t.Fatalf("se esperaba una transferencia")
		}

		if transfers[0] != transfer {
			t.Error("la transferencia no coincide con el historial")
		}
	}
}

// Validamos los errores y que los saldos no se modifiquen.
func TestTransferErrors(t *testing.T) {
	tests := []struct {
		name    string
		fromID  string
		toID    string
		amount  int64
		wantErr error
	}{
		{
			"monto en cero",
			"acc_1", "acc_2", 0,
			domain.ErrInvalidAmount,
		},
		{
			"monto negativo",
			"acc_1", "acc_2", -100,
			domain.ErrInvalidAmount,
		},
		{
			"misma cuenta",
			"acc_1", "acc_1", 100,
			domain.ErrSameAccount,
		},
		{
			"origen inexistente",
			"acc_99", "acc_2", 100,
			domain.ErrAccountNotFound,
		},
		{
			"destino inexistente",
			"acc_1", "acc_99", 100,
			domain.ErrAccountNotFound,
		},
		{
			"fondos insuficientes",
			"acc_1", "acc_2", 20000,
			domain.ErrInsufficientFunds,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wallet := newTestWallet()

			from := createTestAccount(t, wallet, "Ana", 10000)
			to := createTestAccount(t, wallet, "Carlos", 5000)

			_, err := wallet.CreateTransfer(
				tt.fromID,
				tt.toID,
				tt.amount,
				"",
			)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf(
					"error esperado %v, obtenido %v",
					tt.wantErr,
					err,
				)
			}

			// Una operación fallida no debe modificar los saldos.
			assertBalance(t, wallet, from.ID, 10000)
			assertBalance(t, wallet, to.ID, 5000)

			for _, id := range []string{from.ID, to.ID} {
				transfers, err := wallet.GetAccountTransfers(id)

				if err != nil {
					t.Fatal(err)
				}

				if len(transfers) != 0 {
					t.Error("no se deben registrar operaciones fallidas")
				}
			}
		})
	}
}

// Validamos las consultas de cuentas inexistentes.
func TestAccountNotFound(t *testing.T) {
	wallet := newTestWallet()

	_, err := wallet.GetAccount("acc_99")

	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("error inesperado: %v", err)
	}

	_, err = wallet.GetAccountTransfers("acc_99")

	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("error inesperado: %v", err)
	}
}

// Una cuenta nueva debe tener un historial vacío.
func TestEmptyTransferHistory(t *testing.T) {
	wallet := newTestWallet()

	account := createTestAccount(t, wallet, "Ana", 1000)

	transfers, err := wallet.GetAccountTransfers(account.ID)

	if err != nil {
		t.Fatal(err)
	}

	if transfers == nil || len(transfers) != 0 {
		t.Error("se esperaba una lista vacía de transferencias")
	}
}

// Ejecutamos 500 transferencias simultáneas en ambos sentidos.
func TestConcurrentTransfers(t *testing.T) {
	wallet := newTestWallet()

	from := createTestAccount(t, wallet, "Ana", 100000)
	to := createTestAccount(t, wallet, "Carlos", 100000)

	const operations = 500
	const amount int64 = 100

	var wg sync.WaitGroup

	errs := make(chan error, operations)
	start := make(chan struct{})

	for i := 0; i < operations; i++ {
		wg.Add(1)

		go func(index int) {
			defer wg.Done()

			// Esperamos para iniciar las operaciones al mismo tiempo.
			<-start

			sourceID := from.ID
			targetID := to.ID

			if index%2 != 0 {
				sourceID, targetID = targetID, sourceID
			}

			_, err := wallet.CreateTransfer(
				sourceID,
				targetID,
				amount,
				"",
			)

			errs <- err
		}(i)
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("error en transferencia concurrente: %v", err)
		}
	}

	// Los movimientos en ambos sentidos deben compensarse.
	assertBalance(t, wallet, from.ID, 100000)
	assertBalance(t, wallet, to.ID, 100000)

	for _, id := range []string{from.ID, to.ID} {
		transfers, err := wallet.GetAccountTransfers(id)

		if err != nil {
			t.Fatal(err)
		}

		if len(transfers) != operations {
			t.Errorf(
				"transferencias esperadas %d, obtenidas %d",
				operations,
				len(transfers),
			)
		}

		// Verificamos que no existan identificadores duplicados.
		ids := make(map[string]bool)

		for _, transfer := range transfers {
			if ids[transfer.ID] {
				t.Errorf("transferencia duplicada: %s", transfer.ID)
			}

			ids[transfer.ID] = true
		}
	}
}

// Verificamos que varias solicitudes no puedan gastar
// más dinero del que tiene disponible una cuenta.
func TestConcurrentInsufficientFunds(t *testing.T) {
	wallet := newTestWallet()

	from := createTestAccount(t, wallet, "Ana", 1000)
	to := createTestAccount(t, wallet, "Carlos", 0)

	const operations = 200

	var wg sync.WaitGroup

	results := make(chan error, operations)
	start := make(chan struct{})

	for i := 0; i < operations; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-start

			_, err := wallet.CreateTransfer(
				from.ID,
				to.ID,
				100,
				"",
			)

			results <- err
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	successful := 0
	rejected := 0

	for err := range results {
		switch {
		case err == nil:
			successful++

		case errors.Is(err, domain.ErrInsufficientFunds):
			rejected++

		default:
			t.Errorf("error inesperado: %v", err)
		}
	}

	if successful != 10 || rejected != 190 {
		t.Errorf(
			"operaciones exitosas: %d, rechazadas: %d",
			successful,
			rejected,
		)
	}

	assertBalance(t, wallet, from.ID, 0)
	assertBalance(t, wallet, to.ID, 1000)

	transfers, err := wallet.GetAccountTransfers(from.ID)

	if err != nil {
		t.Fatal(err)
	}

	if len(transfers) != 10 {
		t.Errorf("se esperaban 10 transferencias registradas")
	}
}

// Validamos que una solicitud repetida no descuente dinero nuevamente.
func TestTransferIdempotency(t *testing.T) {
	wallet := newTestWallet()

	from := createTestAccount(t, wallet, "Ana", 10000)
	to := createTestAccount(t, wallet, "Carlos", 0)

	first, err := wallet.CreateTransfer(
		from.ID, to.ID, 2500, "transfer-001",
	)

	if err != nil {
		t.Fatal(err)
	}

	second, err := wallet.CreateTransfer(
		from.ID, to.ID, 2500, "transfer-001",
	)

	if err != nil {
		t.Fatal(err)
	}

	if first != second {
		t.Error("se esperaba la misma transferencia")
	}

	// No permitimos reutilizar la clave con datos diferentes.
	_, err = wallet.CreateTransfer(
		from.ID, to.ID, 3000, "transfer-001",
	)

	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Errorf("error inesperado: %v", err)
	}

	assertBalance(t, wallet, from.ID, 7500)
	assertBalance(t, wallet, to.ID, 2500)

	transfers, err := wallet.GetAccountTransfers(from.ID)

	if err != nil {
		t.Fatal(err)
	}

	if len(transfers) != 1 {
		t.Error("solo se debe registrar una transferencia")
	}
}

// Probamos solicitudes simultáneas con la misma clave.
func TestConcurrentIdempotency(t *testing.T) {
	wallet := newTestWallet()

	from := createTestAccount(t, wallet, "Ana", 10000)
	to := createTestAccount(t, wallet, "Carlos", 0)

	const operations = 100

	var wg sync.WaitGroup

	results := make(chan domain.Transfer, operations)
	errs := make(chan error, operations)
	start := make(chan struct{})

	for i := 0; i < operations; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-start

			transfer, err := wallet.CreateTransfer(
				from.ID,
				to.ID,
				2500,
				"concurrent-key",
			)

			if err != nil {
				errs <- err
				return
			}

			results <- transfer
		}()
	}

	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Errorf("error inesperado: %v", err)
	}

	count := 0

	for transfer := range results {
		count++

		if transfer.ID != "trf_1" {
			t.Errorf("transferencia duplicada: %s", transfer.ID)
		}
	}

	if count != operations {
		t.Errorf("se esperaban %d respuestas", operations)
	}

	assertBalance(t, wallet, from.ID, 7500)
	assertBalance(t, wallet, to.ID, 2500)

	transfers, err := wallet.GetAccountTransfers(from.ID)

	if err != nil {
		t.Fatal(err)
	}

	if len(transfers) != 1 {
		t.Error("solo se debe registrar una transferencia")
	}
}

// Evitamos que el saldo supere el límite del tipo int64.
func TestBalanceOverflow(t *testing.T) {
	wallet := newTestWallet()

	from := createTestAccount(t, wallet, "Ana", 1000)
	to := createTestAccount(t, wallet, "Carlos", math.MaxInt64)

	_, err := wallet.CreateTransfer(
		from.ID,
		to.ID,
		100,
		"",
	)

	if !errors.Is(err, domain.ErrBalanceOverflow) {
		t.Errorf("error inesperado: %v", err)
	}

	// Los saldos deben conservarse si la operación falla.
	assertBalance(t, wallet, from.ID, 1000)
	assertBalance(t, wallet, to.ID, math.MaxInt64)
}
