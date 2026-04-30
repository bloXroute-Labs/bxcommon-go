package sdnsdk

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net"
	"net/http"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bloXroute-Labs/bxcommon-go/cert"
	log "github.com/bloXroute-Labs/bxcommon-go/logger"
	"github.com/bloXroute-Labs/bxcommon-go/sdnsdk/message"
	"github.com/bloXroute-Labs/bxcommon-go/types"
)

var (
	// ErrSDNUnavailable - represents SDN service unavailable
	ErrSDNUnavailable = errors.New("SDN service unavailable")
	// ErrNoRelays - sdn did not find any relays error
	ErrNoRelays = errors.New("no relays were acquired from SDN")
)

// SDN Http type constants
const (
	PingTimeout                     = 2000.0
	TimeRegEx                       = "= ([^/]*)"
	blockchainNetworksCacheFileName = "blockchainNetworks.json"
	blockchainNetworkCacheFileName  = "blockchainNetwork.json"
	nodeModelCacheFileName          = "nodemodel.json"
	potentialRelaysFileName         = "potentialrelays.json"
	accountModelsFileName           = "accountmodel.json"
	httpTimeout                     = 10 * time.Second
	latencyThreshold                = 10
	privateCertRenewalPeriodDays    = 180
)

// SDNHTTP is the interface for realSDNHTTP type
type SDNHTTP interface {
	SDNURL() string
	NodeID() types.NodeID
	Networks() *message.BlockchainNetworks
	SetNetworks(networks message.BlockchainNetworks)
	FetchAllBlockchainNetworks() error
	FetchBlockchainNetwork() error
	InitGateway(protocol string, network string) error
	NodeModel() *message.NodeModel
	AccountTier() message.AccountTier
	AccountModel() message.Account
	NetworkNum() types.NetworkNum
	Register() error
	NeedsRegistration() bool
	FetchCustomerAccountModel(accountID types.AccountID) (message.Account, error)
	DirectRelayConnections(relayHosts string, relayLimit uint64, relayInstructions chan<- RelayInstruction, ignoredRelays IgnoredRelaysMap) error
	FindNetwork(networkNum types.NetworkNum) (*message.BlockchainNetwork, error)
	MinTxAge() time.Duration
	SendNodeEvent(event message.NodeEvent, id types.NodeID)
	Get(endpoint string, requestBody []byte) ([]byte, error)
	GetQuotaUsage(accountID string) (*QuotaResponseBody, error)
	FindNewRelay(ctx context.Context, oldRelayIP string, oldRelayIPPort int64, relayInstructions chan RelayInstruction, ignoredRelays IgnoredRelaysMap)
	FindFastestRelays(relayInstructions chan<- RelayInstruction, ignoredRelays IgnoredRelaysMap)
	SetInternalGateway(state *message.InternalGateway) error
	AddInternalGatewaySubscription(accountID types.AccountID) error
	RemoveInternalGatewaySubscription(accountID types.AccountID) error
	RotateCertificate(ctx context.Context) error
}

// realSDNHTTP is a connection to the bloxroute API
type realSDNHTTP struct {
	sslCerts         *cert.SSLCerts
	getPingLatencies func(peers message.Peers) []nodeLatencyInfo
	networks         message.BlockchainNetworks
	accountModel     *message.Account
	nodeID           types.NodeID
	accountID        types.AccountID
	sdnURL           string
	dataDir          string
	nodeModel        *message.NodeModel
	relays           message.Peers
	nodeLock         sync.RWMutex
}

// relayMap maps a relay's IP to its port
type relayMap map[string]int64

// IgnoredRelaysMap sync map for ignored relays
type IgnoredRelaysMap interface {
	Load(key string) (value types.RelayInfo, ok bool)
	Store(key string, value types.RelayInfo)
	Delete(key string)
	Range(f func(key string, value types.RelayInfo) bool)
	LoadOrStore(key string, val types.RelayInfo) (actual types.RelayInfo, loaded bool)
}

// nodeLatencyInfo contains ping results with host and latency info
type nodeLatencyInfo struct {
	IP      string
	Port    int64
	Latency float64
}

// RelayInstruction specifies whether to connect or disconnect to the relay at an IP:Port
type RelayInstruction struct {
	IP             string
	Type           ConnInstructionType
	Port           int64
	IsStatic       bool
	RelaysToSwitch []nodeLatencyInfo
}

// ConnInstructionType specifies connection or disconnection
type ConnInstructionType int

type QuotaRequestBody struct {
	AccountID string `json:"account_id"`
}

// RotateCertificateRequestBody is the request body for rotating a registration-only certificate
type RotateCertificateRequestBody struct {
	AccountID types.AccountID `json:"account_id"`
	CSR       string          `json:"csr"`
}

// NewCertificateResponse is the response body for rotating a registration-only certificate
type NewCertificateResponse struct {
	Certificate string `json:"certificate"`
}

// QuotaResponseBody quota usage response body
type QuotaResponseBody struct {
	AccountID   string `json:"account_id"`
	QuotaFilled int    `json:"quota_filled"`
	QuotaLimit  int    `json:"quota_limit"`
}

type relayToSwitch struct {
	ip   string
	port int64
}

const (
	// Connect is the instruction to connect to a relay
	Connect ConnInstructionType = iota
	// Disconnect is the instruction to disconnect from a relay
	Disconnect
	// Switch is the instruction to switch relay
	Switch
)

// NewSDNHTTP creates a new connection to the bloxroute API
func NewSDNHTTP(sslCerts *cert.SSLCerts, sdnURL string, nodeModel message.NodeModel, dataDir string) SDNHTTP {
	if nodeModel.ExternalIP == "" {
		var err error
		nodeModel.ExternalIP, err = IPResolverHolder.GetPublicIP()
		if err != nil {
			log.Fatalf("could not determine node's public ip: %v. consider specifying an --external-ip address", err)
		}
		if nodeModel.ExternalIP == "" {
			log.Fatal("could not determine node's public ip. consider specifying an --external-ip address")
		}
		log.Infof("no external ip address was provided, using autodiscovered ip address %v", nodeModel.ExternalIP)
	}
	sdn := &realSDNHTTP{
		sslCerts:         sslCerts,
		sdnURL:           sdnURL,
		nodeModel:        &nodeModel,
		getPingLatencies: getPingLatencies,
		dataDir:          dataDir,
		nodeLock:         sync.RWMutex{},
	}
	return sdn
}

// FetchAllBlockchainNetworks fetches list of blockchain networks from the SDN
func (s *realSDNHTTP) FetchAllBlockchainNetworks() error {
	err := s.getBlockchainNetworks()
	if err != nil {
		return err
	}
	return nil
}

// Get is a generic function for sending GET request to SDNHttp
func (s *realSDNHTTP) Get(endpoint string, requestBody []byte) ([]byte, error) {
	url := s.sdnURL + endpoint
	proxyReq, err := http.NewRequest(http.MethodGet, url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	c, err := s.httpClient()
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(proxyReq)
	if err != nil {
		return nil, err
	}

	defer s.close(resp)

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return respBytes, nil
}

// FetchBlockchainNetwork fetches a blockchain network given the blockchain number of the model registered with SDN
func (s *realSDNHTTP) FetchBlockchainNetwork() error {
	networkNum := s.NetworkNum()
	url := fmt.Sprintf("%v/blockchain-networks/%v", s.sdnURL, networkNum)
	resp, err := s.httpWithCache(url, http.MethodGet, blockchainNetworkCacheFileName, nil)
	if err != nil {
		return err
	}
	prev, ok := s.networks[networkNum]
	if !ok {
		s.networks[networkNum] = new(message.BlockchainNetwork)
	}
	if err = json.Unmarshal(resp, s.networks[networkNum]); err != nil {
		return fmt.Errorf("could not deserialize '%s' response into blockchain network (previously cached as: %v) for networkNum %v: %v", string(resp), prev, networkNum, err)
	}
	if prev != nil && s.networks[networkNum].MinTxAgeSeconds != prev.MinTxAgeSeconds {
		log.Debugf("MinTxAgeSeconds changed from %v seconds to %v seconds after the update", prev.MinTxAgeSeconds, s.networks[networkNum].MinTxAgeSeconds)
	}
	if s.networks[networkNum].Protocol == types.EthereumProtocol && s.networks[networkNum].DefaultAttributes.TerminalTotalDifficulty == 0 {
		s.networks[networkNum].DefaultAttributes.TerminalTotalDifficulty = big.NewInt(math.MaxInt)
	}

	return nil
}

// InitGateway fetches all necessary information over HTTP from the SDN
func (s *realSDNHTTP) InitGateway(protocol string, network string) error {
	s.nodeLock.Lock()
	s.nodeModel.Protocol = protocol
	s.nodeModel.Network = network
	s.nodeLock.Unlock()

	s.networks = make(message.BlockchainNetworks)

	if err := s.Register(); err != nil {
		return err
	}
	if err := s.FetchBlockchainNetwork(); err != nil {
		return err
	}

	s.nodeLock.RLock()
	accountID := s.nodeModel.AccountID
	s.nodeLock.RUnlock()

	return s.getAccountModel(accountID)
}

// RotateCertificate checks if the private certificate is expiring within the renewal period and rotates it if needed
func (s *realSDNHTTP) RotateCertificate(ctx context.Context) error {
	expDate, err := s.sslCerts.RegistrationOnlyCertExpirationDate()
	if err != nil {
		return fmt.Errorf("could not get private certificate expiration date: %w", err)
	}

	if time.Until(expDate) > privateCertRenewalPeriodDays*24*time.Hour {
		return nil
	}

	log.Infof("private registration-only certificate expiring on %v, rotating", expDate)

	csr, err := s.sslCerts.CreateRegistrationCSR()
	if err != nil {
		return fmt.Errorf("could not create csr for new private certificate: %w", err)
	}

	req := RotateCertificateRequestBody{
		CSR:       string(csr),
		AccountID: s.nodeModel.AccountID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("could not serialize request body: %w", err)
	}

	resp, err := s.httpWithContext(ctx, s.sdnURL+"/accounts/rotate-cert", http.MethodPatch, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	var newCert NewCertificateResponse

	if err = json.Unmarshal(resp, &newCert); err != nil {
		return fmt.Errorf("could not deserialize '%s' response into node model: %w", string(resp), err)
	}

	err = s.sslCerts.SaveRegistrationOnlyCert(newCert.Certificate)
	if err != nil {
		return fmt.Errorf("could not save new private certificate: %w", err)
	}

	return nil
}

func logLowestLatency(lowestLatencyRelay nodeLatencyInfo) {
	if lowestLatencyRelay.Latency > 40 {
		log.Warnf("ping latency of the fastest relay %v:%v is %v ms, which is more than 40 ms",
			lowestLatencyRelay.IP, lowestLatencyRelay.Port, lowestLatencyRelay.Latency)
	}
	log.Infof("fastest selected relay %v:%v has a latency of %v ms",
		lowestLatencyRelay.IP, lowestLatencyRelay.Port, lowestLatencyRelay.Latency)
}

// DirectRelayConnections directs the gateway on relays to connect/disconnect
func (s *realSDNHTTP) DirectRelayConnections(relayHosts string, relayLimit uint64, relayInstructions chan<- RelayInstruction, ignoredRelays IgnoredRelaysMap) error {
	overrideRelays, autoCount, err := parsedCmdlineRelays(relayHosts, relayLimit)
	if err != nil {
		return err
	}

	// connect relays specified in `relays` argument
	for ip, port := range overrideRelays {
		ignoredRelays.Store(ip, types.RelayInfo{TimeAdded: time.Now(), IsConnected: true, IsStatic: true, Port: port})
		relayInstructions <- RelayInstruction{IP: ip, Port: port, Type: Connect, IsStatic: true}
	}

	if autoCount == 0 {
		return nil
	}

	s.nodeLock.RLock()
	networkNum := s.nodeModel.BlockchainNetworkNum
	nodeID := s.nodeModel.NodeID
	s.nodeLock.RUnlock()

	// TODO: fetching relay from SDN should be done in a loop inside manageAutoRelays
	// if auto relays specified, start and manage them
	relays, err := s.getRelays(nodeID, networkNum)
	if err != nil {
		return fmt.Errorf("failed to extract relay list: %v", err)
	}
	if len(relays) == 0 {
		return ErrNoRelays
	}
	go s.manageAutoRelays(autoCount, relayInstructions, relays, ignoredRelays)
	return nil
}

func (s *realSDNHTTP) connectToNewRelay(relayInstructions chan<- RelayInstruction, ignoredRelays IgnoredRelaysMap) error {
	s.nodeLock.RLock()
	nodeID := s.nodeModel.NodeID
	networkNum := s.nodeModel.BlockchainNetworkNum
	s.nodeLock.RUnlock()

	relays, err := s.getRelays(nodeID, networkNum)
	if err != nil {
		return fmt.Errorf("failed to extract relay list: %v", err)
	}
	if len(relays) == 0 {
		return ErrNoRelays
	}
	s.manageAutoRelays(1, relayInstructions, relays, ignoredRelays)
	return nil
}

// parsedCmdlineRelays parses the relayHosts argument and returns relays IPs up to the relay limit
func parsedCmdlineRelays(relayHosts string, relayLimit uint64) (relayMap, int, error) {
	overrideRelays := make(relayMap)
	autoCount := 0

	if len(relayHosts) == 0 {
		return nil, 0, fmt.Errorf("no --relays/relay-ip arguments were provided")
	}
	for _, relay := range strings.Split(relayHosts, ",") {
		// Clean and get the relay string
		if uint64(len(overrideRelays)+autoCount) == relayLimit { // Only counting unique relays + auto relays
			break
		}
		suggestedRelayString := strings.Trim(relay, " ")
		if suggestedRelayString == "auto" {
			autoCount++
			continue
		}
		if suggestedRelayString == "" {
			return nil, 0, fmt.Errorf("argument to --relays/relay-ip is empty or has an extra comma")
		}
		suggestedRelaySplit := strings.Split(suggestedRelayString, ":")
		if len(suggestedRelaySplit) > 2 {
			return nil, 0, fmt.Errorf("relay from --relays/relay-ip was given in the incorrect format '%s', should be IP:Port", relay)
		}

		host := suggestedRelaySplit[0]
		port := 1809
		var err error
		// Parse the relay string

		if len(suggestedRelaySplit) == 2 { // Make sure that port is an integer
			port, err = strconv.Atoi(suggestedRelaySplit[1])
			if err != nil {
				return nil, 0, fmt.Errorf("port provided %v is not valid - %v", suggestedRelaySplit[1], err)
			}
		}
		ip, err := GetIP(host)
		if err != nil {
			log.Errorf("relay %s from --relays/relay-ip is not valid - %v", suggestedRelaySplit[0], err)
			return nil, 0, err
		}
		if _, ok := overrideRelays[ip]; !ok {
			overrideRelays[ip] = int64(port)
		}
	}
	return overrideRelays, autoCount, nil
}

func (s *realSDNHTTP) getAutoConnectedRelays(ignoredRelays IgnoredRelaysMap) map[string]types.RelayInfo {
	connectedAutoRelays := make(map[string]types.RelayInfo)
	ignoredRelays.Range(func(key string, value types.RelayInfo) bool {
		if value.IsConnected && !value.IsStatic {
			connectedAutoRelays[key] = value
		}
		return true
	})
	return connectedAutoRelays
}

func (s *realSDNHTTP) findFastestAvailableRelays(pingLatencies []nodeLatencyInfo, connectedAutoRelays map[string]types.RelayInfo) []nodeLatencyInfo {
	fastestAvailableRelays := make([]nodeLatencyInfo, 0)

	for _, pingLatency := range pingLatencies {
		info, exists := connectedAutoRelays[pingLatency.IP]
		if exists {
			info.Latency = pingLatency.Latency
			connectedAutoRelays[pingLatency.IP] = info
			continue
		}
		fastestAvailableRelays = append(fastestAvailableRelays, pingLatency)
	}
	return fastestAvailableRelays
}

func (s *realSDNHTTP) findRelaysToSwitch(connectedAutoRelays map[string]types.RelayInfo, fastestAvailableRelays []nodeLatencyInfo) map[relayToSwitch][]nodeLatencyInfo {
	relaysToSwitch := make(map[relayToSwitch][]nodeLatencyInfo) // map[oldIP and Port][]newRelayNodeLatencyInfo

OuterLoop:
	for _, relay := range convertMapToSortedSlice(connectedAutoRelays) {
		for _, pingLatency := range fastestAvailableRelays {
			if relay.relayInfo.Latency < pingLatency.Latency+latencyThreshold {
				continue OuterLoop
			}
			relaysToSwitch[relayToSwitch{ip: relay.ip, port: relay.relayInfo.Port}] = append(relaysToSwitch[relayToSwitch{ip: relay.ip, port: relay.relayInfo.Port}], pingLatency)
		}
	}
	return relaysToSwitch
}

type autoRelay struct {
	ip        string
	relayInfo types.RelayInfo
}

func convertMapToSortedSlice(connectedAutoRelays map[string]types.RelayInfo) []autoRelay {
	relaySlice := make([]autoRelay, 0, len(connectedAutoRelays))
	for k, v := range connectedAutoRelays {
		relaySlice = append(relaySlice, autoRelay{k, v})
	}
	sort.Slice(relaySlice, func(i, j int) bool {
		return relaySlice[i].relayInfo.Latency > relaySlice[j].relayInfo.Latency
	})
	return relaySlice
}

func (s *realSDNHTTP) FindFastestRelays(relayInstructions chan<- RelayInstruction, ignoredRelays IgnoredRelaysMap) {
	s.nodeLock.RLock()
	nodeID := s.nodeModel.NodeID
	networkNum := s.nodeModel.BlockchainNetworkNum
	s.nodeLock.RUnlock()

	relays, err := s.getRelays(nodeID, networkNum)
	if err != nil {
		log.Errorf("failed to extract relyInfo list: %v", err)
		return
	}
	pingLatencies := s.getPingLatencies(relays) // list of SDN relays sorted by ascending order of Latency
	if len(pingLatencies) == 0 {
		log.Errorf("ping latencies not found for relays from SDN")
		return
	}
	connectedAutoRelays := s.getAutoConnectedRelays(ignoredRelays)
	fastestAvailableRelays := s.findFastestAvailableRelays(pingLatencies, connectedAutoRelays)
	relaysToSwitch := s.findRelaysToSwitch(connectedAutoRelays, fastestAvailableRelays)

	for oldRelay, newRelays := range relaysToSwitch {
		relayInstructions <- RelayInstruction{IP: oldRelay.ip, Port: oldRelay.port, Type: Switch, RelaysToSwitch: newRelays}
	}
}

func (s *realSDNHTTP) manageAutoRelays(autoRelayCount int, relayInstructions chan<- RelayInstruction, relays message.Peers, ignoredRelays IgnoredRelaysMap) {
	pingLatencies := s.getPingLatencies(relays) // list of SDN relays sorted by ascending order of latency
	if len(pingLatencies) == 0 {
		log.Errorf("ping latencies not found for relays from SDN")
		return
	}

	autoRelayCounter := 0

	for idx, pingLatency := range pingLatencies {
		newRelayIP, err := GetIP(pingLatency.IP)
		if err != nil {
			log.Errorf("relay %s from the SDN does not have a valid IP address: %v", pingLatency.IP, err)
			continue
		}
		// only connect to the relay if not already connected to or still connected
		if _, ok := ignoredRelays.LoadOrStore(newRelayIP, types.RelayInfo{TimeAdded: time.Now(), IsConnected: true, Port: pingLatency.Port}); ok {
			continue
		}
		logLowestLatency(pingLatencies[idx])
		relayInstructions <- RelayInstruction{IP: newRelayIP, Port: pingLatency.Port, Type: Connect}

		autoRelayCounter++
		if autoRelayCounter == autoRelayCount {
			// we found all autoRelays so we are done
			return
		}
	}
	// if we are here we failed to find all needed auto relays
	log.Errorf("available SDN relays %v; requested auto count %v", autoRelayCounter, autoRelayCount)
}

func (s *realSDNHTTP) FindNewRelay(ctx context.Context, oldRelayIP string, oldRelayIPPort int64, relayInstructions chan RelayInstruction, ignoredRelays IgnoredRelaysMap) {
	log.Errorf("relay %v is not reachable, switching relay", oldRelayIP)
	ignoredRelays.Store(oldRelayIP, types.RelayInfo{TimeAdded: time.Now(), Port: oldRelayIPPort, IsConnected: false})
	for {
		err := s.connectToNewRelay(relayInstructions, ignoredRelays)
		if err == nil {
			return // Exit the function if successful
		}
		log.Errorf("error while trying to reconnect to other relay: %v", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(types.RelayMonitorInterval):
		}
	}
}

// NodeModel returns the node model returned by the SDN
func (s *realSDNHTTP) NodeModel() *message.NodeModel {
	s.nodeLock.RLock()
	defer s.nodeLock.RUnlock()

	return s.nodeModel
}

// AccountTier returns the account tier name
func (s *realSDNHTTP) AccountTier() message.AccountTier {
	return s.accountModel.TierName
}

// AccountModel returns the account model
func (s *realSDNHTTP) AccountModel() message.Account {
	return *s.accountModel
}

// NetworkNum returns the registered network number of the node model
func (s *realSDNHTTP) NetworkNum() types.NetworkNum {
	s.nodeLock.RLock()
	defer s.nodeLock.RUnlock()

	return s.nodeModel.BlockchainNetworkNum
}

func (s *realSDNHTTP) httpClient() (*http.Client, error) {
	var tlsConfig *tls.Config
	var err error
	if s.sslCerts.NeedsPrivateCert() {
		tlsConfig, err = s.sslCerts.LoadRegistrationConfig()
	} else {
		tlsConfig, err = s.sslCerts.LoadPrivateConfig()
	}
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: httpTimeout,
	}

	return client, nil
}

// Register submits a registration request to bxapi. This will return private certificates for the node
// and assign a node ID.
func (s *realSDNHTTP) Register() error {
	if s.sslCerts.NeedsPrivateCert() {
		log.Debug("new private certificate needed, appending csr to node registration")
		csr, err := s.sslCerts.CreateCSR()
		if err != nil {
			return err
		}
		s.nodeLock.Lock()
		s.nodeModel.Csr = string(csr)
		s.nodeLock.Unlock()
	} else {
		nodeID, err := s.sslCerts.GetNodeID()
		if err != nil {
			return err
		}
		s.nodeID = nodeID
	}

	s.nodeLock.RLock()
	nodeType := s.nodeModel.NodeType
	nodeModelNodeID := s.nodeModel.NodeID
	srcVersion := s.nodeModel.SourceVersion
	externalIP := s.nodeModel.ExternalIP
	packed := bytes.NewBuffer(s.nodeModel.Pack())
	s.nodeLock.RUnlock()

	if nodeModelNodeID != "" {
		log.Debugf("registering SDN for %s with node ID '%v' and version '%v'", nodeType, nodeModelNodeID, srcVersion)
	} else {
		log.Debugf("registering SDN for %s with IP '%v' and version '%v'", nodeType, externalIP, srcVersion)
	}

	resp, err := s.httpWithCache(s.sdnURL+"/nodes", http.MethodPost, nodeModelCacheFileName, packed)
	if err != nil {
		return err
	}

	accountID, err := s.sslCerts.GetAccountID()
	if err != nil {
		return err
	}

	s.nodeLock.Lock()
	if err = json.Unmarshal(resp, s.nodeModel); err != nil {
		s.nodeLock.Unlock()
		return fmt.Errorf("could not deserialize '%s' response into node model: %v", string(resp), err)
	}
	s.nodeID = s.nodeModel.NodeID
	s.accountID = accountID
	newCert := s.nodeModel.Cert
	s.nodeLock.Unlock()

	if s.sslCerts.NeedsPrivateCert() {
		err := s.sslCerts.SavePrivateCert(newCert)
		// should pretty much never happen unless there are SDN problems, in which
		// case just abort on startup
		if err != nil {
			debug.PrintStack()
			panic(err)
		}
	}
	return nil
}

// NeedsRegistration indicates whether proxy must register with the SDN to run
func (s *realSDNHTTP) NeedsRegistration() bool {
	return s.nodeID == "" || s.sslCerts.NeedsPrivateCert()
}

func (s *realSDNHTTP) close(resp *http.Response) {
	err := resp.Body.Close()
	if err != nil {
		log.Error(fmt.Errorf("could not close response body %v: %v", resp.Body, err))
	}
}

func (s *realSDNHTTP) GetQuotaUsage(accountID string) (*QuotaResponseBody, error) {
	reqBody := QuotaRequestBody{
		AccountID: accountID,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		log.Errorf("unable to marshal SDN request: %v", err)
		return nil, err
	}

	resp, err := s.Get("/accounts/quota-status", body)
	if err != nil {
		return nil, err
	}

	quotaResp := QuotaResponseBody{}
	if err = json.Unmarshal(resp, &quotaResp); err != nil {
		return nil, fmt.Errorf("could not deserialize '%s' response into quota response: %v", string(resp), err)
	}

	return &quotaResp, nil
}

func (s *realSDNHTTP) getAccountModelWithEndpoint(accountID types.AccountID, endpoint string) (message.Account, error) {
	url := fmt.Sprintf("%v/%v/%v", s.sdnURL, endpoint, accountID)
	accountModel := message.Account{}
	// for accounts endpoint we do no want to use the cache file.
	// in case of SDN error, we set default enterprise account for the customer
	var resp []byte
	var err error
	switch endpoint {
	case "accounts":
		resp, err = s.http(url, http.MethodGet, nil)
	case "account":
		resp, err = s.httpWithCache(url, http.MethodGet, accountModelsFileName, nil)
	default:
		log.Panicf("getAccountModelWithEndpoint called with unsupported endpoint %v", endpoint)
	}

	if err != nil {
		return accountModel, fmt.Errorf("could not get account model from SDN: %v", err)
	}

	if err = json.Unmarshal(resp, &accountModel); err != nil {
		return accountModel, fmt.Errorf("could not deserialize '%s' response into account model: %v", string(resp), err)
	}

	return accountModel, nil
}

func (s *realSDNHTTP) getAccountModel(accountID types.AccountID) error {
	accountModel, err := s.getAccountModelWithEndpoint(accountID, "account")
	s.accountModel = &accountModel
	if s.accountModel.RelayLimit.MsgQuota.Limit == 0 {
		log.Warnf("relay limit was set to 0, setting to 1")
		s.accountModel.RelayLimit.MsgQuota.Limit = 1
	}

	return err
}

// FetchCustomerAccountModel get customer account model
func (s *realSDNHTTP) FetchCustomerAccountModel(accountID types.AccountID) (message.Account, error) {
	return s.getAccountModelWithEndpoint(accountID, "accounts")
}

// getRelays gets the potential relays for a gateway
func (s *realSDNHTTP) getRelays(nodeID types.NodeID, networkNum types.NetworkNum) (message.Peers, error) {
	url := fmt.Sprintf("%v/nodes/%v/%v/potential-relays", s.sdnURL, nodeID, networkNum)
	resp, err := s.httpWithCache(url, http.MethodGet, potentialRelaysFileName, nil)
	if err != nil {
		return nil, err
	}
	var relays message.Peers
	if err = json.Unmarshal(resp, &relays); err != nil {
		return nil, fmt.Errorf("could not deserialize '%s' response into potential relays: %v", string(resp), err)
	}
	return relays, nil
}

func (s *realSDNHTTP) httpWithCache(uri string, method string, fileName string, body io.Reader) ([]byte, error) {
	return s.httpWithCacheAndContext(context.Background(), uri, method, fileName, body)
}

func (s *realSDNHTTP) httpWithCacheAndContext(ctx context.Context, uri string, method string, fileName string, body io.Reader) ([]byte, error) {
	var err error
	data, httpErr := s.httpWithContext(ctx, uri, method, body)
	if httpErr != nil {
		if errors.Is(httpErr, ErrSDNUnavailable) {
			// we can't get the data from http - try to read from cache file
			data, err = LoadCacheFile(s.dataDir, fileName)
			if err != nil {
				return nil, fmt.Errorf("got error from http request: %v and can't load cache file %v: %v", httpErr, fileName, err)
			}
			// we managed to read the data from cache file - issue a warning
			log.Warnf("got error from http request: %v but loaded cache file %v", httpErr, fileName)
			return data, nil
		}
		return nil, httpErr
	}

	err = UpdateCacheFile(s.dataDir, fileName, data)
	if err != nil {
		log.Warnf("can not update cache file %v with data %s. error %v", fileName, data, err)
	}
	return data, nil
}

func (s *realSDNHTTP) http(uri string, method string, body io.Reader) ([]byte, error) {
	return s.httpWithContext(context.Background(), uri, method, body)
}

func (s *realSDNHTTP) httpWithContext(ctx context.Context, uri string, method string, body io.Reader) ([]byte, error) {
	client, err := s.httpClient()
	if err != nil {
		return nil, err
	}
	var resp *http.Response
	defer func() {
		if resp != nil {
			s.close(resp)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, method, uri, body)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusServiceUnavailable {
			log.Debugf("got error from http request: SDN is down")
			return nil, ErrSDNUnavailable
		}
		if resp.Body != nil {
			b, errMsg := io.ReadAll(resp.Body)
			if errMsg != nil {
				return nil, fmt.Errorf("%v on %v could not read response %v, error %v", method, uri, resp.Status, errMsg.Error())
			}
			var errorMessage message.ErrorMessage
			if err = json.Unmarshal(b, &errorMessage); err != nil {
				return nil, fmt.Errorf("could not deserialize '%s' response into error message: %v", string(b), err)
			}
			err = fmt.Errorf("%v to %v received a [%v]: %v", method, uri, resp.Status, errorMessage.Details)
		} else {
			err = fmt.Errorf("%v on %v recv and error %v", method, uri, resp.Status)
		}
		return nil, err
	}

	b, errMsg := io.ReadAll(resp.Body)
	if errMsg != nil {
		return nil, fmt.Errorf("%v on %v could not read response %v, error %v", method, uri, resp.Status, errMsg.Error())
	}
	return b, nil
}

func (s *realSDNHTTP) getBlockchainNetworks() error {
	url := fmt.Sprintf("%v/blockchain-networks", s.sdnURL)
	resp, err := s.httpWithCache(url, http.MethodGet, blockchainNetworksCacheFileName, nil)
	if err != nil {
		return err
	}
	var networks []*message.BlockchainNetwork
	if err = json.Unmarshal(resp, &networks); err != nil {
		return fmt.Errorf("could not deserialize '%s' response into blockchain networks: %v", string(resp), err)
	}
	s.networks = message.BlockchainNetworks{}
	for _, network := range networks {
		s.networks[network.NetworkNum] = network
	}
	return nil
}

// FindNetwork finds a BlockchainNetwork instance by its number and allow update
func (s *realSDNHTTP) FindNetwork(networkNum types.NetworkNum) (*message.BlockchainNetwork, error) {
	return s.networks.FindNetwork(networkNum)
}

// MinTxAge returns MinTxAge for the current blockchain number the node model registered
func (s *realSDNHTTP) MinTxAge() time.Duration {
	blockchainNetwork, err := s.FindNetwork(s.NetworkNum())
	if err != nil {
		log.Warnf("could not get blockchainNetwork: %v, returning default 2 seconds for MinTxAgeSecond", err)
		return 2 * time.Second
	}
	return time.Duration(float64(time.Second) * blockchainNetwork.MinTxAgeSeconds)
}

// getPingLatencies pings list of SDN peers and returns sorted list of nodeLatencyInfo for each successful peer ping
func getPingLatencies(peers message.Peers) []nodeLatencyInfo {
	var wg sync.WaitGroup
	resultsChan := make(chan nodeLatencyInfo, len(peers))

	for _, peer := range peers {
		wg.Add(1)
		go func(p message.Peer) {
			defer wg.Done()

			address := net.JoinHostPort(p.IP, strconv.FormatInt(p.Port, 10))

			start := time.Now()
			conn, err := net.DialTimeout("tcp", address, time.Duration(PingTimeout)*time.Millisecond)
			duration := time.Since(start)

			if err != nil {
				resultsChan <- nodeLatencyInfo{IP: p.IP, Port: p.Port, Latency: 999999.0}
				return
			}
			conn.Close()

			// convert duration to milliseconds
			latencyMs := float64(duration.Microseconds()) / 1000.0
			resultsChan <- nodeLatencyInfo{IP: p.IP, Port: p.Port, Latency: latencyMs}
		}(peer)
	}

	wg.Wait()
	close(resultsChan)

	var pingResults []nodeLatencyInfo
	for res := range resultsChan {
		pingResults = append(pingResults, res)
	}

	sort.Slice(pingResults, func(i, j int) bool {
		return pingResults[i].Latency < pingResults[j].Latency
	})

	return pingResults
}

// SendNodeEvent sends node event to SDN through http
func (s *realSDNHTTP) SendNodeEvent(event message.NodeEvent, id types.NodeID) {
	url := fmt.Sprintf("%v/nodes/%v/events", s.sdnURL, id)
	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Errorf("could not serialize node event %v: %v", event, err)
		return
	}
	resp, err := s.http(url, http.MethodPost, bytes.NewBuffer(eventBytes))
	if err != nil {
		log.Errorf("could not send node event %v to SDN: %v", event.EventType, err)
		return
	}
	log.Infof("node event %v sent to SDN, resp: %s", event.EventType, string(resp))
}

// SDNURL getter for the private sdnURL field
func (s *realSDNHTTP) SDNURL() string {
	return s.sdnURL
}

// NodeID getter for the private nodeID field
func (s *realSDNHTTP) NodeID() types.NodeID {
	return s.nodeID
}

// Networks getter for the private networks field
func (s *realSDNHTTP) Networks() *message.BlockchainNetworks {
	return &s.networks
}

// SetNetworks setter for the private networks field
func (s *realSDNHTTP) SetNetworks(networks message.BlockchainNetworks) {
	s.networks = networks
}

// SetInternalGateway updates the current internal gateway state. [for internal use only]
func (s *realSDNHTTP) SetInternalGateway(state *message.InternalGateway) error {
	url := fmt.Sprintf("%v/internal-gateways/%v", s.sdnURL, s.nodeID)
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("could not serialize internal gateway state: %w", err)
	}
	if _, err = s.http(url, http.MethodPost, bytes.NewBuffer(stateBytes)); err != nil {
		return fmt.Errorf("could not send internal gateway state: %w", err)
	}
	return nil
}

// AddInternalGatewaySubscription tries to add an account subscription. [for internal use only]
func (s *realSDNHTTP) AddInternalGatewaySubscription(accountID types.AccountID) error {
	url := fmt.Sprintf("%v/internal-gateways/%v/subscriptions/%v", s.sdnURL, s.nodeID, accountID)
	if _, err := s.http(url, http.MethodPost, nil); err != nil {
		return fmt.Errorf("could not send request to add internal gateway subscription: %w", err)
	}
	return nil
}

// RemoveInternalGatewaySubscription removes an account subscription. [for internal use only]
func (s *realSDNHTTP) RemoveInternalGatewaySubscription(accountID types.AccountID) error {
	url := fmt.Sprintf("%v/internal-gateways/%v/subscriptions/%v", s.sdnURL, s.nodeID, accountID)
	if _, err := s.http(url, http.MethodDelete, nil); err != nil {
		return fmt.Errorf("could not send request to remove internal gateway subscription: %w", err)
	}
	return nil
}
