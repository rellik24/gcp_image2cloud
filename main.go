// Sample storage-quickstart creates a Google Cloud Storage bucket.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/rellik24/image2cloud/cloudsql"
	"github.com/rellik24/image2cloud/cloudstorage"
)

func main() {
	log.Print("starting server...")

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", cloudsql.Votes)
	http.HandleFunc("/storage", cloudstorage.Handler)

	// Start HTTP server.
	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
