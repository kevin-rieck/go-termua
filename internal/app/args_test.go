package app

import "testing"

func TestParseArgsEndpoint(t *testing.T) {
	opts, err := ParseArgs([]string{"opc.tcp://localhost:4840"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Endpoint != "opc.tcp://localhost:4840" {
		t.Fatalf("endpoint = %q", opts.Endpoint)
	}
}

func TestParseArgsConnection(t *testing.T) {
	opts, err := ParseArgs([]string{"--connection", "press-line-1"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.ConnectionName != "press-line-1" {
		t.Fatalf("connection = %q", opts.ConnectionName)
	}
}

func TestParseArgsRejectsEndpointAndConnection(t *testing.T) {
	_, err := ParseArgs([]string{"--connection", "press-line-1", "opc.tcp://localhost:4840"})
	if err == nil {
		t.Fatal("expected error")
	}
}
