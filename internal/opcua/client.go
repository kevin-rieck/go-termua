package opcua

import (
	"context"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"os"
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
	ReadNodeDetails(ctx context.Context, nodeID string) (NodeDetails, error)
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
	Endpoint              string
	ConnectionName        string
	SecurityPolicy        string
	SecurityMode          string
	AuthType              AuthType
	Username              string
	Password              string
	ClientCertificatePath string
	ClientPrivateKeyPath  string
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

type ValueRange struct {
	Low  float64
	High float64
}

type NodeDetails struct {
	NodeID          string
	Description     string
	DataType        string
	AccessLevel     string
	Writable        bool
	ValueRank       string
	ArrayDimensions string
	EngineeringUnit string
	EURange         *ValueRange
	InstrumentRange *ValueRange
}

type AuthType string

const (
	AuthAnonymous   AuthType = "Anonymous"
	AuthUsername    AuthType = "UserName"
	AuthCertificate AuthType = "Certificate"
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
	if request.AuthType != AuthAnonymous && request.AuthType != AuthUsername && request.AuthType != AuthCertificate {
		return fmt.Errorf("unsupported authentication mode: %s", request.AuthType)
	}
	if connectRequestRequiresCertificateFiles(request) {
		if request.ClientCertificatePath == "" || request.ClientPrivateKeyPath == "" {
			return fmt.Errorf("certificate connection requires client certificate and private key")
		}
		if err := validateSecureEndpointCertificateFiles(request.ClientCertificatePath, request.ClientPrivateKeyPath); err != nil {
			return err
		}
	}
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

	opts := clientOptionsForConnectRequest(ep, request)

	client, err := gopcua.NewClient(ep.EndpointURL, opts...)
	if err != nil {
		log.Printf("opcua: NewClient failed endpointURL=%s error=%v", ep.EndpointURL, err)
		return err
	}
	if err := client.Connect(ctx); err != nil {
		log.Printf("opcua: Connect failed endpointURL=%s error=%v", ep.EndpointURL, err)
		return secureConnectError(request, err)
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

func (c *gopcuaClient) ReadNodeDetails(ctx context.Context, nodeID string) (NodeDetails, error) {
	if c.client == nil {
		return NodeDetails{}, ua.StatusBadServerNotConnected
	}
	parsedNodeID, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return NodeDetails{}, err
	}

	node := c.client.Node(parsedNodeID)
	details := NodeDetails{NodeID: nodeID}
	attrs, err := node.Attributes(ctx,
		ua.AttributeIDDescription,
		ua.AttributeIDAccessLevel,
		ua.AttributeIDDataType,
		ua.AttributeIDValueRank,
		ua.AttributeIDArrayDimensions,
	)
	if err != nil {
		return NodeDetails{}, err
	}
	applyNodeDetailAttributes(&details, attrs)
	if err := c.applyNodeDetailProperties(ctx, node, &details); err != nil {
		log.Printf("opcua: ReadNodeDetails properties failed nodeID=%s error=%v", nodeID, err)
	}
	return details, nil
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

func applyNodeDetailAttributes(details *NodeDetails, attrs []*ua.DataValue) {
	if len(attrs) < 5 {
		return
	}
	if attrs[0] != nil && attrs[0].Status == ua.StatusOK && attrs[0].Value != nil {
		details.Description = localizedTextValue(attrs[0].Value.Value())
	}
	if attrs[1] != nil && attrs[1].Status == ua.StatusOK && attrs[1].Value != nil {
		access := ua.AccessLevelType(attrs[1].Value.Int())
		details.AccessLevel = accessLevelText(access)
		details.Writable = access&ua.AccessLevelTypeCurrentWrite == ua.AccessLevelTypeCurrentWrite
	}
	if attrs[2] != nil && attrs[2].Status == ua.StatusOK && attrs[2].Value != nil {
		details.DataType = dataTypeName(attrs[2].Value.NodeID())
	}
	if attrs[3] != nil && attrs[3].Status == ua.StatusOK && attrs[3].Value != nil {
		details.ValueRank = valueRankText(int32(attrs[3].Value.Int()))
	}
	if attrs[4] != nil && attrs[4].Status == ua.StatusOK && attrs[4].Value != nil {
		details.ArrayDimensions = fmt.Sprint(attrs[4].Value.Value())
	}
}

func (c *gopcuaClient) applyNodeDetailProperties(ctx context.Context, node *gopcua.Node, details *NodeDetails) error {
	refs, err := node.References(ctx, id.HasProperty, ua.BrowseDirectionForward, ua.NodeClassVariable, true)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		property := addressNodeFromReference(ref)
		propertyNodeID, err := ua.ParseNodeID(property.NodeID)
		if err != nil {
			continue
		}
		value, err := c.client.Node(propertyNodeID).Attribute(ctx, ua.AttributeIDValue)
		if err != nil || value == nil {
			continue
		}
		switch strings.TrimPrefix(property.BrowseName, "0:") {
		case "EngineeringUnits":
			details.EngineeringUnit = engineeringUnitText(value.Value())
		case "EURange":
			details.EURange = rangeValue(value.Value())
		case "InstrumentRange":
			details.InstrumentRange = rangeValue(value.Value())
		}
	}
	return nil
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

func localizedTextValue(value any) string {
	switch v := value.(type) {
	case *ua.LocalizedText:
		if v == nil {
			return ""
		}
		return v.Text
	case ua.LocalizedText:
		return v.Text
	default:
		return fmt.Sprint(value)
	}
}

func accessLevelText(access ua.AccessLevelType) string {
	var parts []string
	if access&ua.AccessLevelTypeCurrentRead == ua.AccessLevelTypeCurrentRead {
		parts = append(parts, "CurrentRead")
	}
	if access&ua.AccessLevelTypeCurrentWrite == ua.AccessLevelTypeCurrentWrite {
		parts = append(parts, "CurrentWrite")
	}
	if len(parts) == 0 {
		return "None"
	}
	return strings.Join(parts, ", ")
}

func valueRankText(rank int32) string {
	switch rank {
	case -3:
		return "Scalar or one-dimensional array"
	case -2:
		return "Any"
	case -1:
		return "Scalar"
	case 0:
		return "One or more dimensions"
	case 1:
		return "One-dimensional array"
	default:
		return fmt.Sprintf("%d dimensions", rank)
	}
}

func dataTypeName(nodeID *ua.NodeID) string {
	if nodeID == nil {
		return ""
	}
	if nodeID.Namespace() != 0 {
		return nodeID.String()
	}
	switch nodeID.IntID() {
	case id.Boolean:
		return "Boolean"
	case id.SByte:
		return "SByte"
	case id.Byte:
		return "Byte"
	case id.Int16:
		return "Int16"
	case id.UInt16:
		return "UInt16"
	case id.Int32:
		return "Int32"
	case id.UInt32:
		return "UInt32"
	case id.Int64:
		return "Int64"
	case id.UInt64:
		return "UInt64"
	case id.Float:
		return "Float"
	case id.Double:
		return "Double"
	case id.String:
		return "String"
	case id.DateTime, id.UtcTime:
		return "DateTime"
	default:
		return nodeID.String()
	}
}

func engineeringUnitText(value any) string {
	switch v := value.(type) {
	case *ua.EUInformation:
		if v == nil {
			return ""
		}
		if v.DisplayName != nil && v.DisplayName.Text != "" {
			return v.DisplayName.Text
		}
		return fmt.Sprintf("unit %d", v.UnitID)
	case ua.EUInformation:
		if v.DisplayName != nil && v.DisplayName.Text != "" {
			return v.DisplayName.Text
		}
		return fmt.Sprintf("unit %d", v.UnitID)
	default:
		return fmt.Sprint(value)
	}
}

func rangeValue(value any) *ValueRange {
	switch v := value.(type) {
	case *ua.Range:
		if v == nil {
			return nil
		}
		return &ValueRange{Low: v.Low, High: v.High}
	case ua.Range:
		return &ValueRange{Low: v.Low, High: v.High}
	default:
		return nil
	}
}

func clientOptionsForConnectRequest(ep *ua.EndpointDescription, request ConnectRequest) []gopcua.Option {
	authType := ua.UserTokenTypeAnonymous
	authOptions := []gopcua.Option{gopcua.AuthAnonymous()}
	switch request.AuthType {
	case AuthUsername:
		authType = ua.UserTokenTypeUserName
		authOptions = []gopcua.Option{gopcua.AuthUsername(request.Username, request.Password)}
	case AuthCertificate:
		authType = ua.UserTokenTypeCertificate
		cert, _ := loadCertificateDER(request.ClientCertificatePath)
		key, _ := loadPrivateKey(request.ClientPrivateKeyPath)
		authOptions = []gopcua.Option{gopcua.AuthCertificate(cert), gopcua.AuthPrivateKey(key)}
	}

	opts := []gopcua.Option{
		gopcua.ApplicationName("TermUA"),
		gopcua.ProductURI("urn:termua"),
	}
	if compactMessageSecurityMode(request.SecurityMode) != "None" {
		opts = append(opts,
			gopcua.CertificateFile(request.ClientCertificatePath),
			gopcua.PrivateKeyFile(request.ClientPrivateKeyPath),
		)
	}
	opts = append(opts, gopcua.SecurityFromEndpoint(ep, authType))
	opts = append(opts, authOptions...)
	return opts
}

func connectRequestRequiresCertificateFiles(request ConnectRequest) bool {
	return compactMessageSecurityMode(request.SecurityMode) != "None" || request.AuthType == AuthCertificate
}

func compactMessageSecurityMode(mode string) string {
	mode = strings.TrimPrefix(strings.TrimSpace(mode), "MessageSecurityMode")
	if mode == "" {
		return "Unknown"
	}
	return mode
}

func secureConnectError(request ConnectRequest, err error) error {
	if request.AuthType == AuthCertificate && errorsIndicateCertificateAuthRejected(err) {
		return fmt.Errorf("certificate authentication rejected by server; trust the TermUA client certificate as a user certificate in the OPC UA Server and retry: %s", request.ClientCertificatePath)
	}
	if compactMessageSecurityMode(request.SecurityMode) == "None" {
		return err
	}
	if errorsIsEOF(err) {
		return fmt.Errorf("secure channel closed by server; trust the TermUA client application certificate in the OPC UA Server and retry: %s", request.ClientCertificatePath)
	}
	return err
}

func errorsIndicateCertificateAuthRejected(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "identitytoken") || strings.Contains(text, "identity token") || strings.Contains(text, "certificate")
}

func errorsIsEOF(err error) bool {
	return err == io.EOF || strings.TrimSpace(err.Error()) == "EOF"
}

func validateSecureEndpointCertificateFiles(certificatePath, privateKeyPath string) error {
	if err := validateCertificateFile(certificatePath); err != nil {
		return err
	}
	if err := validatePrivateKeyFile(privateKeyPath); err != nil {
		return err
	}
	return nil
}

func validateCertificateFile(path string) error {
	if _, err := loadCertificateDER(path); err != nil {
		return err
	}
	return nil
}

func loadCertificateDER(path string) ([]byte, error) {
	data, err := readRegularFile("client certificate file", path)
	if err != nil {
		return nil, err
	}
	der := data
	if strings.HasSuffix(path, ".pem") {
		block, _ := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("client certificate file is not a PEM certificate: %s", path)
		}
		der = block.Bytes
	}
	if _, err := x509.ParseCertificate(der); err != nil {
		return nil, fmt.Errorf("client certificate file is invalid: %s: %w", path, err)
	}
	return der, nil
}

func validatePrivateKeyFile(path string) error {
	if _, err := loadPrivateKey(path); err != nil {
		return err
	}
	return nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := readRegularFile("client private key file", path)
	if err != nil {
		return nil, err
	}
	der := data
	if strings.HasSuffix(path, ".pem") {
		block, _ := pem.Decode(data)
		if block == nil || block.Type != "RSA PRIVATE KEY" {
			return nil, fmt.Errorf("client private key file is not a PEM RSA private key: %s", path)
		}
		der = block.Bytes
	}
	key, err := x509.ParsePKCS1PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("client private key file is invalid: %s: %w", path, err)
	}
	return key, nil
}

func readRegularFile(label, path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s not found: %s", label, path)
		}
		return nil, fmt.Errorf("%s cannot be accessed: %s: %w", label, path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory: %s", label, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s cannot be read: %s: %w", label, path, err)
	}
	return data, nil
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
