package message

// InternalGateway is an internal gateway state.
type InternalGateway struct {
	AccountSubscriptionCounts map[string]uint `json:"account_subscription_counts"`
}
