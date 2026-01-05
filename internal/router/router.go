package router

import (
	"database/sql"
	"net/http"
	"os"

	"go-projects/internal/handlers"
	"go-projects/internal/middleware"
	"go-projects/internal/services"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

func SetupRouter(db *sql.DB, logger zerolog.Logger) *mux.Router {
	balanceService := services.NewBalanceService(db, logger)

	authHandler := handlers.NewAuthHandler(db, logger)
	userHandler := handlers.NewUserHandler(db, logger)
	transactionHandler := handlers.NewTransactionHandler(db, logger, balanceService)
	balanceHandler := handlers.NewBalanceHandler(db, logger)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-secret-key-change-in-production"
		logger.Warn().Msg("JWT_SECRET not set, using default key")
	}

	r := mux.NewRouter()

	rateLimiter := middleware.NewRateLimiter(rate.Limit(10), 20)

	r.Use(middleware.ErrorHandling(logger))
	r.Use(middleware.PerformanceMonitoring(logger))
	r.Use(middleware.RequestLogging(logger))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.CORS())
	r.Use(rateLimiter.Middleware())

	api := r.PathPrefix("/api/v1").Subrouter()

	auth := api.PathPrefix("/auth").Subrouter()
	auth.HandleFunc("/register", authHandler.Register).Methods("POST")
	auth.HandleFunc("/login", authHandler.Login).Methods("POST")
	
	protectedAuth := auth.PathPrefix("").Subrouter()
	protectedAuth.Use(middleware.Authentication(jwtSecret, logger))
	protectedAuth.HandleFunc("/refresh", authHandler.Refresh).Methods("POST")

	users := api.PathPrefix("/users").Subrouter()
	users.Use(middleware.Authentication(jwtSecret, logger))
	users.HandleFunc("", userHandler.GetUsers).Methods("GET")
	users.HandleFunc("/{id}", userHandler.GetUser).Methods("GET")
	users.HandleFunc("/{id}", userHandler.UpdateUser).Methods("PUT")
	users.HandleFunc("/{id}", userHandler.DeleteUser).Methods("DELETE")

	transactions := api.PathPrefix("/transactions").Subrouter()
	transactions.Use(middleware.Authentication(jwtSecret, logger))
	transactions.Use(middleware.RequestValidation())
	transactions.HandleFunc("/credit", transactionHandler.Credit).Methods("POST")
	transactions.HandleFunc("/debit", transactionHandler.Debit).Methods("POST")
	transactions.HandleFunc("/transfer", transactionHandler.Transfer).Methods("POST")
	transactions.HandleFunc("/history", transactionHandler.GetHistory).Methods("GET")
	transactions.HandleFunc("/{id}", transactionHandler.GetTransaction).Methods("GET")

	balances := api.PathPrefix("/balances").Subrouter()
	balances.Use(middleware.Authentication(jwtSecret, logger))
	balances.HandleFunc("/current", balanceHandler.GetCurrentBalance).Methods("GET")
	balances.HandleFunc("/historical", balanceHandler.GetHistoricalBalance).Methods("GET")
	balances.HandleFunc("/at-time", balanceHandler.GetBalanceAtTime).Methods("GET")
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	return r
}

