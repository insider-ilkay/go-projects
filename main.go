package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go-projects/internal/config"
	"go-projects/internal/db"
	"go-projects/internal/logger"
	"go-projects/internal/router"
)

func main() {
	cfg := config.LoadConfig()

	log := logger.InitLogger()
	database := db.InitDB(cfg.DBUrl)
	defer database.Close()

	db.RunMigrations(database)
	r := router.SetupRouter(database, log)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Info().Msgf("Server running on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Shutdown failed")
	}
}
