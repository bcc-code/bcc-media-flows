package directus

import (
	"fmt"
	"os"
	"testing"
)

func TestAssetExists(t *testing.T) {
	baseURL := "http://localhost:8055"
	apiKey := "dYKUnnzVvKG_CkpNzKhQe2_CHkprxanz"
	mediabankenID := "VX-50727822" // TODO: Replace with a real mediabanken_id for testing

	client := NewClient(baseURL, apiKey)
	exists, err := client.AssetExists(mediabankenID)
	if err != nil {
		t.Fatalf("AssetExists failed: %v", err)
	}
	fmt.Fprintf(os.Stdout, "AssetExists for mediabanken_id=%s: %v\n", mediabankenID, exists)
}
