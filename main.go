package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go-projects/internal/config"
	"go-projects/internal/db"
	"go-projects/internal/logger"
)

func main() {
	cfg := config.LoadConfig()

	log := logger.InitLogger()
	log.Info().Msg("Uygulama başlıyor")

	database := db.InitDB(cfg.DBUrl)
	defer database.Close()

	db.RunMigrations(database)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Uygulama Çalışıyor.")
	})

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	// 6. Graceful shutdown
	go func() {
		log.Info().Msgf("Sunucu %s portunda çalışıyor", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Sunucu hatası")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Info().Msg("Kapatma sinyali alındı...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Graceful shutdown başarısız")
	}

	log.Info().Msg("Sunucu kapatıldı")
}
