package services

import (
	"database/sql"
	"errors"
	"fmt"

	"go-projects/internal/models"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	db     *sql.DB
	logger zerolog.Logger
}

func NewUserService(db *sql.DB, logger zerolog.Logger) *UserService {
	return &UserService{
		db:     db,
		logger: logger,
	}
}

func (s *UserService) Register(req *models.RegisterRequest) (*models.User, error) {
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, errors.New("username, email, and password are required")
	}

	validRole := false
	validRoles := []string{string(models.RoleUser), string(models.RoleAdmin), string(models.RoleMerchant)}
	for _, r := range validRoles {
		if req.Role == r {
			validRole = true
			break
		}
	}
	if !validRole {
		req.Role = string(models.RoleUser)
	}
	var existingID int
	err := s.db.QueryRow("SELECT id FROM users WHERE email = ? OR username = ?", req.Email, req.Username).Scan(&existingID)
	if err == nil {
		return nil, errors.New("user with this email or username already exists")
	} else if err != sql.ErrNoRows {
		s.logger.Error().Err(err).Msg("Error checking existing user")
		return nil, fmt.Errorf("database error: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error hashing password")
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	result, err := s.db.Exec(
		"INSERT INTO users (username, email, password_hash, role) VALUES (?, ?, ?, ?)",
		req.Username, req.Email, string(hashedPassword), req.Role,
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error creating user")
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		s.logger.Error().Err(err).Msg("Error getting user ID")
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}

	user, err := s.GetUserByID(int(userID))
	if err != nil {
		return nil, err
	}

	s.logger.Info().Int("user_id", user.ID).Str("email", user.Email).Msg("User registered successfully")
	return user, nil
}

func (s *UserService) Authenticate(req *models.LoginRequest) (*models.User, error) {
	if req.Email == "" || req.Password == "" {
		return nil, errors.New("email and password are required")
	}

	var user models.User
	var passwordHash string

	err := s.db.QueryRow(
		"SELECT id, username, email, password_hash, role, created_at, updated_at FROM users WHERE email = ?",
		req.Email,
	).Scan(
		&user.ID, &user.Username, &user.Email, &passwordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("invalid email or password")
	}
	if err != nil {
		s.logger.Error().Err(err).Msg("Error querying user")
		return nil, fmt.Errorf("database error: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
		s.logger.Warn().Str("email", req.Email).Msg("Failed authentication attempt")
		return nil, errors.New("invalid email or password")
	}

	s.logger.Info().Int("user_id", user.ID).Str("email", user.Email).Msg("User authenticated successfully")
	return &user, nil
}

func (s *UserService) GetUserByID(userID int) (*models.User, error) {
	var user models.User
	err := s.db.QueryRow(
		"SELECT id, username, email, password_hash, role, created_at, updated_at FROM users WHERE id = ?",
		userID,
	).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		s.logger.Error().Err(err).Int("user_id", userID).Msg("Error fetching user")
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &user, nil
}

func (s *UserService) HasRole(userID int, requiredRole string) (bool, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return false, err
	}

	return user.Role == requiredRole, nil
}

func (s *UserService) IsAuthorized(userID int, action string, resourceID *int) (bool, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return false, err
	}

	if user.Role == string(models.RoleAdmin) {
		return true, nil
	}

	if resourceID != nil && user.ID == *resourceID {
		return true, nil
	}

	switch action {
	case "view_own_account", "update_own_account", "view_own_transactions":
		return resourceID != nil && user.ID == *resourceID, nil
	case "view_all_accounts", "view_all_transactions", "manage_users":
		return user.Role == string(models.RoleAdmin), nil
	default:
		return false, nil
	}
}

func (s *UserService) UpdateUserRole(userID int, newRole string, adminID int) error {
	isAdmin, err := s.HasRole(adminID, string(models.RoleAdmin))
	if err != nil {
		return err
	}
	if !isAdmin {
		return errors.New("only admins can update user roles")
	}

	validRoles := []string{string(models.RoleUser), string(models.RoleAdmin), string(models.RoleMerchant)}
	validRole := false
	for _, r := range validRoles {
		if newRole == r {
			validRole = true
			break
		}
	}
	if !validRole {
		return errors.New("invalid role")
	}

	_, err = s.db.Exec("UPDATE users SET role = ? WHERE id = ?", newRole, userID)
	if err != nil {
		s.logger.Error().Err(err).Int("user_id", userID).Str("new_role", newRole).Msg("Error updating user role")
		return fmt.Errorf("failed to update user role: %w", err)
	}

	s.logger.Info().Int("user_id", userID).Str("new_role", newRole).Int("admin_id", adminID).Msg("User role updated")
	return nil
}

