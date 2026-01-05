package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"go-projects/internal/middleware"
	"go-projects/internal/services"

	"github.com/rs/zerolog"
)

type BalanceHandler struct {
	balanceService *services.BalanceService
	logger         zerolog.Logger
}

func NewBalanceHandler(db *sql.DB, logger zerolog.Logger) *BalanceHandler {
	return &BalanceHandler{
		balanceService: services.NewBalanceService(db, logger),
		logger:         logger,
	}
}

func (h *BalanceHandler) GetCurrentBalance(w http.ResponseWriter, r *http.Request) {
	currentUserID, ok := middleware.GetUserID(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "unauthorized", "User not authenticated")
		return
	}

	userRole, _ := middleware.GetUserRole(r)
	
	var userID int
	if userRole == "admin" {
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

	balance, err := h.balanceService.GetBalance(userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch balance")
		h.respondWithError(w, http.StatusInternalServerError, "fetch_failed", "Failed to fetch balance")
		return
	}

	h.respondWithJSON(w, http.StatusOK, balance)
}

func (h *BalanceHandler) GetHistoricalBalance(w http.ResponseWriter, r *http.Request) {
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
	if userRole == "admin" {
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

	history, err := h.balanceService.GetBalanceHistory(userID, limit, offset)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch balance history")
		h.respondWithError(w, http.StatusInternalServerError, "fetch_failed", "Failed to fetch balance history")
		return
	}

	h.respondWithJSON(w, http.StatusOK, history)
}

func (h *BalanceHandler) GetBalanceAtTime(w http.ResponseWriter, r *http.Request) {
	currentUserID, ok := middleware.GetUserID(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "unauthorized", "User not authenticated")
		return
	}

	timeStr := r.URL.Query().Get("time")
	if timeStr == "" {
		h.respondWithError(w, http.StatusBadRequest, "missing_parameter", "time parameter is required")
		return
	}

	targetTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_time", "Invalid time format. Use RFC3339 format")
		return
	}

	userRole, _ := middleware.GetUserRole(r)
	
	var userID int
	if userRole == "admin" {
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

	balance, err := h.balanceService.GetBalanceAtTime(userID, targetTime)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch balance at time")
		h.respondWithError(w, http.StatusInternalServerError, "fetch_failed", "Failed to fetch balance at time")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":    userID,
		"balance":    balance,
		"at_time":    targetTime,
	})
}

func (h *BalanceHandler) respondWithError(w http.ResponseWriter, code int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   errorCode,
		"message": message,
	})
}

func (h *BalanceHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

