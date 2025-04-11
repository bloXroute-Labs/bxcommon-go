package types

import (
	"fmt"
	"strings"
)

// TimeDateLayoutISO - used to parse ISO time date format string
const TimeDateLayoutISO = "2006-01-02"

// ExpiredDate - constant for an expired date
const ExpiredDate = "1970-01-01"

// NodeID represents a node's assigned ID. This field is a UUID.
type NodeID string

// AccountID represents a user's BDN account. This field is a UUID.
type AccountID string

// NodeType represents flag indicating node type (Gateway, Relay, etc.)
type NodeType int

// NetworkID represent the chain ID that a message is being routed in (1 for Ethereum Mainnet, 56 for BSC-Mainnet, etc.)
type NetworkID int64

// NetworkNum represents the network that a message is being routed in (Ethereum Mainnet, Ethereum Holesky, etc.)
type NetworkNum uint32

const (
	// InternalGateway is a gateway run by bloxroute
	InternalGateway NodeType = 1 << iota

	// ExternalGateway is a gateway run by anyone
	ExternalGateway

	_ //Deprecated: RelayTransaction used for legacy relay

	// API is the bloxroute SDN
	API

	// APISocket is the bloxroute SDN socket broker
	APISocket

	// CloudAPI is the cloud API instances
	CloudAPI

	_ // Deprecated: Jobs used for legacy jobs that used for monitor cloud services

	_ // Deprecated: GatewayGo used for legacy gateway

	// RelayProxy is the proxy relay that connects to gateways and sits in front of relays
	RelayProxy

	// Websocket is a websocket connection to a node
	Websocket

	// GRPC is a gRPC connection
	GRPC

	// Blockchain represents a blockchain connection type
	Blockchain

	// SolanaRelay is a relay routing solana messages only
	SolanaRelay

	// Gateway collects all the various gateway types
	Gateway = InternalGateway | ExternalGateway
)

var nodeTypeNames = map[NodeType]string{
	InternalGateway: "INTERNAL_GATEWAY",
	ExternalGateway: "EXTERNAL_GATEWAY",
	API:             "API",
	APISocket:       "API_SOCKET",
	CloudAPI:        "BLOXROUTE_CLOUD_API",
	Gateway:         "GATEWAY",
	RelayProxy:      "RELAY_PROXY",
	Websocket:       "WEBSOCKET",
	GRPC:            "GRPC",
	Blockchain:      "BLOCKCHAIN",
	SolanaRelay:     "SOLANA_RELAY",
}

var nodeNameTypes = map[string]NodeType{
	"INTERNAL_GATEWAY":    InternalGateway,
	"EXTERNAL_GATEWAY":    ExternalGateway,
	"API":                 API,
	"API_SOCKET":          APISocket,
	"BLOXROUTE_CLOUD_API": CloudAPI,
	"GATEWAY":             Gateway,
	"WEBSOCKET":           Websocket,
	"GRPC":                GRPC,
	"RELAY_PROXY":         RelayProxy,
	"BLOCKCHAIN":          Blockchain,
	"SOLANA_RELAY":        SolanaRelay,
}

// String returns the string representation of a node type for use (e.g. in JSON dumps)
func (n NodeType) String() string {
	s, ok := nodeTypeNames[n]
	if ok {
		return s
	}
	return "UNKNOWN"
}

// DeserializeNodeType parses the node type from a serialized form.
// Placeholder function, since this node type is not currently used.
func DeserializeNodeType(b []byte) (NodeType, error) {
	s, ok := nodeNameTypes[string(b)]
	if ok {
		return s, nil
	}
	return 0, fmt.Errorf("could not deserialize unknown node value %v", string(b))
}

// FromStringToNodeType return nodeType of string name
func FromStringToNodeType(s string) (NodeType, error) {
	cs := strings.Replace(s, "-", "", -1)
	cs = strings.ToUpper(cs)
	nt, ok := nodeNameTypes[cs]
	if ok {
		return nt, nil
	}
	return 0, fmt.Errorf("could not deserialize unknown node value %v", cs)
}

// FormatShortNodeType returns the short string representation of a node type
func (n NodeType) FormatShortNodeType() string {
	if n&Gateway != 0 {
		return "G"
	}
	if n&RelayProxy != 0 {
		return "R"
	}
	return n.String()
}
