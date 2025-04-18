package message

import (
	"time"

	"github.com/bloXroute-Labs/bxcommon-go/types"
)

// FirewallRule is SDN P2P message that sent to proxy
type FirewallRule struct {
	AccountID      types.AccountID `json:"account_id"`
	PeerID         types.NodeID    `json:"node_id"`
	Duration       int             `json:"duration"`
	Reason         string          `json:"reason"`
	expirationTime time.Time
}

// GetExpirationTime - returns the expirationTime of a rule
func (firewallRule *FirewallRule) GetExpirationTime() time.Time {
	return firewallRule.expirationTime
}

// SetExpirationTime - sets the expirationTime of a rule
func (firewallRule *FirewallRule) SetExpirationTime(expirationTime time.Time) {
	firewallRule.expirationTime = expirationTime
}
