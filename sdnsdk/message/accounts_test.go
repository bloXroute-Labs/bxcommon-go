package message

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bloXroute-Labs/bxcommon-go/types"
)

// TestDefaultEliteAccountIsGraded pins the DI-4161 contract: the fallback account served when
// the SDN cannot supply a real account model must carry an explicit grade, never the zero
// value. Consumers attach policy to grade 0 (the ungraded grade), so a fallback that looked
// ungraded would hand every account that policy for the duration of an SDN outage.
func TestDefaultEliteAccountIsGraded(t *testing.T) {
	account := GetDefaultEliteAccount(time.Now().UTC())

	assert.Equal(t, DefaultAccountGrade, account.AccountInfo.BSCGrade)
	assert.Equal(t, DefaultAccountGrade, account.AccountInfo.ETHGrade)
	assert.NotZero(t, DefaultAccountGrade)
}

// accountPayload is a trimmed bxapi response: date-only expire_date values, services the SDN
// reports as null, optional bools left null, a large quota, and the 1970 expired sentinel.
const accountPayload = `{
  "account_id": "13239207-7bec-4905-aec0-e8a00f3634ef",
  "logical_account_name": "cliff",
  "expire_date": "2072-11-01",
  "tier_name": "Introductory",
  "is_miner": null,
  "trusted": null,
  "tx_free": null,
  "tx_paid": {"expire_date": "2999-01-01", "msg_quota": {"interval": "DAILY", "service_type": "MSG_QUOTA", "limit": 10}},
  "cloud_api": {"expire_date": "2999-01-01"},
  "new_transaction_streaming": {"expire_date": "2999-01-01", "feed": {"allow_filtering": false, "available_fields": ["tx_hash"], "plan": "FEEDS", "limit": 1}},
  "new_internal_transaction_streaming": {"expire_date": "1970-01-01", "feed": null},
  "private_orders_streaming": null,
  "private_transaction_fee": {"expire_date": "2999-01-01", "msg_quota": {"interval": "WITHOUT_INTERVAL", "service_type": "MSG_QUOTA", "limit": 1579712250000000000}},
  "boost_mevsearcher": {"expire_date": "1970-01-01"},
  "secret_hash": "***"
}`

// TestAccountExpireDates covers the three service models that carry an expiry. bxapi sends
// expire_date as a calendar date, which a plain time.Time field cannot decode, and sends whole
// services as null, which must degrade to expired rather than fail the account.
func TestAccountExpireDates(t *testing.T) {
	var account Account
	require.NoError(t, json.Unmarshal([]byte(accountPayload), &account))

	// BDNFeedService
	assert.Equal(t, "2999-01-01", account.NewTransactionStreaming.ExpireDate.Format(types.TimeDateLayoutISO))
	assert.True(t, account.NewTransactionStreaming.IsActive())
	assert.Equal(t, 1, account.NewTransactionStreaming.Feed.Limit)
	assert.False(t, account.InternalTransactionMinedStreaming.IsActive())

	// BDNQuotaService
	assert.True(t, account.PaidTransactions.IsActive())
	assert.Equal(t, BDNServiceLimit(10), account.PaidTransactions.MsgQuota.Limit)
	assert.Equal(t, BDNServiceLimit(1579712250000000000), account.PrivateTransactionFee.MsgQuota.Limit)

	// bxapi sends is_miner as an optional bool; null is a no-op for a Go bool, which leaves
	// it false - the same answer Python reaches by treating None as falsy
	assert.False(t, account.Miner)

	// BDNBasicService
	assert.True(t, account.CloudAPI.IsActive())
	assert.False(t, account.BoostMEVSearcher.IsActive())

	// a service the SDN reports as null is expired, not a decode failure
	assert.True(t, account.FreeTransactions.ExpireDate.Equal(types.ExpiredISODate.Time))
	assert.False(t, account.FreeTransactions.IsActive())
	assert.False(t, account.PrivateOrdersStreaming.IsActive())

	// the account's own expiry decodes the same way
	assert.Equal(t, "2072-11-01", account.ExpireDate.Format(types.TimeDateLayoutISO))
	assert.False(t, account.IsExpired())
}

// TestBDNServiceLimitSaturates exercises the int64 boundary, which no real payload reaches:
// the largest quota the SDN has been seen to send, 1579712250000000000, is well inside int64,
// so the saturating path was never covered before. strconv returns the saturated value with
// ErrRange, and the limit has to end up at that bound rather than at zero.
func TestBDNServiceLimitSaturates(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit string
		want  BDNServiceLimit
	}{
		{name: "in range", limit: "1579712250000000000", want: 1579712250000000000},
		{name: "max int64", limit: "9223372036854775807", want: math.MaxInt64},
		{name: "one over max", limit: "9223372036854775808", want: math.MaxInt64},
		{name: "far over max", limit: "99999999999999999999999", want: math.MaxInt64},
		{name: "negative", limit: "-5", want: -5},
		{name: "min int64", limit: "-9223372036854775808", want: math.MinInt64},
		{name: "far under min", limit: "-99999999999999999999999", want: math.MinInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var account Account
			require.NoError(t, json.Unmarshal([]byte(
				`{"tx_paid":{"expire_date":"2999-01-01","msg_quota":{"limit":`+tc.limit+`}}}`), &account))
			assert.Equal(t, tc.want, account.PaidTransactions.MsgQuota.Limit)
		})
	}
}

// TestBDNServiceLimitRejectsNonInteger: only an out-of-range value is saturated; anything
// else is a parse failure, and the type assertion the old code did on that error path is gone.
func TestBDNServiceLimitRejectsNonInteger(t *testing.T) {
	for _, limit := range []string{`"10"`, `1e30`, `true`, `1.5`, `null`} {
		var account Account
		assert.Error(t, json.Unmarshal([]byte(
			`{"tx_paid":{"expire_date":"2999-01-01","msg_quota":{"limit":`+limit+`}}}`), &account), limit)
	}
}

// TestAccountMiner pins the optional bool bxapi sends as is_miner. Unlike trusted, null and
// false mean the same thing here - not a miner - so a plain bool is enough.
func TestAccountMiner(t *testing.T) {
	for _, tc := range []struct {
		payload string
		miner   bool
	}{
		{payload: `{"is_miner":null}`},
		{payload: `{"is_miner":false}`},
		{payload: `{}`},
		{payload: `{"is_miner":true}`, miner: true},
	} {
		var account Account
		require.NoError(t, json.Unmarshal([]byte(tc.payload), &account), tc.payload)
		assert.Equal(t, tc.miner, account.Miner, tc.payload)
	}

	// a miner is trusted even when trusted is explicitly false
	var minerAccount Account
	require.NoError(t, json.Unmarshal([]byte(`{"is_miner":true,"trusted":false}`), &minerAccount))
	assert.True(t, minerAccount.IsTrusted())
}

// TestAccountIsExpired covers the account's own expire_date, which bxapi always sets and
// always writes as a calendar date, using EPOCH_DATE for an account with no entitlement.
func TestAccountIsExpired(t *testing.T) {
	for _, tc := range []struct {
		expireDate string
		expired    bool
	}{
		{expireDate: "2072-11-01"},
		{expireDate: time.Now().UTC().Format(types.TimeDateLayoutISO)},
		{expireDate: time.Now().UTC().AddDate(0, 0, -1).Format(types.TimeDateLayoutISO), expired: true},
		{expireDate: types.ExpiredDate, expired: true},
	} {
		var account Account
		require.NoError(t, json.Unmarshal([]byte(`{"expire_date":"`+tc.expireDate+`"}`), &account))
		assert.Equal(t, tc.expired, account.IsExpired(), tc.expireDate)
	}

	fallback := GetDefaultEliteAccount(time.Now().UTC())
	assert.False(t, fallback.IsExpired())
}

// TestAccountExpireDateRoundTrip pins the wire format, which the SDK writes to its on-disk
// account cache and reads back: a null service comes back out as the expired sentinel, and
// decoding what we encoded lands on the same value we started from.
func TestAccountExpireDateRoundTrip(t *testing.T) {
	var account Account
	require.NoError(t, json.Unmarshal([]byte(accountPayload), &account))

	encoded, err := json.Marshal(account)
	require.NoError(t, err)

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))

	for field, want := range map[string]string{
		"tx_paid":                            `"2999-01-01"`,
		"cloud_api":                          `"2999-01-01"`,
		"new_transaction_streaming":          `"2999-01-01"`,
		"new_internal_transaction_streaming": `"1970-01-01"`,
		"tx_free":                            `"1970-01-01"`,
		"private_orders_streaming":           `"1970-01-01"`,
	} {
		var service map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(fields[field], &service), field)
		assert.Equal(t, want, string(service["expire_date"]), field)
	}

	var reread Account
	require.NoError(t, json.Unmarshal(encoded, &reread))

	// Every service the payload carries decodes to the same value the second time, which is
	// what a cached account read back from disk has to satisfy. A service the payload omits
	// entirely never reaches UnmarshalJSON, so it keeps Go's zero time on the first pass and
	// normalizes to the sentinel here; both are expired, and the wire form is stable either
	// way, which the re-encode below pins.
	for name, dates := range map[string][2]types.ISODate{
		"tx_free":                            {account.FreeTransactions.ExpireDate, reread.FreeTransactions.ExpireDate},
		"tx_paid":                            {account.PaidTransactions.ExpireDate, reread.PaidTransactions.ExpireDate},
		"cloud_api":                          {account.CloudAPI.ExpireDate, reread.CloudAPI.ExpireDate},
		"new_transaction_streaming":          {account.NewTransactionStreaming.ExpireDate, reread.NewTransactionStreaming.ExpireDate},
		"new_internal_transaction_streaming": {account.InternalTransactionMinedStreaming.ExpireDate, reread.InternalTransactionMinedStreaming.ExpireDate},
		"private_orders_streaming":           {account.PrivateOrdersStreaming.ExpireDate, reread.PrivateOrdersStreaming.ExpireDate},
		"boost_mevsearcher":                  {account.BoostMEVSearcher.ExpireDate, reread.BoostMEVSearcher.ExpireDate},
	} {
		assert.True(t, dates[1].Equal(dates[0].Time), name)
	}

	reencoded, err := json.Marshal(reread)
	require.NoError(t, err)
	assert.Equal(t, string(encoded), string(reencoded))
}

// TestAccountMalformedExpireDateIsolated mirrors bxapi, where an unreadable expire_date makes
// is_service_valid return False for that service and leaves the rest of the account loaded.
// Failing the whole json.Unmarshal instead would drop the caller onto the default elite
// account for every service because of one bad field.
func TestAccountMalformedExpireDateIsolated(t *testing.T) {
	var account Account
	require.NoError(t, json.Unmarshal([]byte(
		`{"cloud_api":{"expire_date":"2026-13-45"},"eth_builder":{"expire_date":"2999-01-01"},`+
			`"tx_paid":{"expire_date":"2999-01-01","msg_quota":{"limit":7}}}`), &account))

	assert.False(t, account.CloudAPI.IsActive(), "the unreadable date invalidates its service")
	assert.True(t, account.CloudAPI.ExpireDate.Equal(types.ExpiredISODate.Time))

	assert.True(t, account.EthBuilder.IsActive(), "sibling services are unaffected")
	assert.True(t, account.PaidTransactions.IsActive())
	assert.Equal(t, BDNServiceLimit(7), account.PaidTransactions.MsgQuota.Limit,
		"the rest of the failing service's siblings still decode")
}

// TestAccountExpiryDayIsInclusive pins the comparison against bxapi's is_service_valid,
// which is expire_date >= utcnow().date(). Comparing instants against local wall time, as
// this package used to, retired a service partway through the day it was still entitled to.
func TestAccountExpiryDayIsInclusive(t *testing.T) {
	today := time.Now().UTC().Format(types.TimeDateLayoutISO)
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format(types.TimeDateLayoutISO)

	var account Account
	require.NoError(t, json.Unmarshal([]byte(
		`{"cloud_api":{"expire_date":"`+today+`"},"eth_builder":{"expire_date":"`+yesterday+`"}}`), &account))

	assert.True(t, account.CloudAPI.IsActive(), "a service expiring today is valid all day")
	assert.False(t, account.EthBuilder.IsActive())
}

// TestAccountTrusted covers the optional bool bxapi sends as "trusted". The field used to be
// read from an "untrusted" key the SDN never sends, so it was never populated and every
// account read as trusted; null keeps that, and an explicit false now takes effect.
func TestAccountTrusted(t *testing.T) {
	var nullTrusted Account
	require.NoError(t, json.Unmarshal([]byte(accountPayload), &nullTrusted))
	assert.Nil(t, nullTrusted.Trusted)
	assert.True(t, nullTrusted.IsTrusted())

	for _, tc := range []struct {
		payload string
		trusted bool
	}{
		{payload: `{"trusted":true}`, trusted: true},
		{payload: `{"trusted":false}`, trusted: false},
		{payload: `{"trusted":false,"is_miner":true}`, trusted: true},
		{payload: `{}`, trusted: true},
	} {
		var account Account
		require.NoError(t, json.Unmarshal([]byte(tc.payload), &account))
		assert.Equal(t, tc.trusted, account.IsTrusted(), tc.payload)
	}

	fallback := GetDefaultEliteAccount(time.Now().UTC())
	assert.True(t, fallback.IsTrusted())
}

// TestAccountLogicalAccountName pins the wire key, which is logical_account_name; the struct
// used to read logical_account_id, so the field was always empty.
func TestAccountLogicalAccountName(t *testing.T) {
	var account Account
	require.NoError(t, json.Unmarshal([]byte(accountPayload), &account))

	assert.Equal(t, "cliff", account.LogicalAccountName)
}

// TestDefaultEliteAccountServicesAreActive guards the fallback account served during an SDN
// outage. CloudAPI previously set BDNBasicService.ExpireDate (a string) while IsActive read
// ExpireDateTime, so it was always inactive; one field per service removes that failure mode.
func TestDefaultEliteAccountServicesAreActive(t *testing.T) {
	account := GetDefaultEliteAccount(time.Now().UTC())

	for name, service := range map[string]ActiveService{
		"CloudAPI":                account.CloudAPI,
		"FreeTransactions":        account.FreeTransactions,
		"PaidTransactions":        account.PaidTransactions,
		"NewTransactionStreaming": account.NewTransactionStreaming,
		"MinAllowedNodes":         account.MinAllowedNodes,
		"BDNPrivateRegions":       account.BDNPrivateRegions,
		"EthBuilder":              account.EthBuilder,
	} {
		assert.True(t, service.IsActive(), name)
	}

	assert.False(t, account.BoostMEVSearcher.IsActive(), "BoostMEVSearcher is expired on purpose")
}
