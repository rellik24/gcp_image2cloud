// Sample storage-quickstart creates a Google Cloud Storage bucket.
package cloudstorage

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

var (
	bucketName string
)

// ListObjects: 存取 Storage 資料
func ListObjects(w http.ResponseWriter, account string) {
	ctx := context.Background()

	// Creates a client.
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Creates a Bucket instance.
	bucket := client.Bucket(bucketName)

	query := &storage.Query{Prefix: account}
	var names []string
	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		names = append(names, attrs.Name)
	}
	fmt.Fprintf(w, "List Objects: %s!\n", names)
}

func Set(name string) {
	bucketName = name
}
