package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

func initDB() (*Database, error) {
	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_wallets (
			user_id INTEGER,
			wallet_address TEXT,
			added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, wallet_address)
		);
	`)

	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) AddWallet(userID int64, walletAddress string) error {
	_, err := d.db.Exec(
		"INSERT INTO user_wallets (user_id, wallet_address) VALUES (?, ?)",
		userID, walletAddress,
	)
	return err
}

func (d *Database) RemoveWallet(userID int64, walletAddress string) error {
	_, err := d.db.Exec(
		"DELETE FROM user_wallets WHERE user_id = ? AND wallet_address = ?",
		userID, walletAddress,
	)
	return err
}

func (d *Database) GetWallets(userID int64) ([]string, error) {
	rows, err := d.db.Query(
		"SELECT wallet_address FROM user_wallets WHERE user_id = ?",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wallets []string
	for rows.Next() {
		var wallet string
		if err := rows.Scan(&wallet); err != nil {
			return nil, err
		}
		wallets = append(wallets, wallet)
	}
	return wallets, nil
}
