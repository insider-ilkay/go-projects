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

type UserHandler struct {
	userService *services.UserService
	logger zerolog.Logger
}

func NewUserHandler(db *sql.DB, logger zerolog.Logger) *UserHandler {
	return &UserHandler{
		userService: services.NewUserService(db, logger),
		logger: logger,
	}
}

func (h *UserHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	userRole, ok := middleware.GetUserRole(r)
	if !ok || userRole != string(models.RoleAdmin) {
		h.respondWithError(w, http.StatusForbidden, "forbidden", "Only admins can view all users")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Get all users - implementation needed",
	})
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID")
		return
	}

	currentUserID, ok := middleware.GetUserID(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "unauthorized", "User not authenticated")
		return
	}

	userRole, _ := middleware.GetUserRole(r)
	
	if userRole != string(models.RoleAdmin) && currentUserID != userID {
		h.respondWithError(w, http.StatusForbidden, "forbidden", "You can only view your own profile")
		return
	}

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		h.respondWithError(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}

	user.PasswordHash = ""
	h.respondWithJSON(w, http.StatusOK, user)
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID")
		return
	}

	currentUserID, ok := middleware.GetUserID(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "unauthorized", "User not authenticated")
		return
	}

	userRole, _ := middleware.GetUserRole(r)
	
	if userRole != string(models.RoleAdmin) && currentUserID != userID {
		h.respondWithError(w, http.StatusForbidden, "forbidden", "You can only update your own profile")
		return
	}

	var updateReq struct {
		Username string `json:"username,omitempty"`
		Email    string `json:"email,omitempty"`
		Role     string `json:"role,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		h.respondWithError(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}

	if updateReq.Username != "" {
		user.Username = updateReq.Username
	}
	if updateReq.Email != "" {
		user.Email = updateReq.Email
	}
	
	if updateReq.Role != "" && userRole == string(models.RoleAdmin) {
		err = h.userService.UpdateUserRole(userID, updateReq.Role, currentUserID)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "update_failed", err.Error())
			return
		}
		user.Role = updateReq.Role
	}

	user.PasswordHash = ""
	h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message": "User updated successfully",
		"user":    user,
	})
}

func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID")
		return
	}

	userRole, ok := middleware.GetUserRole(r)
	if !ok || userRole != string(models.RoleAdmin) {
		h.respondWithError(w, http.StatusForbidden, "forbidden", "Only admins can delete users")
		return
	}

	_, err = h.userService.GetUserByID(userID)
	if err != nil {
		h.respondWithError(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "User deleted successfully",
	})
}

func (h *UserHandler) respondWithError(w http.ResponseWriter, code int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   errorCode,
		"message": message,
	})
}

func (h *UserHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

