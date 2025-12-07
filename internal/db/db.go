package db

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func InitDB(dbURL string) *sql.DB {
	db, err := sql.Open("mysql", dbURL)
	if err != nil {
		log.Fatal("❌ Veritabanına bağlanılamadı:", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("❌ Veritabanı yanıt vermiyor:", err)
	}

	log.Println("✅ Veritabanına bağlanıldı")
	return db
}

func RunMigrations(db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(100),
			email VARCHAR(100),
			password_hash VARCHAR(255),
			role VARCHAR(50),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id INT AUTO_INCREMENT PRIMARY KEY,
			from_user_id INT,
			to_user_id INT,
			amount DECIMAL(20,2),
			type VARCHAR(50),
			status VARCHAR(50),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS balances (
			user_id INT PRIMARY KEY,
			amount DECIMAL(20,2),
			last_updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS balance_history (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			balance DECIMAL(20,2) NOT NULL,
			change_amount DECIMAL(20,2) NOT NULL,
			transaction_id INT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_user_id (user_id),
			INDEX idx_created_at (created_at),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INT AUTO_INCREMENT PRIMARY KEY,
			entity_type VARCHAR(50),
			entity_id INT,
			action VARCHAR(50),
			details TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		_, err := db.Exec(q)
		if err != nil {
			log.Fatal("Migration hatası:", err)
		}
	}
	log.Println("Migration tamamlandı")
}
