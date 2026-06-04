package opcua

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	gopcua "github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
)

// Client is the app-level OPC UA boundary consumed by the TUI.
// Keep this interface small; concrete protocol details belong behind it.
type Client interface {
	DiscoverEndpoints(ctx context.Context, endpoint string) ([]Endpoint, error)
	Connect(ctx context.Context, request ConnectRequest) error
	BrowseChildren(ctx context.Context, nodeID string) ([]AddressNode, error)
	SubscribeValue(ctx context.Context, nodeID string) (<-chan LiveValue, ValueSubscription, error)
	Close(ctx context.Context) error
}

// ValueSubscription is an active monitored item subscription.
type ValueSubscription interface {
	Cancel(ctx context.Context) error
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
	AuthType       AuthType
	Username       string
	Password       string
}

// AddressNode is the app-level projection of a browsed Address Space node.
type AddressNode struct {
	NodeID      string
	DisplayName string
	BrowseName  string
	NodeClass   string
}

// LiveValue is the app-level projection of a subscribed Variable Node value.
type LiveValue struct {
	NodeID          string
	Value           string
	Status          string
	SourceTimestamp time.Time
	ServerTimestamp time.Time
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
	client        *gopcua.Client
	subscriptions []ValueSubscription
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
	// Some OPC UA Servers advertise an endpoint URL that is reachable from the
	// server host but not from this client environment (for example VHS running
	// inside Docker). Preserve the selected security/auth metadata while dialing
	// the endpoint the Automation Engineer supplied.
	ep.EndpointURL = request.Endpoint

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

func (c *gopcuaClient) BrowseChildren(ctx context.Context, nodeID string) ([]AddressNode, error) {
	if c.client == nil {
		return nil, ua.StatusBadServerNotConnected
	}
	idToBrowse, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return nil, err
	}

	log.Printf("opcua: BrowseChildren request nodeID=%s", nodeID)
	refs, err := c.client.Node(idToBrowse).References(ctx, id.HierarchicalReferences, ua.BrowseDirectionForward, ua.NodeClassAll, true)
	if err != nil {
		log.Printf("opcua: BrowseChildren failed nodeID=%s error=%v", nodeID, err)
		return nil, err
	}

	nodes := make([]AddressNode, 0, len(refs))
	for _, ref := range refs {
		nodes = append(nodes, addressNodeFromReference(ref))
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		return strings.ToLower(nodes[i].DisplayName) < strings.ToLower(nodes[j].DisplayName)
	})
	log.Printf("opcua: BrowseChildren response nodeID=%s count=%d", nodeID, len(nodes))
	return nodes, nil
}

func (c *gopcuaClient) SubscribeValue(ctx context.Context, nodeID string) (<-chan LiveValue, ValueSubscription, error) {
	if c.client == nil {
		return nil, nil, ua.StatusBadServerNotConnected
	}
	parsedNodeID, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return nil, nil, err
	}

	notifyCh := make(chan *gopcua.PublishNotificationData, 8)
	sub, err := c.client.Subscribe(ctx, &gopcua.SubscriptionParameters{Interval: opcuaSubscriptionInterval}, notifyCh)
	if err != nil {
		return nil, nil, err
	}

	request := gopcua.NewMonitoredItemCreateRequestWithDefaults(parsedNodeID, ua.AttributeIDValue, uint32(len(c.subscriptions)+1))
	res, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, request)
	if err != nil {
		_ = sub.Cancel(ctx)
		return nil, nil, err
	}
	if len(res.Results) == 0 || res.Results[0].StatusCode != ua.StatusOK {
		_ = sub.Cancel(ctx)
		if len(res.Results) == 0 {
			return nil, nil, fmt.Errorf("monitor %s failed: no result", nodeID)
		}
		return nil, nil, fmt.Errorf("monitor %s failed: %s", nodeID, res.Results[0].StatusCode)
	}

	values := make(chan LiveValue, 8)
	go forwardValueNotifications(nodeID, notifyCh, values)
	c.subscriptions = append(c.subscriptions, sub)
	log.Printf("opcua: SubscribeValue succeeded nodeID=%s subscriptionID=%d", nodeID, sub.SubscriptionID)
	return values, sub, nil
}

func (c *gopcuaClient) Close(ctx context.Context) error {
	for _, sub := range c.subscriptions {
		_ = sub.Cancel(ctx)
	}
	c.subscriptions = nil
	if c.client == nil {
		return nil
	}
	err := c.client.Close(ctx)
	c.client = nil
	return err
}

func forwardValueNotifications(nodeID string, notifyCh <-chan *gopcua.PublishNotificationData, values chan<- LiveValue) {
	defer close(values)
	for notification := range notifyCh {
		if notification == nil || notification.Error != nil {
			continue
		}
		dataChange, ok := notification.Value.(*ua.DataChangeNotification)
		if !ok {
			continue
		}
		for _, item := range dataChange.MonitoredItems {
			if item == nil || item.Value == nil {
				continue
			}
			values <- liveValueFromDataValue(nodeID, item.Value)
		}
	}
}

func liveValueFromDataValue(nodeID string, data *ua.DataValue) LiveValue {
	value := "<nil>"
	status := "Unknown"
	var sourceTimestamp time.Time
	var serverTimestamp time.Time
	if data != nil {
		if data.Value != nil {
			value = fmt.Sprint(data.Value.Value())
		}
		status = fmt.Sprint(data.Status)
		sourceTimestamp = data.SourceTimestamp
		serverTimestamp = data.ServerTimestamp
	}
	return LiveValue{NodeID: nodeID, Value: value, Status: status, SourceTimestamp: sourceTimestamp, ServerTimestamp: serverTimestamp}
}

var opcuaSubscriptionInterval = 1 * time.Second

func addressNodeFromReference(ref *ua.ReferenceDescription) AddressNode {
	nodeID := ""
	if ref.NodeID != nil {
		nodeID = ua.NewNodeIDFromExpandedNodeID(ref.NodeID).String()
	}

	displayName := "(unnamed)"
	if ref.DisplayName != nil && ref.DisplayName.Text != "" {
		displayName = ref.DisplayName.Text
	}

	browseName := ""
	if ref.BrowseName != nil {
		browseName = ref.BrowseName.Name
		if ref.BrowseName.NamespaceIndex != 0 {
			browseName = fmt.Sprintf("%d:%s", ref.BrowseName.NamespaceIndex, ref.BrowseName.Name)
		}
	}

	return AddressNode{
		NodeID:      nodeID,
		DisplayName: displayName,
		BrowseName:  browseName,
		NodeClass:   nodeClassName(ref.NodeClass.String()),
	}
}

func nodeClassName(value string) string {
	return strings.TrimPrefix(value, "NodeClass")
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
