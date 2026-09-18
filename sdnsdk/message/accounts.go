package message

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bloXroute-Labs/bxcommon-go/types"
)

// AccountRequest represents a request to bxapi for account details
// Either field is nullable, so fields must be pointers.
type AccountRequest struct {
	AccountID *types.AccountID `json:"account_id"`
	PeerID    *types.NodeID    `json:"peer_id"`
}

// AccountResponse represents a response from bxapi with account details
type AccountResponse struct {
	NodeID  *types.NodeID `json:"node_id"` // will be none if a pushed down update from bxapi
	Account *Account      `json:"account"`
}

// AccountTier represents a tier name
type AccountTier string

// AccountTier types enumeration
const (
	ATierUltra        AccountTier = "Ultra"
	ATierElite        AccountTier = "EnterpriseElite"
	ATierEnterprise   AccountTier = "Enterprise"
	ATierProfessional AccountTier = "Professional"
	ATierDeveloper    AccountTier = "Developer"
	ATierIntroductory AccountTier = "Introductory"
)

// GetRequestPriority return priority for blxr_private_tx and blxr_submit_bundle requests
func (at AccountTier) GetRequestPriority() int {
	var priority int
	switch at {
	case ATierUltra:
		priority = 5
	case ATierElite:
		priority = 4
	case ATierEnterprise:
		priority = 3
	case ATierProfessional, ATierDeveloper:
		priority = 2
	case ATierIntroductory:
		priority = 1
	default:
		priority = 0
	}
	return priority
}

// IsAtLeast indicates whether the account tier is higher or equal to minimumTier
func (at AccountTier) IsAtLeast(minimumTier AccountTier) bool {
	return at.GetRequestPriority() >= minimumTier.GetRequestPriority()
}

// IsUltra indicates whether the account tier is ultra
func (at AccountTier) IsUltra() bool {
	return at == ATierUltra
}

// IsElite indicates whether the account tier is elite or ultra
func (at AccountTier) IsElite() bool {
	return at == ATierElite || at == ATierUltra
}

// IsEnterprise indicates whether the account tier is considered enterprise, elite or ultra
func (at AccountTier) IsEnterprise() bool {
	return at == ATierEnterprise || at == ATierElite || at == ATierUltra
}

// ReceivesUnpaidTxs indicates whether the account tier receives unpaid txs (only >= ATierProfessional)
func (at AccountTier) ReceivesUnpaidTxs() bool {
	return at == ATierUltra || at == ATierElite || at == ATierEnterprise || at == ATierProfessional
}

// IsValid indicates whether the account tier is valid
func (at AccountTier) IsValid() error {
	switch at {
	case ATierUltra, ATierElite, ATierEnterprise, ATierProfessional, ATierDeveloper, ATierIntroductory:
		return nil
	}
	return fmt.Errorf("unrecognized account tier: %v", at)
}

// TimeIntervalType represents an time interval type
type TimeIntervalType string

// BDNServiceLimit represents a quota, saturating at the int64 bounds for values the SDN
// sends outside them. It can be negative.
type BDNServiceLimit int64

// TimeIntervalType enumeration
const (
	TimeIntervalDaily   TimeIntervalType = "DAILY"
	TimeIntervalWithout TimeIntervalType = "WITHOUT_INTERVAL"
)

// BDNServiceType represents a BDN service type
type BDNServiceType string

// BDNServiceType enumeration
const (
	BDNServiceMsgQuota BDNServiceType = "MSG_QUOTA"
	BDNServicePermit   BDNServiceType = "PERMIT"
)

// BDNServiceBehaviorType represents various flags for service handling behaviors
type BDNServiceBehaviorType string

// BDNServiceBehaviorType enumeration
const (
	// BehaviorNoAction means
	BehaviorNoAction BDNServiceBehaviorType = "NO_ACTION"
	// BehaviorBlock means to block transaction propagation
	BehaviorBlock BDNServiceBehaviorType = "BLOCK"
	// BehaviorAlert means issue customer alert
	BehaviorAlert BDNServiceBehaviorType = "ALERT"
	// BehaviorAuditLog means log audit entry
	BehaviorAuditLog BDNServiceBehaviorType = "AUDIT_LOG"
	// BehaviorBlockAlert means to block event and issue customer alert
	BehaviorBlockAlert BDNServiceBehaviorType = "BLOCK_ALERT"
	// BehaviorAuditAlert means
	BehaviorAuditAlert BDNServiceBehaviorType = "AUDIT_ALERT"
)

// BDNMinAllowedNodesService represents a service model config for MinAllowedNodes
type BDNMinAllowedNodesService struct {
	BDNQuotaService
}

// IsActive indicates whether the BDNMinAllowedNodesService is not expired and limit is 0
func (bdnmans BDNMinAllowedNodesService) IsActive() bool {
	return !bdnmans.ExpireDate.Expired() && bdnmans.MsgQuota.Limit == 0
}

// BDNService represents a service model config
// This struct is roughly equivalent to 'BdnServiceModel' in Python
type BDNService struct {
	TimeInterval      TimeIntervalType       `json:"interval"`
	ServiceType       BDNServiceType         `json:"service_type"`
	Limit             BDNServiceLimit        `json:"limit"`
	BehaviorLimitOK   BDNServiceBehaviorType `json:"behavior_limit_ok"`
	BehaviorLimitFail BDNServiceBehaviorType `json:"behavior_limit_fail"`
}

// BDNQuotaService represents quota service model configs
type BDNQuotaService struct {
	MsgQuota   BDNService    `json:"msg_quota"`
	ExpireDate types.ISODate `json:"expire_date"`
}

// UnmarshalJSON implements deserialization for BDNServiceLimit. A quota outside int64 is
// saturated instead of failing the account: strconv.ParseInt already returns MaxInt64 or
// MinInt64 alongside ErrRange, with the right sign, so the parsed value is used as-is.
// Any other parse failure is reported.
func (i *BDNServiceLimit) UnmarshalJSON(b []byte) error {
	limit, err := json.Number(b).Int64()
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return err
	}

	*i = BDNServiceLimit(limit)

	return nil
}

// nullJSON is the literal bxapi sends for a service an account is not provisioned for.
const nullJSON = "null"

// UnmarshalJSON implements deserialization for BDNQuotaService. bxapi sends a service the
// account is not provisioned for as JSON null, which the default struct decoding would leave
// at the Go zero value; decoding it to the expired date instead preserves what the SDN means
// and keeps a decode/encode/decode cycle idempotent for the SDK's on-disk account cache.
func (bdnQS *BDNQuotaService) UnmarshalJSON(b []byte) error {
	if string(b) == nullJSON {
		*bdnQS = BDNQuotaService{ExpireDate: types.ExpiredISODate}

		return nil
	}

	// the local type strips the method set to avoid recursing, and keeps every field and
	// struct tag, so adding a field here needs no change to this method
	type quotaService BDNQuotaService

	var qs quotaService
	if err := json.Unmarshal(b, &qs); err != nil {
		return err
	}

	*bdnQS = BDNQuotaService(qs)

	return nil
}

// IsActive indicates whether the BDNQuotaService is not expired and has quota left
func (bdnQS BDNQuotaService) IsActive() bool {
	return !bdnQS.ExpireDate.Expired() && bdnQS.MsgQuota.Limit > 0
}

// SubscriptionPlanType represents the available feed subscription plan types
type SubscriptionPlanType string

// SubscriptionPlanType enumeration
const (
	SubscriptionPlanFeeds    SubscriptionPlanType = "FEEDS"
	SubscriptionPlanNetworks SubscriptionPlanType = "NETWORKS"
)

// AllowedNetworksForFeedsPlan is the number of allowed networks for SubscriptionPlanFeeds plans
const AllowedNetworksForFeedsPlan = 1

// FeedProperties represent feed in BDN service
type FeedProperties struct {
	AllowFiltering  bool                 `json:"allow_filtering"`
	AvailableFields []string             `json:"available_fields"`
	Plan            SubscriptionPlanType `json:"plan"`
	Limit           int                  `json:"limit"`
}

// BDNBasicService is a placeholder for service model configs
type BDNBasicService struct {
	ExpireDate types.ISODate `json:"expire_date"`
}

// UnmarshalJSON implements deserialization for BDNBasicService; see BDNQuotaService for why
// a JSON null decodes to the expired date.
func (bdnbs *BDNBasicService) UnmarshalJSON(b []byte) error {
	if string(b) == nullJSON {
		*bdnbs = BDNBasicService{ExpireDate: types.ExpiredISODate}

		return nil
	}

	type basicService BDNBasicService

	var bs basicService
	if err := json.Unmarshal(b, &bs); err != nil {
		return err
	}

	*bdnbs = BDNBasicService(bs)

	return nil
}

// IsActive indicates whether the BDNBasicService is not expired
func (bdnbs BDNBasicService) IsActive() bool {
	return !bdnbs.ExpireDate.Expired()
}

// BDNFeedService is a placeholder for service model configs
type BDNFeedService struct {
	ExpireDate      types.ISODate  `json:"expire_date"`
	Feed            FeedProperties `json:"feed"`
	AllowedNetworks []string       `json:"allowed_networks"`
}

// UnmarshalJSON implements deserialization for BDNFeedService; see BDNQuotaService for why
// a JSON null decodes to the expired date.
func (bdnFS *BDNFeedService) UnmarshalJSON(b []byte) error {
	if string(b) == nullJSON {
		*bdnFS = BDNFeedService{ExpireDate: types.ExpiredISODate}

		return nil
	}

	type feedService BDNFeedService

	var fs feedService
	if err := json.Unmarshal(b, &fs); err != nil {
		return err
	}

	*bdnFS = BDNFeedService(fs)

	return nil
}

// IsActive indicates whether the BDNFeedService is not expired
func (bdnFS BDNFeedService) IsActive() bool {
	return !bdnFS.ExpireDate.Expired()
}

// BDNPrivateRelayService is a placeholder for service model configs
type BDNPrivateRelayService interface{}

// ActiveService is implemented by every service model that carries an expiry
type ActiveService interface {
	IsActive() bool
}

var (
	_ ActiveService = BDNQuotaService{}
	_ ActiveService = BDNBasicService{}
	_ ActiveService = BDNFeedService{}
	_ ActiveService = BDNMinAllowedNodesService{}
)

// Account represents the account structure fetched from bxapi
type Account struct {
	AccountInfo
	SecretHash                          string                 `json:"secret_hash"`
	FreeTransactions                    BDNQuotaService        `json:"tx_free"`
	PaidTransactions                    BDNQuotaService        `json:"tx_paid"`
	CloudAPI                            BDNBasicService        `json:"cloud_api"`
	NewTransactionStreaming             BDNFeedService         `json:"new_transaction_streaming"`
	NewBlockStreaming                   BDNFeedService         `json:"new_block_streaming"`
	PendingTransactionStreaming         BDNFeedService         `json:"new_pending_transaction_streaming"`
	InternalTransactionMinedStreaming   BDNFeedService         `json:"new_internal_transaction_streaming"`
	InternalTransactionMempoolStreaming BDNFeedService         `json:"new_internal_transaction_pending_streaming"`
	TransactionStateFeed                BDNFeedService         `json:"transaction_state_feed"`
	OnBlockFeed                         BDNFeedService         `json:"on_block_feed"`
	TransactionReceiptFeed              BDNFeedService         `json:"transaction_receipts_feed"`
	PrivateRelay                        BDNPrivateRelayService `json:"private_relays"`
	PrivateTransaction                  BDNQuotaService        `json:"private_transaction"`
	PrivateTransactionFee               BDNQuotaService        `json:"private_transaction_fee"`
	RelayLimit                          BDNQuotaService        `json:"relay_limit"`
	MaxAllowedNodes                     BDNQuotaService        `json:"max_allowed_nodes"`
	InboundNodeConnections              BDNQuotaService        `json:"inbound_node_connections"`

	// txs allowed per 5s
	UnpaidTransactionBurstLimit BDNQuotaService `json:"unpaid_tx_burst_limit"`
	PaidTransactionBurstLimit   BDNQuotaService `json:"paid_tx_burst_limit"`
	VIPBuildersAllowed          BDNQuotaService `json:"vip_builders"`

	BoostMEVSearcher BDNBasicService `json:"boost_mevsearcher"`

	SolanaDexAPIRateLimit   BDNQuotaService `json:"solana_dex_api_rate_limit"`
	SolanaDexAPIStreamLimit BDNQuotaService `json:"solana_dex_api_stream_limit"`

	PrivateOrdersStreaming        BDNFeedService `json:"private_orders_streaming"`
	PendingPrivateTxsStreaming    BDNFeedService `json:"pending_private_txs_streaming"`
	MEVProposerGetHeaderStreaming BDNFeedService `json:"mev_get_header_streaming"`

	EthValidatorGateway BDNQuotaService `json:"eth_validator_gateway"`

	EthBundlePerBlock  BDNQuotaService `json:"eth_bundle_per_block"`
	EthBundlePerSecond BDNQuotaService `json:"eth_bundle_per_second"`
	BscBundlePerBlock  BDNQuotaService `json:"bsc_bundle_per_block"`
	BscBundlePerSecond BDNQuotaService `json:"bsc_bundle_per_second"`

	// Pricing restructure
	EthMempoolStreaming    BDNQuotaService `json:"eth_mempool_streaming"`
	EthBlocksStreaming     BDNQuotaService `json:"eth_blocks_streaming"`
	EthTxReceiptsStreaming BDNQuotaService `json:"eth_tx_receipts_streaming"`
	EthMevStreaming        BDNBasicService `json:"eth_mev_streaming"`
	EthBundleSimulation    BDNBasicService `json:"eth_bundle_simulation"`
	EthBuilder             BDNBasicService `json:"eth_builder"`

	BscMempoolStreaming    BDNQuotaService `json:"bsc_mempool_streaming"`
	BscBlocksStreaming     BDNQuotaService `json:"bsc_blocks_streaming"`
	BscTxReceiptsStreaming BDNQuotaService `json:"bsc_tx_receipts_streaming"`
	BscBundleSimulation    BDNBasicService `json:"bsc_bundle_simulation"`
	BscBigBundles          BDNBasicService `json:"bsc_big_bundles"`
	BscBoosterNetwork      BDNBasicService `json:"bsc_booster_network"`

	BaseFlashblocksStreaming       BDNQuotaService `json:"base_flashblocks_streaming"`
	BaseParsedFlashblocksStreaming BDNQuotaService `json:"base_parsed_flashblocks_streaming"`
	BaseBoosterNetwork             BDNBasicService `json:"base_booster_network"`
	BaseStateDiffStreaming         BDNQuotaService `json:"base_state_diff_streaming"`

	OnlineSolanaGateways       BDNQuotaService            `json:"online_solana_gateways"`
	SolanaShredStreams         BDNQuotaService            `json:"solana_shred_streams"`
	SolanaRegionalShredStreams map[string]BDNQuotaService `json:"solana_regional_shred_streams"`
	SolanaTxStreamers          BDNQuotaService            `json:"solana_tx_streamers"`
	SolanaTraderApiCredits     BDNQuotaService            `json:"solana_trader_api_credits"`

	// TxTool
	TxTraceRateLimitation     BDNQuotaService `json:"tx_trace_rate_limitation"`
	BundleTraceRateLimitation BDNQuotaService `json:"bundle_trace_rate_limitation"`

	OnlineGateways    BDNQuotaService           `json:"online_gateways"`
	MinAllowedNodes   BDNMinAllowedNodesService `json:"min_allowed_nodes"`
	BDNPrivateRegions BDNBasicService           `json:"bdn_private_regions"`
}

// Validate verifies the response that the response from bxapi is well understood
func (a *Account) Validate() error {
	err := a.TierName.IsValid()
	if err != nil {
		a.TierName = ATierElite
		return err
	}
	return nil
}

// GetBundlesLimitForNetwork returns the bundle limits for the given network
func (a *Account) GetBundlesLimitForNetwork(network string) (uint64, uint64) {
	switch network {
	case types.Mainnet:
		return uint64(a.EthBundlePerBlock.MsgQuota.Limit), uint64(a.EthBundlePerSecond.MsgQuota.Limit)
	case types.BSCMainnet:
		return uint64(a.BscBundlePerBlock.MsgQuota.Limit), uint64(a.BscBundlePerSecond.MsgQuota.Limit)
	}

	return 10000, 10000
}

// AccountInfo represents basic info about the account model
// This struct is roughly equivalent to `AccountTemplate` in Python
type AccountInfo struct {
	AccountID          types.AccountID `json:"account_id"`
	LogicalAccountName string          `json:"logical_account_name"`
	Certificate        string          `json:"certificate"`
	ExpireDate         types.ISODate   `json:"expire_date"`
	BlockchainProtocol string          `json:"blockchain_protocol"`
	BlockchainNetwork  string          `json:"blockchain_network"`
	TierName           AccountTier     `json:"tier_name"`
	Miner              bool            `json:"is_miner"`
	Trusted            *bool           `json:"trusted"`
	MEVBuilders        []string        `json:"mev_builders"`
	BSCGrade           int             `json:"bsc_grade"`
	ETHGrade           int             `json:"eth_grade"`
}

// DefaultAccountGrade is the per-chain grade carried by GetDefaultEliteAccount.
//
// It is deliberately NOT 0. Consumers tier accounts by grade (cloud-api's grade_tiers), and
// 0 is the grade an ungraded account carries, so a fallback account left at the zero value
// would be indistinguishable from an ungraded one - meaning that whatever policy is attached
// to grade 0 would apply to every account the moment the SDN stops answering. Grading the
// fallback explicitly keeps the outage case out of that tier (DI-4161).
//
// The value is high on purpose, not arbitrary: it must land in the tier that applies no
// restriction, since during an outage it stands in for EVERY account, including the ones
// whose real grade is the highest. Under cloud-api's shipped thresholds 100 resolves to the
// top tier - no routing override, no tier rate limit. Lowering it would silently subject all
// traffic to a restricted tier's policy for the length of an SDN outage.
const DefaultAccountGrade = 100

// IsTrusted indicates whether the account is trusted. bxapi sends trusted as an
// optional bool; an absent or null value is treated as trusted, and only an explicit
// false makes an account untrusted. Miners are always trusted.
func (a *Account) IsTrusted() bool { return a.Trusted == nil || *a.Trusted || a.Miner }

// IsExpired indicates whether the account itself has expired, on the same terms as a
// service: calendar dates in UTC with the expiry day still valid. bxapi always sets
// expire_date, using EPOCH_DATE ("1970-01-01") for an account with no entitlement.
func (a *AccountInfo) IsExpired() bool { return a.ExpireDate.Expired() }

// GetDefaultEliteAccount get a default elite account by current time.
//
// Both per-chain grades are set to DefaultAccountGrade rather than left at the zero value -
// see that constant for why an SDN-outage fallback must not look ungraded.
func GetDefaultEliteAccount(now time.Time) Account {
	return Account{
		AccountInfo: AccountInfo{
			AccountID:          "",
			LogicalAccountName: "",
			Certificate:        "",
			ExpireDate:         types.NewISODate(now.AddDate(0, 0, 1)),
			BlockchainProtocol: "Ethereum",
			BlockchainNetwork:  "Mainnet",
			TierName:           ATierElite,
			Miner:              false,
			BSCGrade:           DefaultAccountGrade,
			ETHGrade:           DefaultAccountGrade,
		},
		FreeTransactions: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval: TimeIntervalDaily,
				ServiceType:  BDNServiceMsgQuota,
				Limit:        1,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		PaidTransactions: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval: TimeIntervalDaily,
				ServiceType:  BDNServiceMsgQuota,
				Limit:        1,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		CloudAPI: BDNBasicService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
		},
		NewTransactionStreaming: BDNFeedService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
			Feed: FeedProperties{
				AllowFiltering:  true,
				AvailableFields: []string{"all"},
				Plan:            SubscriptionPlanFeeds,
				Limit:           20,
			},
		},
		NewBlockStreaming: BDNFeedService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
			Feed: FeedProperties{
				AllowFiltering:  true,
				AvailableFields: []string{"all"},
				Plan:            SubscriptionPlanFeeds,
				Limit:           20,
			},
		},
		PendingTransactionStreaming: BDNFeedService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
			Feed: FeedProperties{
				AllowFiltering:  true,
				AvailableFields: []string{"all"},
				Plan:            SubscriptionPlanFeeds,
				Limit:           20,
			},
		},
		InternalTransactionMinedStreaming: BDNFeedService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
			Feed: FeedProperties{
				AllowFiltering:  true,
				AvailableFields: []string{"all"},
				Plan:            SubscriptionPlanFeeds,
				Limit:           20,
			},
		},
		InternalTransactionMempoolStreaming: BDNFeedService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
			Feed: FeedProperties{
				AllowFiltering:  true,
				AvailableFields: []string{"all"},
				Plan:            SubscriptionPlanFeeds,
				Limit:           20,
			},
		},
		TransactionStateFeed: BDNFeedService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
			Feed: FeedProperties{
				AllowFiltering:  true,
				AvailableFields: []string{"all"},
			},
		},
		OnBlockFeed: BDNFeedService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
			Feed: FeedProperties{
				AllowFiltering:  false,
				AvailableFields: nil,
			},
		},
		TransactionReceiptFeed: BDNFeedService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
			Feed: FeedProperties{
				AllowFiltering:  false,
				AvailableFields: nil,
			},
		},
		PrivateRelay: nil,
		PrivateTransaction: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval: TimeIntervalDaily,
				ServiceType:  BDNServiceMsgQuota,
				Limit:        1,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		UnpaidTransactionBurstLimit: BDNQuotaService{
			MsgQuota: BDNService{
				ServiceType:       BDNServiceMsgQuota,
				Limit:             20,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		PaidTransactionBurstLimit: BDNQuotaService{
			MsgQuota: BDNService{
				ServiceType:       BDNServiceMsgQuota,
				Limit:             50,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorAlert,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		BoostMEVSearcher: BDNBasicService{
			ExpireDate: types.ExpiredISODate,
		},
		RelayLimit: BDNQuotaService{
			MsgQuota: BDNService{
				ServiceType: BDNServicePermit,
				Limit:       2,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		MaxAllowedNodes: BDNQuotaService{
			MsgQuota: BDNService{
				ServiceType: BDNServicePermit,
				Limit:       2,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		InboundNodeConnections: BDNQuotaService{
			MsgQuota: BDNService{
				ServiceType: BDNServiceMsgQuota,
				Limit:       20,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},

		SolanaDexAPIRateLimit: BDNQuotaService{
			MsgQuota: BDNService{
				ServiceType: BDNServicePermit,
				Limit:       100,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		SolanaDexAPIStreamLimit: BDNQuotaService{
			MsgQuota: BDNService{
				ServiceType: BDNServicePermit,
				Limit:       50,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		PrivateOrdersStreaming: BDNFeedService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
			Feed: FeedProperties{
				AllowFiltering:  true,
				AvailableFields: []string{"all"},
				Plan:            SubscriptionPlanFeeds,
				Limit:           20,
			},
		},
		PendingPrivateTxsStreaming: BDNFeedService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
			Feed: FeedProperties{
				AllowFiltering:  true,
				AvailableFields: []string{"all"},
				Plan:            SubscriptionPlanFeeds,
				Limit:           20,
			},
		},
		MEVProposerGetHeaderStreaming: BDNFeedService{
			ExpireDate: types.NewISODate(now.AddDate(0, 0, 1)),
			Feed: FeedProperties{
				AllowFiltering:  true,
				AvailableFields: []string{"all"},
				Plan:            SubscriptionPlanFeeds,
				Limit:           20,
			},
		},
		SecretHash: "",
		EthValidatorGateway: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval: TimeIntervalWithout,
				Limit:        0,
			},
		},
		EthBundlePerBlock: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval: TimeIntervalWithout,
				Limit:        10000,
			},
		},

		EthBundlePerSecond: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval: TimeIntervalWithout,
				Limit:        10000,
			},
		},
		BscBundlePerBlock: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval: TimeIntervalWithout,
				Limit:        30,
			},
		},
		BscBundlePerSecond: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval: TimeIntervalWithout,
				Limit:        15,
			},
		},
		EthMempoolStreaming: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             20,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		EthBlocksStreaming: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             20,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		EthMevStreaming: BDNBasicService{
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		EthBundleSimulation: BDNBasicService{
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},

		BscMempoolStreaming: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             20,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		BscBlocksStreaming: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             20,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		BscTxReceiptsStreaming: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             20,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		BscBundleSimulation: BDNBasicService{
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		BscBigBundles: BDNBasicService{
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		BscBoosterNetwork: BDNBasicService{
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},

		BaseFlashblocksStreaming: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             3,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		BaseParsedFlashblocksStreaming: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             3,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		BaseBoosterNetwork: BDNBasicService{
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		BaseStateDiffStreaming: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             3,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},

		OnlineSolanaGateways: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             5,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		SolanaShredStreams: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             5,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		SolanaTxStreamers: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             5,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		SolanaTraderApiCredits: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             3000,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},

		TxTraceRateLimitation: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval: TimeIntervalDaily,
				ServiceType:  BDNServiceMsgQuota,
				Limit:        1,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		BundleTraceRateLimitation: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval:      TimeIntervalDaily,
				ServiceType:       BDNServiceMsgQuota,
				Limit:             60,
				BehaviorLimitOK:   BehaviorNoAction,
				BehaviorLimitFail: BehaviorNoAction,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},

		OnlineGateways: BDNQuotaService{
			MsgQuota: BDNService{
				TimeInterval: TimeIntervalDaily,
				ServiceType:  BDNServiceMsgQuota,
				Limit:        1,
			},
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		MinAllowedNodes: BDNMinAllowedNodesService{
			BDNQuotaService: BDNQuotaService{
				MsgQuota: BDNService{
					ServiceType: BDNServicePermit,
					Limit:       0,
				},
				ExpireDate: types.NewISODate(now.Add(time.Hour)),
			},
		},
		BDNPrivateRegions: BDNBasicService{
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
		EthBuilder: BDNBasicService{
			ExpireDate: types.NewISODate(now.Add(time.Hour)),
		},
	}
}
