package opcua

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestIntegrationDiscoverEndpoints(t *testing.T) {
	endpoint := os.Getenv("TERMUA_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("set TERMUA_TEST_ENDPOINT to run OPC UA integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := NewClient()
	endpoints, err := client.DiscoverEndpoints(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) == 0 {
		t.Fatal("expected at least one endpoint")
	}
}
