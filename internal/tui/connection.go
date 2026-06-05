package tui

import (
	"strings"

	"termua/internal/opcua"
)

type ServerConnection struct {
	endpointText     string
	endpoints        []opcua.Endpoint
	selectedEndpoint int
	discovering      bool
	connecting       bool
	connected        bool
	status           ServerConnectionStatus
}

type ServerConnectionStatus int

const (
	ServerConnectionIdle ServerConnectionStatus = iota
	ServerConnectionNeedsEndpoint
	ServerConnectionDiscovering
	ServerConnectionDiscoveryFailed
	ServerConnectionDiscovered
	ServerConnectionConnecting
	ServerConnectionConnected
	ServerConnectionFailed
	ServerConnectionEndpointRequiresCredentials
)

type ServerConnectionRequestKind int

const (
	ServerConnectionRequestDiscoverEndpoints ServerConnectionRequestKind = iota + 1
	ServerConnectionRequestConnectEndpoint
)

type ServerConnectionRequest struct {
	Kind     ServerConnectionRequestKind
	Endpoint string
	Connect  opcua.ConnectRequest
}

type ServerConnectionView struct {
	EndpointText         string
	Endpoints            []opcua.Endpoint
	SelectedEndpoint     int
	Discovering          bool
	Connecting           bool
	Connected            bool
	Status               ServerConnectionStatus
	HasEndpointSelection bool
}

func NewServerConnection(initialEndpoint string) ServerConnection {
	return ServerConnection{endpointText: strings.TrimSpace(initialEndpoint), status: ServerConnectionIdle}
}

func (c ServerConnection) View() ServerConnectionView {
	endpoints := make([]opcua.Endpoint, len(c.endpoints))
	copy(endpoints, c.endpoints)
	return ServerConnectionView{
		EndpointText:         c.endpointText,
		Endpoints:            endpoints,
		SelectedEndpoint:     c.selectedEndpoint,
		Discovering:          c.discovering,
		Connecting:           c.connecting,
		Connected:            c.connected,
		Status:               c.status,
		HasEndpointSelection: len(c.endpoints) > 0 && !c.connecting && !c.connected,
	}
}

func (c *ServerConnection) SetEndpointText(value string) {
	c.endpointText = strings.TrimSpace(value)
}

func (c *ServerConnection) Submit() []ServerConnectionRequest {
	endpoint := strings.TrimSpace(c.endpointText)
	if endpoint == "" {
		c.status = ServerConnectionNeedsEndpoint
		return nil
	}
	if len(c.endpoints) == 0 {
		c.discovering = true
		c.connecting = false
		c.connected = false
		c.status = ServerConnectionDiscovering
		return []ServerConnectionRequest{{Kind: ServerConnectionRequestDiscoverEndpoints, Endpoint: endpoint}}
	}
	if c.selectedEndpoint < 0 || c.selectedEndpoint >= len(c.endpoints) {
		return nil
	}
	selected := c.endpoints[c.selectedEndpoint]
	if !serverConnectionSupportsAuth(selected, opcua.AuthAnonymous) {
		c.connecting = false
		c.status = ServerConnectionEndpointRequiresCredentials
		return nil
	}
	request := opcua.ConnectRequest{
		Endpoint:       endpoint,
		SecurityPolicy: selected.SecurityPolicy,
		SecurityMode:   selected.SecurityMode,
		AuthType:       opcua.AuthAnonymous,
	}
	c.connecting = true
	c.connected = false
	c.status = ServerConnectionConnecting
	return []ServerConnectionRequest{{Kind: ServerConnectionRequestConnectEndpoint, Connect: request}}
}

func (c *ServerConnection) MoveEndpointSelection(delta int) {
	if len(c.endpoints) == 0 || c.connecting || c.connected {
		return
	}
	c.selectedEndpoint = (c.selectedEndpoint + delta + len(c.endpoints)) % len(c.endpoints)
}

func (c *ServerConnection) ApplyDiscovery(endpoints []opcua.Endpoint, err error) {
	c.discovering = false
	c.connecting = false
	c.connected = false
	if err != nil {
		c.status = ServerConnectionDiscoveryFailed
		return
	}
	c.endpoints = make([]opcua.Endpoint, len(endpoints))
	copy(c.endpoints, endpoints)
	c.selectedEndpoint = defaultServerConnectionEndpointSelection(c.endpoints)
	c.status = ServerConnectionDiscovered
}

func (c *ServerConnection) ApplyConnection(request opcua.ConnectRequest, err error) {
	_ = request
	c.connecting = false
	if err != nil {
		c.connected = false
		c.status = ServerConnectionFailed
		return
	}
	c.connected = true
	c.status = ServerConnectionConnected
}

func defaultServerConnectionEndpointSelection(endpoints []opcua.Endpoint) int {
	for i, endpoint := range endpoints {
		if serverConnectionCompactSecurityMode(endpoint.SecurityMode) == "None" && endpoint.SecurityPolicy == "None" && serverConnectionSupportsAuth(endpoint, opcua.AuthAnonymous) {
			return i
		}
	}
	for i, endpoint := range endpoints {
		if serverConnectionSupportsAuth(endpoint, opcua.AuthAnonymous) {
			return i
		}
	}
	return 0
}

func serverConnectionSupportsAuth(endpoint opcua.Endpoint, auth opcua.AuthType) bool {
	for _, token := range endpoint.UserTokenTypes {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(token), "UserTokenType"), string(auth)) {
			return true
		}
	}
	return false
}

func serverConnectionCompactSecurityMode(mode string) string {
	mode = strings.TrimPrefix(strings.TrimSpace(mode), "MessageSecurityMode")
	if mode == "" {
		return "Unknown"
	}
	return mode
}
