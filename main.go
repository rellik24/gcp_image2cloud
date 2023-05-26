// Sample storage-quickstart creates a Google Cloud Storage bucket.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/rellik24/image2cloud/cloudkey"
	"github.com/rellik24/image2cloud/cloudsql"
	"github.com/rellik24/image2cloud/cloudstorage"
)

var (
	port        string
	project_id  string
	key_ring    string
	key_name    string
	key_version string
)

func main() {
	log.Print("starting server...haha")

	Init()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	http.HandleFunc("/api/", cloudsql.API)

	// Start HTTP server.
	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// Init: Get ENV variable
func Init() {
	// Determine port for HTTP service.
	port = os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	project_id = os.Getenv("PROJECT_ID")
	if project_id == "" {
		log.Fatal("Can't get ENV variable: PROJECT_ID")
	}
	key_ring = os.Getenv("KEY_RING")
	if key_ring == "" {
		log.Fatal("Can't get ENV variable: KEY_RING")
	}
	key_name = os.Getenv("KEY_NAME")
	if key_name == "" {
		log.Fatal("Can't get ENV variable: KEY_NAME")
	}
	key_version = os.Getenv("KEY_VERSION")
	if key_version == "" {
		log.Fatal("Can't get ENV variable: KEY_VERSION")
	}

	cloudkey.SetHMAC(project_id, key_ring, key_name, key_version)

	bucket_name := os.Getenv("BUCKET_NAME")
	if bucket_name == "" {
		log.Fatal("Can't get ENV variable: BUCKET_NAME")
	}
	cloudstorage.Set(bucket_name)
}
