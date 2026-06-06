package tui

import (
	"strings"

	"termua/internal/opcua"
)

type ServerConnection struct {
	endpointText          string
	endpoints             []opcua.Endpoint
	selectedEndpoint      int
	authTypes             []string
	selectedAuthType      int
	username              string
	password              string
	clientCertificatePath string
	clientPrivateKeyPath  string
	discovering           bool
	connecting            bool
	connected             bool
	lastError             string
	status                ServerConnectionStatus
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
	ServerConnectionSelectingAuthType
	ServerConnectionEnteringCredentials
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
	EndpointText               string
	Endpoints                  []opcua.Endpoint
	SelectedEndpoint           int
	Discovering                bool
	Connecting                 bool
	Connected                  bool
	Status                     ServerConnectionStatus
	HasEndpointSelection       bool
	AuthTypes                  []string
	SelectedAuthType           int
	HasAuthTypeSelection       bool
	LastError                  string
	HasClientCertificateAndKey bool
}

func NewServerConnection(initialEndpoint string) ServerConnection {
	return ServerConnection{endpointText: strings.TrimSpace(initialEndpoint), status: ServerConnectionIdle}
}

func (c ServerConnection) View() ServerConnectionView {
	endpoints := make([]opcua.Endpoint, len(c.endpoints))
	copy(endpoints, c.endpoints)
	authTypes := make([]string, len(c.authTypes))
	copy(authTypes, c.authTypes)
	return ServerConnectionView{
		EndpointText:               c.endpointText,
		Endpoints:                  endpoints,
		SelectedEndpoint:           c.selectedEndpoint,
		Discovering:                c.discovering,
		Connecting:                 c.connecting,
		Connected:                  c.connected,
		Status:                     c.status,
		HasEndpointSelection:       len(c.endpoints) > 0 && !c.connecting && !c.connected && (c.status == ServerConnectionDiscovered || c.status == ServerConnectionFailed || c.status == ServerConnectionEndpointRequiresCredentials),
		AuthTypes:                  authTypes,
		SelectedAuthType:           c.selectedAuthType,
		HasAuthTypeSelection:       len(c.authTypes) > 0 && !c.connecting && !c.connected && c.status == ServerConnectionSelectingAuthType,
		LastError:                  c.lastError,
		HasClientCertificateAndKey: c.clientCertificatePath != "" && c.clientPrivateKeyPath != "",
	}
}

func (c *ServerConnection) SetEndpointText(value string) {
	c.endpointText = strings.TrimSpace(value)
}

func (c *ServerConnection) SetClientCertificatePaths(certificatePath, privateKeyPath string) {
	c.clientCertificatePath = strings.TrimSpace(certificatePath)
	c.clientPrivateKeyPath = strings.TrimSpace(privateKeyPath)
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
		c.lastError = ""
		c.status = ServerConnectionDiscovering
		return []ServerConnectionRequest{{Kind: ServerConnectionRequestDiscoverEndpoints, Endpoint: endpoint}}
	}
	if c.selectedEndpoint < 0 || c.selectedEndpoint >= len(c.endpoints) {
		return nil
	}
	selected := c.endpoints[c.selectedEndpoint]
	authTypes := serverConnectionAuthTypes(selected)
	if len(authTypes) > 1 {
		c.authTypes = authTypes
		c.selectedAuthType = 0
		c.lastError = ""
		c.status = ServerConnectionSelectingAuthType
		return nil
	}
	if len(authTypes) == 1 && opcua.AuthType(authTypes[0]) == opcua.AuthUsername {
		c.connecting = false
		c.status = ServerConnectionEnteringCredentials
		return nil
	}
	if len(authTypes) == 1 && opcua.AuthType(authTypes[0]) != opcua.AuthAnonymous {
		c.connecting = false
		c.authTypes = nil
		c.selectedAuthType = 0
		c.lastError = "unsupported authentication mode: " + authTypes[0]
		c.status = ServerConnectionFailed
		return nil
	}
	if !serverConnectionSupportsAuth(selected, opcua.AuthAnonymous) {
		c.connecting = false
		c.lastError = "endpoint does not advertise supported authentication"
		c.status = ServerConnectionEndpointRequiresCredentials
		return nil
	}
	if c.secureEndpointRequiresCertificate(selected) {
		c.failMissingSecureEndpointCertificate()
		return nil
	}
	request := c.connectRequest(selected, opcua.AuthAnonymous)
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

func (c *ServerConnection) MoveAuthTypeSelection(delta int) {
	if len(c.authTypes) == 0 || c.connecting || c.connected {
		return
	}
	c.selectedAuthType = (c.selectedAuthType + delta + len(c.authTypes)) % len(c.authTypes)
}

func (c *ServerConnection) SelectAuthType(index int) []ServerConnectionRequest {
	if index < 0 || index >= len(c.authTypes) || c.selectedEndpoint < 0 || c.selectedEndpoint >= len(c.endpoints) {
		return nil
	}
	authType := opcua.AuthType(c.authTypes[index])
	if authType == opcua.AuthUsername {
		c.lastError = ""
		c.status = ServerConnectionEnteringCredentials
		return nil
	}
	if authType != opcua.AuthAnonymous {
		c.connecting = false
		c.lastError = "unsupported authentication mode: " + string(authType)
		c.status = ServerConnectionSelectingAuthType
		return nil
	}
	selected := c.endpoints[c.selectedEndpoint]
	if c.secureEndpointRequiresCertificate(selected) {
		c.failMissingSecureEndpointCertificate()
		return nil
	}

	request := c.connectRequest(selected, authType)
	c.lastError = ""
	c.connecting = true
	c.connected = false
	c.status = ServerConnectionConnecting
	return []ServerConnectionRequest{{Kind: ServerConnectionRequestConnectEndpoint, Connect: request}}
}

func (c *ServerConnection) SetCredentials(username, password string) {
	c.username = username
	c.password = password
}

func (c *ServerConnection) SubmitCredentials() []ServerConnectionRequest {
	if c.selectedEndpoint < 0 || c.selectedEndpoint >= len(c.endpoints) {
		return nil
	}
	selected := c.endpoints[c.selectedEndpoint]
	if c.secureEndpointRequiresCertificate(selected) {
		c.failMissingSecureEndpointCertificate()
		return nil
	}

	request := c.connectRequest(selected, opcua.AuthUsername)
	request.Username = c.username
	request.Password = c.password
	c.lastError = ""
	c.connecting = true
	c.connected = false
	c.status = ServerConnectionConnecting
	return []ServerConnectionRequest{{Kind: ServerConnectionRequestConnectEndpoint, Connect: request}}
}

func (c *ServerConnection) Back() {
	switch c.status {
	case ServerConnectionSelectingAuthType:
		c.authTypes = nil
		c.selectedAuthType = 0
		c.status = ServerConnectionDiscovered
	case ServerConnectionEnteringCredentials:
		c.username = ""
		c.password = ""
		if len(c.authTypes) > 0 {
			c.status = ServerConnectionSelectingAuthType
		} else {
			c.status = ServerConnectionDiscovered
		}
	}
}

func (c *ServerConnection) ApplyDiscovery(endpoints []opcua.Endpoint, err error) {
	c.discovering = false
	c.connecting = false
	c.connected = false
	c.authTypes = nil
	c.selectedAuthType = 0
	if err != nil {
		c.status = ServerConnectionDiscoveryFailed
		c.lastError = err.Error()
		return
	}
	c.endpoints = make([]opcua.Endpoint, len(endpoints))
	copy(c.endpoints, endpoints)
	c.selectedEndpoint = c.defaultEndpointSelection()
	c.lastError = ""
	c.status = ServerConnectionDiscovered
}

func (c *ServerConnection) ApplyConnection(request opcua.ConnectRequest, err error) {
	_ = request
	c.connecting = false
	if err != nil {
		c.connected = false
		c.authTypes = nil
		c.selectedAuthType = 0
		c.status = ServerConnectionFailed
		c.lastError = err.Error()
		return
	}
	c.connected = true
	c.authTypes = nil
	c.selectedAuthType = 0
	c.lastError = ""
	c.status = ServerConnectionConnected
}

func (c ServerConnection) defaultEndpointSelection() int {
	for i, endpoint := range c.endpoints {
		if !c.endpointCanStartConnection(endpoint) {
			continue
		}
		if serverConnectionCompactSecurityMode(endpoint.SecurityMode) == "None" && endpoint.SecurityPolicy == "None" && serverConnectionSupportsAuth(endpoint, opcua.AuthAnonymous) {
			return i
		}
	}
	for i, endpoint := range c.endpoints {
		if c.endpointCanStartConnection(endpoint) {
			return i
		}
	}
	return 0
}

func (c ServerConnection) endpointCanStartConnection(endpoint opcua.Endpoint) bool {
	if c.secureEndpointRequiresCertificate(endpoint) {
		return false
	}
	for _, authType := range serverConnectionAuthTypes(endpoint) {
		auth := opcua.AuthType(authType)
		if auth == opcua.AuthAnonymous || auth == opcua.AuthUsername {
			return true
		}
	}
	return false
}

func serverConnectionSupportsAuth(endpoint opcua.Endpoint, auth opcua.AuthType) bool {
	for _, token := range endpoint.UserTokenTypes {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(token), "UserTokenType"), string(auth)) {
			return true
		}
	}
	return false
}

func (c *ServerConnection) connectRequest(endpoint opcua.Endpoint, authType opcua.AuthType) opcua.ConnectRequest {
	return opcua.ConnectRequest{
		Endpoint:              strings.TrimSpace(c.endpointText),
		SecurityPolicy:        endpoint.SecurityPolicy,
		SecurityMode:          endpoint.SecurityMode,
		AuthType:              authType,
		ClientCertificatePath: c.clientCertificatePath,
		ClientPrivateKeyPath:  c.clientPrivateKeyPath,
	}
}

func (c *ServerConnection) secureEndpointRequiresCertificate(endpoint opcua.Endpoint) bool {
	securityMode := serverConnectionCompactSecurityMode(endpoint.SecurityMode)
	return securityMode != "None" && (c.clientCertificatePath == "" || c.clientPrivateKeyPath == "")
}

func (c *ServerConnection) failMissingSecureEndpointCertificate() {
	c.connecting = false
	c.authTypes = nil
	c.selectedAuthType = 0
	c.lastError = "secure endpoint requires client certificate and private key"
	c.status = ServerConnectionFailed
}

func serverConnectionAuthTypes(endpoint opcua.Endpoint) []string {
	types := make([]string, 0, len(endpoint.UserTokenTypes))
	for _, token := range endpoint.UserTokenTypes {
		normalized := strings.TrimPrefix(strings.TrimSpace(token), "UserTokenType")
		if normalized != "" {
			types = append(types, normalized)
		}
	}
	return types
}

func serverConnectionCompactSecurityMode(mode string) string {
	mode = strings.TrimPrefix(strings.TrimSpace(mode), "MessageSecurityMode")
	if mode == "" {
		return "Unknown"
	}
	return mode
}
