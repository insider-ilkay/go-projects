package services

import (
	"database/sql"
	"errors"
	"fmt"

	"go-projects/internal/models"

	"github.com/rs/zerolog"
)

type TransactionService struct {
	db            *sql.DB
	logger        zerolog.Logger
	balanceService *BalanceService
}

func NewTransactionService(db *sql.DB, logger zerolog.Logger, balanceService *BalanceService) *TransactionService {
	return &TransactionService{
		db:             db,
		logger:         logger,
		balanceService: balanceService,
	}
}

func (s *TransactionService) Credit(req *models.CreditRequest) (*models.Transaction, error) {
	if req.Amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}

	tx, err := s.db.Begin()
	if err != nil {
		s.logger.Error().Err(err).Msg("Error starting transaction")
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO transactions (from_user_id, to_user_id, amount, type, status) VALUES (?, ?, ?, ?, ?)",
		nil, req.UserID, req.Amount, string(models.TransactionTypeCredit), string(models.TransactionStatusPending),
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error creating credit transaction")
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	transactionID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction ID: %w", err)
	}

	err = s.balanceService.updateBalanceInTx(tx, req.UserID, req.Amount)
	if err != nil {
		s.logger.Error().Err(err).Int("user_id", req.UserID).Msg("Error updating balance for credit")
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	_, err = tx.Exec("UPDATE transactions SET status = ? WHERE id = ?", string(models.TransactionStatusCompleted), transactionID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error updating transaction status")
		return nil, fmt.Errorf("failed to update transaction status: %w", err)
	}

	if err = tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("Error committing credit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	transaction, err := s.GetTransactionByID(int(transactionID))
	if err != nil {
		return nil, err
	}

	s.logger.Info().
		Int("transaction_id", transaction.ID).
		Int("user_id", req.UserID).
		Float64("amount", req.Amount).
		Msg("Credit transaction completed")

	return transaction, nil
}

func (s *TransactionService) Debit(req *models.DebitRequest) (*models.Transaction, error) {
	if req.Amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}

	balance, err := s.balanceService.GetBalance(req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to check balance: %w", err)
	}

	if balance.Amount < req.Amount {
		return nil, errors.New("insufficient balance")
	}

	tx, err := s.db.Begin()
	if err != nil {
		s.logger.Error().Err(err).Msg("Error starting transaction")
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO transactions (from_user_id, to_user_id, amount, type, status) VALUES (?, ?, ?, ?, ?)",
		req.UserID, nil, req.Amount, string(models.TransactionTypeDebit), string(models.TransactionStatusPending),
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error creating debit transaction")
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	transactionID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction ID: %w", err)
	}

	err = s.balanceService.updateBalanceInTx(tx, req.UserID, -req.Amount)
	if err != nil {
		s.logger.Error().Err(err).Int("user_id", req.UserID).Msg("Error updating balance for debit")
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	_, err = tx.Exec("UPDATE transactions SET status = ? WHERE id = ?", string(models.TransactionStatusCompleted), transactionID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error updating transaction status")
		return nil, fmt.Errorf("failed to update transaction status: %w", err)
	}

	if err = tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("Error committing debit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	transaction, err := s.GetTransactionByID(int(transactionID))
	if err != nil {
		return nil, err
	}

	s.logger.Info().
		Int("transaction_id", transaction.ID).
		Int("user_id", req.UserID).
		Float64("amount", req.Amount).
		Msg("Debit transaction completed")

	return transaction, nil
}

func (s *TransactionService) Transfer(req *models.TransferRequest) (*models.Transaction, error) {
	if req.Amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}

	if req.FromUserID == req.ToUserID {
		return nil, errors.New("cannot transfer to the same account")
	}

	balance, err := s.balanceService.GetBalance(req.FromUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to check balance: %w", err)
	}

	if balance.Amount < req.Amount {
		return nil, errors.New("insufficient balance")
	}

	tx, err := s.db.Begin()
	if err != nil {
		s.logger.Error().Err(err).Msg("Error starting transfer transaction")
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO transactions (from_user_id, to_user_id, amount, type, status) VALUES (?, ?, ?, ?, ?)",
		req.FromUserID, req.ToUserID, req.Amount, string(models.TransactionTypeTransfer), string(models.TransactionStatusPending),
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error creating transfer transaction")
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	transactionID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction ID: %w", err)
	}

	err = s.balanceService.updateBalanceInTx(tx, req.FromUserID, -req.Amount)
	if err != nil {
		s.logger.Error().Err(err).Int("from_user_id", req.FromUserID).Msg("Error debiting from sender")
		return nil, fmt.Errorf("failed to debit from sender: %w", err)
	}

	err = s.balanceService.updateBalanceInTx(tx, req.ToUserID, req.Amount)
	if err != nil {
		s.logger.Error().Err(err).Int("to_user_id", req.ToUserID).Msg("Error crediting to receiver")
		return nil, fmt.Errorf("failed to credit to receiver: %w", err)
	}

	_, err = tx.Exec("UPDATE transactions SET status = ? WHERE id = ?", string(models.TransactionStatusCompleted), transactionID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error updating transaction status")
		return nil, fmt.Errorf("failed to update transaction status: %w", err)
	}

	if err = tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("Error committing transfer transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	transaction, err := s.GetTransactionByID(int(transactionID))
	if err != nil {
		return nil, err
	}

	s.logger.Info().
		Int("transaction_id", transaction.ID).
		Int("from_user_id", req.FromUserID).
		Int("to_user_id", req.ToUserID).
		Float64("amount", req.Amount).
		Msg("Transfer transaction completed")

	return transaction, nil
}

func (s *TransactionService) RollbackTransaction(transactionID int) error {
	transaction, err := s.GetTransactionByID(transactionID)
	if err != nil {
		return err
	}

	if transaction.Status == string(models.TransactionStatusRolledBack) {
		return errors.New("transaction already rolled back")
	}

	if transaction.Status != string(models.TransactionStatusCompleted) {
		return errors.New("only completed transactions can be rolled back")
	}

	tx, err := s.db.Begin()
	if err != nil {
		s.logger.Error().Err(err).Msg("Error starting rollback transaction")
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	switch transaction.Type {
	case string(models.TransactionTypeCredit):
		if transaction.ToUserID != nil {
			err = s.balanceService.updateBalanceInTx(tx, *transaction.ToUserID, -transaction.Amount)
			if err != nil {
				return fmt.Errorf("failed to reverse credit: %w", err)
			}
		}

	case string(models.TransactionTypeDebit):
		if transaction.FromUserID != nil {
			err = s.balanceService.updateBalanceInTx(tx, *transaction.FromUserID, transaction.Amount)
			if err != nil {
				return fmt.Errorf("failed to reverse debit: %w", err)
			}
		}

	case string(models.TransactionTypeTransfer):
		if transaction.FromUserID != nil && transaction.ToUserID != nil {
			err = s.balanceService.updateBalanceInTx(tx, *transaction.FromUserID, transaction.Amount)
			if err != nil {
				return fmt.Errorf("failed to reverse transfer (sender): %w", err)
			}

			err = s.balanceService.updateBalanceInTx(tx, *transaction.ToUserID, -transaction.Amount)
			if err != nil {
				return fmt.Errorf("failed to reverse transfer (receiver): %w", err)
			}
		}

	default:
		return errors.New("unknown transaction type")
	}

	_, err = tx.Exec("UPDATE transactions SET status = ? WHERE id = ?", string(models.TransactionStatusRolledBack), transactionID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error updating transaction status to rolled_back")
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	if err = tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("Error committing rollback transaction")
		return fmt.Errorf("failed to commit rollback: %w", err)
	}

	s.logger.Info().Int("transaction_id", transactionID).Msg("Transaction rolled back successfully")
	return nil
}

func (s *TransactionService) GetTransactionByID(transactionID int) (*models.Transaction, error) {
	var transaction models.Transaction
	var fromUserID, toUserID sql.NullInt64

	err := s.db.QueryRow(
		"SELECT id, from_user_id, to_user_id, amount, type, status, created_at FROM transactions WHERE id = ?",
		transactionID,
	).Scan(
		&transaction.ID, &fromUserID, &toUserID, &transaction.Amount,
		&transaction.Type, &transaction.Status, &transaction.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("transaction not found")
	}
	if err != nil {
		s.logger.Error().Err(err).Int("transaction_id", transactionID).Msg("Error fetching transaction")
		return nil, fmt.Errorf("database error: %w", err)
	}

	if fromUserID.Valid {
		val := int(fromUserID.Int64)
		transaction.FromUserID = &val
	}
	if toUserID.Valid {
		val := int(toUserID.Int64)
		transaction.ToUserID = &val
	}

	return &transaction, nil
}

func (s *TransactionService) GetUserTransactions(userID int, limit, offset int) ([]*models.Transaction, error) {
	query := `
		SELECT id, from_user_id, to_user_id, amount, type, status, created_at 
		FROM transactions 
		WHERE from_user_id = ? OR to_user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(query, userID, userID, limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Int("user_id", userID).Msg("Error fetching user transactions")
		return nil, fmt.Errorf("database error: %w", err)
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		var transaction models.Transaction
		var fromUserID, toUserID sql.NullInt64

		err := rows.Scan(
			&transaction.ID, &fromUserID, &toUserID, &transaction.Amount,
			&transaction.Type, &transaction.Status, &transaction.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning transaction: %w", err)
		}

		if fromUserID.Valid {
			val := int(fromUserID.Int64)
			transaction.FromUserID = &val
		}
		if toUserID.Valid {
			val := int(toUserID.Int64)
			transaction.ToUserID = &val
		}

		transactions = append(transactions, &transaction)
	}

	return transactions, nil
}

