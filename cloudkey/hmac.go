package cloudkey

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
)

var (
	fullKeyName string
)

// signMac will sign a plaintext message using the HMAC algorithm.
func SignMac(w io.Writer, data string) error {
	// Create the client.
	ctx := context.Background()
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create kms client: %v", err)
	}
	defer client.Close()

	// Build the request.
	req := &kmspb.MacSignRequest{
		Name: fullKeyName,
		Data: []byte(data),
	}

	// Generate HMAC of data.
	result, err := client.MacSign(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to hmac sign: %v", err)
	}

	// The data comes back as raw bytes, which may include non-printable
	// characters. This base64-encodes the result so it can be printed below.
	encodedSignature := base64.StdEncoding.EncodeToString(result.Mac)

	fmt.Fprintf(w, "Signature: %s", encodedSignature)
	return nil
}
