package services

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"go-projects/internal/models"

	"github.com/rs/zerolog"
)

type BalanceService struct {
	db     *sql.DB
	logger zerolog.Logger
	mu     sync.Map
}

func NewBalanceService(db *sql.DB, logger zerolog.Logger) *BalanceService {
	return &BalanceService{
		db:     db,
		logger: logger,
	}
}

func (s *BalanceService) getMutex(userID int) *sync.Mutex {
	mu, _ := s.mu.LoadOrStore(userID, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

func (s *BalanceService) GetBalance(userID int) (*models.Balance, error) {
	var balance models.Balance

	err := s.db.QueryRow(
		"SELECT user_id, amount, last_updated_at FROM balances WHERE user_id = ?",
		userID,
	).Scan(&balance.UserID, &balance.Amount, &balance.LastUpdatedAt)

	if err == sql.ErrNoRows {
		_, err = s.db.Exec("INSERT INTO balances (user_id, amount) VALUES (?, 0)", userID)
		if err != nil {
			s.logger.Error().Err(err).Int("user_id", userID).Msg("Error initializing balance")
			return nil, fmt.Errorf("failed to initialize balance: %w", err)
		}
		return &models.Balance{
			UserID:       userID,
			Amount:       0,
			LastUpdatedAt: time.Now(),
		}, nil
	}

	if err != nil {
		s.logger.Error().Err(err).Int("user_id", userID).Msg("Error fetching balance")
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &balance, nil
}

func (s *BalanceService) updateBalanceInTx(tx *sql.Tx, userID int, amount float64) error {
	var currentBalance float64
	err := tx.QueryRow(
		"SELECT amount FROM balances WHERE user_id = ? FOR UPDATE",
		userID,
	).Scan(&currentBalance)

	if err == sql.ErrNoRows {
		newBalance := amount
		if newBalance < 0 {
			return errors.New("insufficient balance")
		}
		_, err = tx.Exec("INSERT INTO balances (user_id, amount) VALUES (?, ?)", userID, newBalance)
		if err != nil {
			return fmt.Errorf("failed to initialize balance: %w", err)
		}

		_, err = tx.Exec(
			"INSERT INTO balance_history (user_id, balance, change_amount, transaction_id) VALUES (?, ?, ?, NULL)",
			userID, newBalance, amount,
		)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Failed to record balance history (non-critical)")
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to fetch balance: %w", err)
	}

	newBalance := currentBalance + amount
	if newBalance < 0 {
		return errors.New("insufficient balance")
	}

	_, err = tx.Exec(
		"UPDATE balances SET amount = ?, last_updated_at = NOW() WHERE user_id = ?",
		newBalance, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	_, err = tx.Exec(
		"INSERT INTO balance_history (user_id, balance, change_amount, transaction_id) VALUES (?, ?, ?, NULL)",
		userID, newBalance, amount,
	)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to record balance history (non-critical)")
	}

	return nil
}

func (s *BalanceService) UpdateBalance(userID int, amount float64) error {
	mu := s.getMutex(userID)
	mu.Lock()
	defer mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		s.logger.Error().Err(err).Msg("Error starting balance update transaction")
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	err = s.updateBalanceInTx(tx, userID, amount)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		s.logger.Error().Err(err).Msg("Error committing balance update")
		return fmt.Errorf("failed to commit balance update: %w", err)
	}

	s.logger.Info().
		Int("user_id", userID).
		Float64("amount_change", amount).
		Msg("Balance updated successfully")

	return nil
}

func (s *BalanceService) GetBalanceHistory(userID int, limit, offset int) ([]*models.BalanceHistory, error) {
	query := `
		SELECT id, user_id, balance, change_amount, transaction_id, created_at
		FROM balance_history
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(query, userID, limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Int("user_id", userID).Msg("Error fetching balance history")
		return nil, fmt.Errorf("database error: %w", err)
	}
	defer rows.Close()

	var history []*models.BalanceHistory
	for rows.Next() {
		var record models.BalanceHistory
		var transactionID sql.NullInt64

		err := rows.Scan(
			&record.ID, &record.UserID, &record.Balance, &record.ChangeAmount,
			&transactionID, &record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning balance history: %w", err)
		}

		if transactionID.Valid {
			val := int(transactionID.Int64)
			record.TransactionID = &val
		}

		history = append(history, &record)
	}

	return history, nil
}

func (s *BalanceService) CalculateBalanceFromHistory(userID int) (float64, error) {
	var totalBalance float64

	err := s.db.QueryRow(
		"SELECT COALESCE(SUM(change_amount), 0) FROM balance_history WHERE user_id = ?",
		userID,
	).Scan(&totalBalance)

	if err != nil {
		s.logger.Error().Err(err).Int("user_id", userID).Msg("Error calculating balance from history")
		return 0, fmt.Errorf("database error: %w", err)
	}

	return totalBalance, nil
}

func (s *BalanceService) ReconcileBalance(userID int) error {
	currentBalance, err := s.GetBalance(userID)
	if err != nil {
		return err
	}

	calculatedBalance, err := s.CalculateBalanceFromHistory(userID)
	if err != nil {
		return err
	}

	if currentBalance.Amount != calculatedBalance {
		s.logger.Warn().
			Int("user_id", userID).
			Float64("current_balance", currentBalance.Amount).
			Float64("calculated_balance", calculatedBalance).
			Msg("Balance discrepancy detected")
	}

	return nil
}

func (s *BalanceService) GetBalanceAtTime(userID int, targetTime time.Time) (float64, error) {
	var balance float64

	err := s.db.QueryRow(
		`SELECT balance FROM balance_history 
		 WHERE user_id = ? AND created_at <= ?
		 ORDER BY created_at DESC
		 LIMIT 1`,
		userID, targetTime,
	).Scan(&balance)

	if err == sql.ErrNoRows {
		return 0, nil
	}

	if err != nil {
		s.logger.Error().Err(err).Int("user_id", userID).Time("target_time", targetTime).Msg("Error fetching balance at time")
		return 0, fmt.Errorf("database error: %w", err)
	}

	return balance, nil
}

