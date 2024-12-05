package common

import (
	"database/sql"
	"errors"
	"log"

	_ "github.com/lib/pq" // PostgreSQL driver
)

var db *sql.DB

// InitializeDB initializes the global database connection
func InitializeDB(connectionString string) error {
	var err error
	db, err = sql.Open("postgres", connectionString)
	if err != nil {
		log.Printf("Error opening database connection: %v", err)
		return err
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		log.Printf("Error pinging database: %v", err)
		return err
	}

	log.Println("Database connection successfully established")
	return nil
}

// GetDBConnection provides the global database connection
func GetDBConnection() (*sql.DB, error) {
	if db == nil {
		return nil, errors.New("database connection is not initialized")
	}
	return db, nil
}

// CloseDBConnection closes the global database connection
func CloseDBConnection() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
