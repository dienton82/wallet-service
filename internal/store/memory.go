package store

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/dienton82/wallet-service/internal/domain"
)

// Registro utilizado para controlar transferencias duplicadas.
type idempotencyRecord struct {
	transfer domain.Transfer
}

// MemoryStore almacena las cuentas y transferencias en memoria.
// El mutex permite controlar el acceso concurrente a los datos.
type MemoryStore struct {
	mu          sync.RWMutex
	accounts    map[string]domain.Account
	transfers   []domain.Transfer
	idempotency map[string]idempotencyRecord
	accountSeq  uint64
	transferSeq uint64
}

// NewMemoryStore inicializa el almacenamiento de la billetera.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		accounts:    make(map[string]domain.Account),
		transfers:   make([]domain.Transfer, 0),
		idempotency: make(map[string]idempotencyRecord),
	}
}

// CreateAccount registra una cuenta y genera su identificador.
func (s *MemoryStore) CreateAccount(
	owner string,
	initialBalance int64,
) domain.Account {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.accountSeq++

	account := domain.Account{
		ID:      fmt.Sprintf("acc_%d", s.accountSeq),
		Owner:   owner,
		Balance: initialBalance,
	}

	s.accounts[account.ID] = account

	return account
}

// GetAccount consulta una cuenta por su identificador.
func (s *MemoryStore) GetAccount(
	id string,
) (domain.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	account, exists := s.accounts[id]

	if !exists {
		return domain.Account{}, domain.ErrAccountNotFound
	}

	return account, nil
}

// CreateTransfer realiza el movimiento de dinero entre dos cuentas.
// Las validaciones y la actualización de los saldos se realizan
// dentro del mismo bloqueo para garantizar la atomicidad.
func (s *MemoryStore) CreateTransfer(
	fromID string,
	toID string,
	amount int64,
	idempotencyKey string,
) (domain.Transfer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validamos los datos antes de realizar el movimiento.
	if amount <= 0 {
		return domain.Transfer{}, domain.ErrInvalidAmount
	}

	if fromID == toID {
		return domain.Transfer{}, domain.ErrSameAccount
	}

	// Verificamos si la transferencia ya fue procesada.
	if idempotencyKey != "" {
		if record, exists := s.idempotency[idempotencyKey]; exists {
			transfer := record.transfer

			if transfer.FromAccountID != fromID ||
				transfer.ToAccountID != toID ||
				transfer.Amount != amount {
				return domain.Transfer{}, domain.ErrIdempotencyConflict
			}

			return transfer, nil
		}
	}

	// Consultamos las cuentas de origen y destino.
	from, exists := s.accounts[fromID]

	if !exists {
		return domain.Transfer{}, domain.ErrAccountNotFound
	}

	to, exists := s.accounts[toID]

	if !exists {
		return domain.Transfer{}, domain.ErrAccountNotFound
	}

	// Validamos que la cuenta de origen tenga fondos suficientes.
	if from.Balance < amount {
		return domain.Transfer{}, domain.ErrInsufficientFunds
	}

	// Evitamos superar el límite permitido por el tipo int64.
	if to.Balance > math.MaxInt64-amount {
		return domain.Transfer{}, domain.ErrBalanceOverflow
	}

	// Actualizamos ambos saldos después de validar la operación.
	from.Balance -= amount
	to.Balance += amount

	s.transferSeq++

	transfer := domain.Transfer{
		ID:            fmt.Sprintf("trf_%d", s.transferSeq),
		FromAccountID: fromID,
		ToAccountID:   toID,
		Amount:        amount,
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
	}

	// Guardamos los nuevos saldos y el registro de la transferencia.
	s.accounts[fromID] = from
	s.accounts[toID] = to
	s.transfers = append(s.transfers, transfer)

	// Guardamos la clave para evitar procesar nuevamente la operación.
	if idempotencyKey != "" {
		s.idempotency[idempotencyKey] = idempotencyRecord{
			transfer: transfer,
		}
	}

	return transfer, nil
}

// GetAccountTransfers consulta las transferencias enviadas
// y recibidas por una cuenta.
func (s *MemoryStore) GetAccountTransfers(
	accountID string,
) ([]domain.Transfer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.accounts[accountID]; !exists {
		return nil, domain.ErrAccountNotFound
	}

	result := make([]domain.Transfer, 0)

	for _, transfer := range s.transfers {
		if transfer.FromAccountID == accountID ||
			transfer.ToAccountID == accountID {
			result = append(result, transfer)
		}
	}

	return result, nil
}
