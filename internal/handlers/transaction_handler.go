package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"go-projects/internal/middleware"
	"go-projects/internal/models"
	"go-projects/internal/services"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type TransactionHandler struct {
	transactionService *services.TransactionService
	logger zerolog.Logger
}

func NewTransactionHandler(db *sql.DB, logger zerolog.Logger, balanceService *services.BalanceService) *TransactionHandler {
	return &TransactionHandler{
		transactionService: services.NewTransactionService(db, logger, balanceService),
		logger: logger,
	}
}

func (h *TransactionHandler) Credit(w http.ResponseWriter, r *http.Request) {
	var req models.CreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	userRole, ok := middleware.GetUserRole(r)
	if !ok || userRole != string(models.RoleAdmin) {
		h.respondWithError(w, http.StatusForbidden, "forbidden", "Only admins can credit accounts")
		return
	}

	transaction, err := h.transactionService.Credit(&req)
	if err != nil {
		h.logger.Error().Err(err).Msg("Credit transaction failed")
		h.respondWithError(w, http.StatusBadRequest, "transaction_failed", err.Error())
		return
	}

	h.respondWithJSON(w, http.StatusCreated, transaction)
}

func (h *TransactionHandler) Debit(w http.ResponseWriter, r *http.Request) {
	var req models.DebitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	currentUserID, ok := middleware.GetUserID(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "unauthorized", "User not authenticated")
		return
	}

	userRole, _ := middleware.GetUserRole(r)
	
	if userRole != string(models.RoleAdmin) && currentUserID != req.UserID {
		h.respondWithError(w, http.StatusForbidden, "forbidden", "You can only debit your own account")
		return
	}

	transaction, err := h.transactionService.Debit(&req)
	if err != nil {
		h.logger.Error().Err(err).Msg("Debit transaction failed")
		h.respondWithError(w, http.StatusBadRequest, "transaction_failed", err.Error())
		return
	}

	h.respondWithJSON(w, http.StatusCreated, transaction)
}

func (h *TransactionHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	var req models.TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	currentUserID, ok := middleware.GetUserID(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "unauthorized", "User not authenticated")
		return
	}

	userRole, _ := middleware.GetUserRole(r)
	
	if userRole != string(models.RoleAdmin) && currentUserID != req.FromUserID {
		h.respondWithError(w, http.StatusForbidden, "forbidden", "You can only transfer from your own account")
		return
	}

	transaction, err := h.transactionService.Transfer(&req)
	if err != nil {
		h.logger.Error().Err(err).Msg("Transfer transaction failed")
		h.respondWithError(w, http.StatusBadRequest, "transaction_failed", err.Error())
		return
	}

	h.respondWithJSON(w, http.StatusCreated, transaction)
}

func (h *TransactionHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	currentUserID, ok := middleware.GetUserID(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "unauthorized", "User not authenticated")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50 // default
	offset := 0 // default

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	userRole, _ := middleware.GetUserRole(r)
	
	var userID int
	if userRole == string(models.RoleAdmin) {
		userIDStr := r.URL.Query().Get("user_id")
		if userIDStr != "" {
			if uid, err := strconv.Atoi(userIDStr); err == nil {
				userID = uid
			} else {
				userID = currentUserID
			}
		} else {
			userID = currentUserID
		}
	} else {
		userID = currentUserID
	}

	transactions, err := h.transactionService.GetUserTransactions(userID, limit, offset)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch transaction history")
		h.respondWithError(w, http.StatusInternalServerError, "fetch_failed", "Failed to fetch transaction history")
		return
	}

	h.respondWithJSON(w, http.StatusOK, transactions)
}

func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transactionIDStr := vars["id"]
	transactionID, err := strconv.Atoi(transactionIDStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_transaction_id", "Invalid transaction ID")
		return
	}

	currentUserID, ok := middleware.GetUserID(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "unauthorized", "User not authenticated")
		return
	}

	transaction, err := h.transactionService.GetTransactionByID(transactionID)
	if err != nil {
		h.respondWithError(w, http.StatusNotFound, "transaction_not_found", "Transaction not found")
		return
	}

	userRole, _ := middleware.GetUserRole(r)
	
	if userRole != string(models.RoleAdmin) {
		if (transaction.FromUserID != nil && *transaction.FromUserID != currentUserID) &&
			(transaction.ToUserID != nil && *transaction.ToUserID != currentUserID) {
			h.respondWithError(w, http.StatusForbidden, "forbidden", "You can only view your own transactions")
			return
		}
	}

	h.respondWithJSON(w, http.StatusOK, transaction)
}

func (h *TransactionHandler) respondWithError(w http.ResponseWriter, code int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   errorCode,
		"message": message,
	})
}

func (h *TransactionHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

