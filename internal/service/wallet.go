package service

import (
	"strings"

	"github.com/dienton82/wallet-service/internal/domain"
	"github.com/dienton82/wallet-service/internal/store"
)

// WalletService se encarga de gestionar las operaciones
// y validaciones de negocio de la billetera digital.
type WalletService struct {
	store *store.MemoryStore
}

// NewWalletService inicializa el servicio con el
// almacenamiento que se va a utilizar.
func NewWalletService(memoryStore *store.MemoryStore) *WalletService {
	return &WalletService{
		store: memoryStore,
	}
}

// CreateAccount valida los datos antes de registrar una cuenta.
func (s *WalletService) CreateAccount(
	owner string,
	initialBalance int64,
) (domain.Account, error) {

	// Eliminamos los espacios al inicio y al final del nombre.
	owner = strings.TrimSpace(owner)

	if owner == "" {
		return domain.Account{}, domain.ErrInvalidOwner
	}

	if initialBalance < 0 {
		return domain.Account{}, domain.ErrInvalidInitialBalance
	}

	return s.store.CreateAccount(owner, initialBalance), nil
}

// GetAccount permite consultar la información y el saldo
// actual de una cuenta.
func (s *WalletService) GetAccount(
	id string,
) (domain.Account, error) {

	return s.store.GetAccount(id)
}

// CreateTransfer solicita el movimiento de dinero entre dos cuentas.
// El almacenamiento controla las validaciones de saldo y la
// actualización de ambas cuentas dentro de una operación atómica.
func (s *WalletService) CreateTransfer(
	fromID string,
	toID string,
	amount int64,
	idempotencyKey string,
) (domain.Transfer, error) {

	return s.store.CreateTransfer(
		fromID,
		toID,
		amount,
		idempotencyKey,
	)
}

// GetAccountTransfers consulta el historial de movimientos
// enviados y recibidos por una cuenta.
func (s *WalletService) GetAccountTransfers(
	accountID string,
) ([]domain.Transfer, error) {

	return s.store.GetAccountTransfers(accountID)
}
