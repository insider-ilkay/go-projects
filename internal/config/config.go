package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUrl string
	Port  string
}

func LoadConfig() Config {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env dosyası bulunamadı, varsayılanlar kullanılacak")
	}

	return Config{
		DBUrl: os.Getenv("DB_URL"),
		Port:  os.Getenv("PORT"),
	}
}
