// Sample storage-quickstart creates a Google Cloud Storage bucket.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"cloud.google.com/go/storage"
)

func main() {
	ctx := context.Background()

	// Creates a client.
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Sets the name for the new bucket.
	bucketName := "rellikimage2cloud"

	// Creates a Bucket instance.
	bucket := client.Bucket(bucketName)

	// fmt.Printf("Bucket %v created.\n", bucketName)
	// Read the object1 from bucket.
	rc, err := bucket.Object("cat.png").NewReader(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer rc.Close()

	// 讀取所有檔案內容
	fileContent, err := io.ReadAll(rc)
	if err != nil {
		log.Fatal(err)
	}

	// 建立目標檔案
	err = os.WriteFile("cat.png", fileContent, 0644)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("完成")
}
