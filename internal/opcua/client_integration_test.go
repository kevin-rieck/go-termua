package opcua

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestIntegrationBrowseObjectsRoot(t *testing.T) {
	endpoint := os.Getenv("TERMUA_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("set TERMUA_TEST_ENDPOINT to run OPC UA integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := NewClient()
	endpoints, err := client.DiscoverEndpoints(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) == 0 {
		t.Fatal("expected at least one endpoint")
	}
	selected := endpoints[0]
	for _, candidate := range endpoints {
		if candidate.SecurityMode == "None" && candidate.SecurityPolicy == "None" {
			selected = candidate
			break
		}
	}
	if err := client.Connect(ctx, ConnectRequest{Endpoint: endpoint, SecurityMode: selected.SecurityMode, SecurityPolicy: selected.SecurityPolicy, AuthType: AuthAnonymous}); err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())

	children, err := client.BrowseChildren(ctx, "i=85")
	if err != nil {
		t.Fatal(err)
	}
	if len(children) == 0 {
		t.Fatal("expected Objects root to have children")
	}
}

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
