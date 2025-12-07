package models

import "time"

type Transaction struct {
	ID         int       `json:"id"`
	FromUserID *int      `json:"from_user_id,omitempty"`
	ToUserID   *int      `json:"to_user_id,omitempty"`
	Amount     float64   `json:"amount"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type TransactionType string

const (
	TransactionTypeCredit   TransactionType = "credit"
	TransactionTypeDebit    TransactionType = "debit"
	TransactionTypeTransfer TransactionType = "transfer"
)

type TransactionStatus string

const (
	TransactionStatusPending    TransactionStatus = "pending"
	TransactionStatusCompleted  TransactionStatus = "completed"
	TransactionStatusFailed     TransactionStatus = "failed"
	TransactionStatusRolledBack TransactionStatus = "rolled_back"
)

type CreditRequest struct {
	UserID int     `json:"user_id"`
	Amount float64 `json:"amount"`
}

type DebitRequest struct {
	UserID int     `json:"user_id"`
	Amount float64 `json:"amount"`
}

type TransferRequest struct {
	FromUserID int     `json:"from_user_id"`
	ToUserID   int     `json:"to_user_id"`
	Amount     float64 `json:"amount"`
}
