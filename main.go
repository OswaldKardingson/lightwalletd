package main

import (
	"log"
	"os"

	"github.com/PirateNetwork/lightwalletd/cmd"
	"github.com/PirateNetwork/lightwalletd/common"
)

func main() {
	// Get the database connection string from an environment variable or configuration
	dbConnectionString := os.Getenv("DATABASE_URL")
	if dbConnectionString == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	// Initialize the database connection
	err := common.InitializeDB(dbConnectionString)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer common.CloseDBConnection()

	// Start the application
	log.Println("LightwalletD server is starting...")

	// Execute the LightwalletD commands
	cmd.Execute()
}
