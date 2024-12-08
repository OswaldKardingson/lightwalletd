package main

import (
	"log"
	"net/http"
	"os"

	"github.com/PirateNetwork/lightwalletd/cmd"
	"github.com/PirateNetwork/lightwalletd/common"
	"github.com/PirateNetwork/lightwalletd/frontend"
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

	// Register the HTTP endpoints
	http.HandleFunc("/get_merkle_root", frontend.GetMerkleRootHandler)
	http.HandleFunc("/get_merkle_proof", frontend.GetMerkleProofHandler)

	// Start the HTTP server
	log.Println("Starting HTTP server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}

	// Start the application
	log.Println("LightwalletD server is starting...")

	// Execute the LightwalletD commands
	cmd.Execute()
}
