package common

import (
	"database/sql"
	"errors"
	"log"

	_ "github.com/lib/pq" // PostgreSQL driver
)

var db *sql.DB

const createMerkleTreeTable = `
CREATE TABLE IF NOT EXISTS merkle_tree (
	id INTEGER PRIMARY KEY,
	tree_data JSONB NOT NULL
);
`

const createMerkleFrontiersTable = `
CREATE TABLE IF NOT EXISTS merkle_frontiers (
	block_height INTEGER PRIMARY KEY,
	frontier_data JSONB NOT NULL,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

// InitializeDB initializes the global database connection and sets up schemas.
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

	// Initialize database schemas
	err = initSchemas(db)
	if err != nil {
		log.Printf("Error initializing database schemas: %v", err)
		return err
	}

	return nil
}

// initSchemas sets up the required database tables.
func initSchemas(db *sql.DB) error {
	_, err := db.Exec(createMerkleTreeTable)
	if err != nil {
		return err
	}
	_, err = db.Exec(createMerkleFrontiersTable)
	if err != nil {
		return err
	}
	log.Println("Database tables initialized successfully")
	return nil
}

// GetDBConnection provides the global database connection.
func GetDBConnection() (*sql.DB, error) {
	if db == nil {
		return nil, errors.New("database connection is not initialized")
	}
	return db, nil
}

// CloseDBConnection closes the global database connection.
func CloseDBConnection() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
