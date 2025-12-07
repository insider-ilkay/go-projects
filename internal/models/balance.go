package models

import "time"

type Balance struct {
	UserID        int       `json:"user_id"`
	Amount        float64   `json:"amount"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

type BalanceHistory struct {
	ID            int       `json:"id"`
	UserID        int       `json:"user_id"`
	Balance       float64   `json:"balance"`
	ChangeAmount  float64   `json:"change_amount"`
	TransactionID *int      `json:"transaction_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
