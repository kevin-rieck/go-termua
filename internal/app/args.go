package app

import (
	"errors"
	"flag"
)

// LaunchOptions describes how the TUI should start.
type LaunchOptions struct {
	Endpoint              string
	ConnectionName        string
	Portable              bool
	ClientCertificatePath string
	ClientPrivateKeyPath  string
}

// ParseArgs parses the initial CLI shape without performing any OPC UA work.
func ParseArgs(args []string) (LaunchOptions, error) {
	var opts LaunchOptions

	fs := flag.NewFlagSet("termua", flag.ContinueOnError)
	fs.StringVar(&opts.ConnectionName, "connection", "", "saved connection name")
	fs.BoolVar(&opts.Portable, "portable", false, "store configuration next to the executable")
	fs.StringVar(&opts.ClientCertificatePath, "client-certificate", "", "client certificate path for secure OPC UA endpoints")
	fs.StringVar(&opts.ClientPrivateKeyPath, "client-private-key", "", "client private key path for secure OPC UA endpoints")

	if err := fs.Parse(args); err != nil {
		return LaunchOptions{}, err
	}

	remaining := fs.Args()
	if len(remaining) > 1 {
		return LaunchOptions{}, errors.New("expected at most one endpoint argument")
	}
	if len(remaining) == 1 {
		opts.Endpoint = remaining[0]
	}
	if opts.Endpoint != "" && opts.ConnectionName != "" {
		return LaunchOptions{}, errors.New("use either an endpoint argument or --connection, not both")
	}

	return opts, nil
}
