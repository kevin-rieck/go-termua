package opcua

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"log"
	"strings"

	gopcua "github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

// Client is the app-level OPC UA boundary consumed by the TUI.
// Keep this interface small; concrete protocol details belong behind it.
type Client interface {
	DiscoverEndpoints(ctx context.Context, endpoint string) ([]Endpoint, error)
	Connect(ctx context.Context, request ConnectRequest) error
	Close(ctx context.Context) error
}

// Endpoint is the app-level projection of an OPC UA endpoint description.
type Endpoint struct {
	URL              string
	SecurityPolicy   string
	SecurityMode     string
	SecurityLevel    uint8
	UserTokenTypes   []string
	ServerThumbprint string
}

type ConnectRequest struct {
	Endpoint       string
	ConnectionName string
	SecurityPolicy string
	SecurityMode   string
	AuthType        AuthType
	Username        string
	Password        string
}

type AuthType string

const (
	AuthAnonymous AuthType = "Anonymous"
	AuthUsername  AuthType = "UserName"
)

// NewClient returns the production OPC UA client service.
func NewClient() Client {
	return &gopcuaClient{}
}

type gopcuaClient struct {
	client *gopcua.Client
}

func (c *gopcuaClient) DiscoverEndpoints(ctx context.Context, endpoint string) ([]Endpoint, error) {
	log.Printf("opcua: GetEndpoints request endpoint=%s", endpoint)
	endpoints, err := gopcua.GetEndpoints(ctx, endpoint)
	if err != nil {
		log.Printf("opcua: GetEndpoints failed endpoint=%s error=%v", endpoint, err)
		return nil, err
	}
	log.Printf("opcua: GetEndpoints response endpoint=%s count=%d", endpoint, len(endpoints))

	result := make([]Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		result = append(result, endpointFromDescription(ep))
	}
	return result, nil
}

func (c *gopcuaClient) Connect(ctx context.Context, request ConnectRequest) error {
	log.Printf("opcua: Connect request endpoint=%s securityPolicy=%s securityMode=%s authType=%s", request.Endpoint, request.SecurityPolicy, request.SecurityMode, request.AuthType)
	securityMode := ua.MessageSecurityModeFromString(request.SecurityMode)
	endpoints, err := gopcua.GetEndpoints(ctx, request.Endpoint)
	if err != nil {
		log.Printf("opcua: Connect endpoint discovery failed endpoint=%s error=%v", request.Endpoint, err)
		return err
	}
	ep, err := gopcua.SelectEndpoint(endpoints, request.SecurityPolicy, securityMode)
	if err != nil {
		log.Printf("opcua: SelectEndpoint failed endpoint=%s securityPolicy=%s securityMode=%s error=%v", request.Endpoint, request.SecurityPolicy, request.SecurityMode, err)
		return err
	}

	authType := ua.UserTokenTypeAnonymous
	opts := []gopcua.Option{
		gopcua.ApplicationName("TermUA"),
		gopcua.ProductURI("urn:termua"),
		gopcua.SecurityFromEndpoint(ep, authType),
		gopcua.AuthAnonymous(),
	}

	if request.AuthType == AuthUsername {
		authType = ua.UserTokenTypeUserName
		opts = []gopcua.Option{
			gopcua.ApplicationName("TermUA"),
			gopcua.ProductURI("urn:termua"),
			gopcua.SecurityFromEndpoint(ep, authType),
			gopcua.AuthUsername(request.Username, request.Password),
		}
	}

	client, err := gopcua.NewClient(ep.EndpointURL, opts...)
	if err != nil {
		log.Printf("opcua: NewClient failed endpointURL=%s error=%v", ep.EndpointURL, err)
		return err
	}
	if err := client.Connect(ctx); err != nil {
		log.Printf("opcua: Connect failed endpointURL=%s error=%v", ep.EndpointURL, err)
		return err
	}
	log.Printf("opcua: Connect succeeded endpointURL=%s", ep.EndpointURL)

	if c.client != nil {
		_ = c.client.Close(ctx)
	}
	c.client = client
	return nil
}

func (c *gopcuaClient) Close(ctx context.Context) error {
	if c.client == nil {
		return nil
	}
	err := c.client.Close(ctx)
	c.client = nil
	return err
}

func endpointFromDescription(ep *ua.EndpointDescription) Endpoint {
	tokens := make([]string, 0, len(ep.UserIdentityTokens))
	for _, token := range ep.UserIdentityTokens {
		tokens = append(tokens, token.TokenType.String())
	}

	return Endpoint{
		URL:              ep.EndpointURL,
		SecurityPolicy:   securityPolicyName(ep.SecurityPolicyURI),
		SecurityMode:     ep.SecurityMode.String(),
		SecurityLevel:    ep.SecurityLevel,
		UserTokenTypes:   tokens,
		ServerThumbprint: certificateThumbprint(ep.ServerCertificate),
	}
}

func securityPolicyName(uri string) string {
	if uri == "" {
		return "None"
	}
	parts := strings.Split(uri, "#")
	return parts[len(parts)-1]
}

func certificateThumbprint(cert []byte) string {
	if len(cert) == 0 {
		return ""
	}
	sum := sha1.Sum(cert)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
