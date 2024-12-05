package main

import (
	"log"
	"os"

	"github.com/PirateNetwork/lightwalletd/cmd"
	"github.com/PirateNetwork/lightwalletd/common"
	"github.com/PirateNetwork/lightwalletd/merkle" // Correct import for Merkle Frontiers
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

	// Check if Merkle Frontiers feature is enabled
	enableMerkleFrontiers := os.Getenv("ENABLE_MERKLE_FRONTIERS") == "true"
	if enableMerkleFrontiers {
		log.Println("Initializing Merkle Frontiers...")

		// Initialize Merkle Frontiers components
		err := merkle.InitializeMerkleFrontiers()
		if err != nil {
			log.Fatalf("Failed to initialize Merkle Frontiers: %v", err)
		}
		log.Println("Merkle Frontiers initialized successfully.")
	} else {
		log.Println("Merkle Frontiers is disabled. Running in standard mode.")
	}

	// Start the application
	log.Println("LightwalletD server is starting...")

	// Execute the LightwalletD commands
	cmd.Execute()
}
