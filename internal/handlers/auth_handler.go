package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"go-projects/internal/middleware"
	"go-projects/internal/models"
	"go-projects/internal/services"

	"github.com/rs/zerolog"
)

type AuthHandler struct {
	userService *services.UserService
	authService *services.AuthService
	logger      zerolog.Logger
}

func NewAuthHandler(db *sql.DB, logger zerolog.Logger) *AuthHandler {
	userService := services.NewUserService(db, logger)
	authService := services.NewAuthService(logger)

	return &AuthHandler{
		userService: userService,
		authService: authService,
		logger:      logger,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	user, err := h.userService.Register(&req)
	if err != nil {
		h.logger.Error().Err(err).Msg("Registration failed")
		h.respondWithError(w, http.StatusBadRequest, "registration_failed", err.Error())
		return
	}

	token, err := h.authService.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		h.logger.Error().Err(err).Msg("Token generation failed")
		h.respondWithError(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate token")
		return
	}

	h.respondWithJSON(w, http.StatusCreated, models.AuthResponse{
		User:  user,
		Token: token,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	user, err := h.userService.Authenticate(&req)
	if err != nil {
		h.logger.Warn().Str("email", req.Email).Msg("Login failed")
		h.respondWithError(w, http.StatusUnauthorized, "authentication_failed", "Invalid email or password")
		return
	}

	token, err := h.authService.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		h.logger.Error().Err(err).Msg("Token generation failed")
		h.respondWithError(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate token")
		return
	}

	h.respondWithJSON(w, http.StatusOK, models.AuthResponse{
		User:  user,
		Token: token,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "unauthorized", "User not authenticated")
		return
	}

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		h.respondWithError(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}

	token, err := h.authService.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		h.logger.Error().Err(err).Msg("Token generation failed")
		h.respondWithError(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate token")
		return
	}

	h.respondWithJSON(w, http.StatusOK, models.AuthResponse{
		User:  user,
		Token: token,
	})
}

func (h *AuthHandler) respondWithError(w http.ResponseWriter, code int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   errorCode,
		"message": message,
	})
}

func (h *AuthHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

