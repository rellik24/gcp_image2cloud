// Sample storage-quickstart creates a Google Cloud Storage bucket.
package gconn

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

func Get(w http.ResponseWriter) {
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

	// // 讀取所有檔案內容
	// fileContent, err := io.ReadAll(rc)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// // 建立目標檔案
	// err = os.WriteFile("cat.png", fileContent, 0644)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println("完成")

	query := &storage.Query{Prefix: ""}
	var names []string
	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			fmt.Println("57:", err.Error())
			break
		}
		if err != nil {
			log.Fatal(err)
			fmt.Println("61:", err.Error())
		}
		names = append(names, attrs.Name)
	}
	fmt.Fprintf(w, "List Objects: %s!\n", names)
}
