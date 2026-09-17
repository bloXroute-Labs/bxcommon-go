package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dateHolder struct {
	ExpireDate ISODate `json:"expire_date"`
}

func TestISODateUnmarshal(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string // TimeDateLayoutISO
		zero bool   // the key never reached UnmarshalJSON, so the field stays at its zero value
	}{
		{name: "date", in: `{"expire_date":"2076-03-23"}`, want: "2076-03-23"},
		{name: "expired sentinel", in: `{"expire_date":"1970-01-01"}`, want: ExpiredDate},
		{name: "null", in: `{"expire_date":null}`, want: ExpiredDate},
		{name: "empty string", in: `{"expire_date":""}`, want: ExpiredDate},
		{name: "absent", in: `{}`, want: ExpiredDate, zero: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var h dateHolder
			require.NoError(t, json.Unmarshal([]byte(tc.in), &h))

			assert.Equal(t, tc.zero, h.ExpireDate.IsZero())

			if tc.zero {
				// the zero value is expired too, and reaches the wire as the sentinel
				b, err := json.Marshal(h)
				require.NoError(t, err)
				assert.JSONEq(t, `{"expire_date":"1970-01-01"}`, string(b))

				return
			}

			assert.Equal(t, tc.want, h.ExpireDate.Format(TimeDateLayoutISO))
			assert.Equal(t, time.UTC, h.ExpireDate.Location())
		})
	}
}

// TestISODateUnmarshalFallsBackOnMalformed pins bxapi's own handling: its model loader keeps
// expire_date as a string and only parses it when asked whether the service is valid, where
// an unreadable date returns False. So a bad date has to invalidate its own service, not fail
// the account it arrived in.
func TestISODateUnmarshalFallsBackOnMalformed(t *testing.T) {
	for _, in := range []string{
		`{"expire_date":"2026-13-45"}`,
		`{"expire_date":"23/03/2076"}`,
		// RFC 3339 is not accepted either: bxapi writes expire_date with date.isoformat(),
		// so a timestamp would mean the contract changed, not that this service is current.
		`{"expire_date":"2026-01-02T00:00:00Z"}`,
		`{"expire_date":12345}`,
		`{"expire_date":{"nested":true}}`,
	} {
		var h dateHolder
		require.NoError(t, json.Unmarshal([]byte(in), &h), in)
		assert.True(t, h.ExpireDate.Equal(ExpiredISODate.Time), in)
		assert.True(t, h.ExpireDate.Expired(), in)
	}
}

// TestParseISODateIsStrict: the explicit parse path still reports a bad date, so callers that
// want to validate rather than degrade have a way to.
func TestParseISODateIsStrict(t *testing.T) {
	_, err := ParseISODate("2026-13-45")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidDate)

	var d ISODate
	assert.ErrorIs(t, d.UnmarshalText([]byte("nonsense")), ErrInvalidDate)
}

// TestISODateExpired pins the comparison against bxapi's is_service_valid, which is
// `expire_date >= datetime.utcnow().date()`: calendar dates in UTC, expiry day inclusive.
func TestISODateExpired(t *testing.T) {
	kyiv, err := time.LoadLocation("Europe/Kiev")
	require.NoError(t, err)

	expire, err := ParseISODate("2026-09-17")
	require.NoError(t, err)

	for _, tc := range []struct {
		name    string
		now     time.Time
		expired bool
	}{
		{name: "day before", now: time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)},
		{name: "expiry day, start", now: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)},
		{name: "expiry day, end", now: time.Date(2026, 9, 17, 23, 59, 59, 0, time.UTC)},
		// the case the old instant comparison got wrong: past 03:00 in Kyiv the wall clock
		// is ahead of UTC midnight, and the service was retired most of a day early
		{name: "expiry day, Kyiv morning", now: time.Date(2026, 9, 17, 4, 0, 0, 0, kyiv)},
		{name: "expiry day, Kyiv evening", now: time.Date(2026, 9, 17, 23, 0, 0, 0, kyiv)},
		{name: "day after", now: time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), expired: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expired, expire.expiredAt(tc.now))
		})
	}

	assert.True(t, ExpiredISODate.Expired())
	assert.True(t, ISODate{}.Expired())
	assert.False(t, NewISODate(time.Now().UTC()).Expired(), "a service expiring today is still valid")
	assert.False(t, NewISODate(time.Now().Add(time.Hour)).Expired())
}

func TestISODateMarshal(t *testing.T) {
	date, err := ParseISODate("2076-03-23")
	require.NoError(t, err)

	b, err := json.Marshal(dateHolder{ExpireDate: date})
	require.NoError(t, err)
	assert.JSONEq(t, `{"expire_date":"2076-03-23"}`, string(b))

	b, err = json.Marshal(dateHolder{})
	require.NoError(t, err)
	assert.JSONEq(t, `{"expire_date":"1970-01-01"}`, string(b))
}

// TestISODateDecodeIsIdempotent pins the property the SDK's on-disk account cache relies
// on: decoding a value, re-encoding it and decoding it again must land on the same value.
// Decoding null to the zero time instead of the sentinel would break this.
func TestISODateDecodeIsIdempotent(t *testing.T) {
	for _, in := range []string{
		`{"expire_date":"2999-01-01"}`,
		`{"expire_date":"1970-01-01"}`,
		`{"expire_date":null}`,
	} {
		var first dateHolder
		require.NoError(t, json.Unmarshal([]byte(in), &first))

		encoded, err := json.Marshal(first)
		require.NoError(t, err)

		var second dateHolder
		require.NoError(t, json.Unmarshal(encoded, &second))

		reencoded, err := json.Marshal(second)
		require.NoError(t, err)

		assert.Equal(t, string(encoded), string(reencoded), in)
		assert.True(t, second.ExpireDate.Equal(first.ExpireDate.Time), in)
	}
}

func TestISODateRoundTrip(t *testing.T) {
	for _, in := range []string{
		`{"expire_date":"2999-01-01"}`,
		`{"expire_date":"1970-01-01"}`,
	} {
		var h dateHolder
		require.NoError(t, json.Unmarshal([]byte(in), &h))

		b, err := json.Marshal(h)
		require.NoError(t, err)
		assert.JSONEq(t, in, string(b))
	}
}

// TestISODateTextCodec guards against the RFC 3339 MarshalText promoted from the embedded
// time.Time leaking into contexts that use a TextMarshaler rather than a JSON one, such as
// a map key, where the same value would otherwise serialize two different ways.
func TestISODateTextCodec(t *testing.T) {
	date, err := ParseISODate("2076-03-23")
	require.NoError(t, err)

	b, err := json.Marshal(map[ISODate]int{date: 1})
	require.NoError(t, err)
	assert.JSONEq(t, `{"2076-03-23":1}`, string(b))

	var decoded map[ISODate]int
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, 1, decoded[date])
}

func TestNewISODateKeepsTimeOfDay(t *testing.T) {
	// Only the wire format is date-granular. A service expiring later today must still
	// read as unexpired in memory, which is what the in-process defaults rely on.
	soon := time.Now().Add(time.Hour)
	date := NewISODate(soon)

	assert.False(t, date.Expired())
	assert.Equal(t, soon.Format(TimeDateLayoutISO), date.Format(TimeDateLayoutISO))
}

// TestISODateAbsentKeyNormalizesOnce documents the one case that is not equal on the first
// pass: a key missing from the payload never reaches UnmarshalJSON, so the field keeps Go's
// zero time, which encodes as the sentinel and decodes back as the sentinel. Both values are
// expired - and bxapi has no zero-date state at all, its loader leaves an absent service as
// None - so this is cosmetic, and everything is stable from the second decode on.
func TestISODateAbsentKeyNormalizesOnce(t *testing.T) {
	var first dateHolder
	require.NoError(t, json.Unmarshal([]byte(`{}`), &first))
	assert.True(t, first.ExpireDate.IsZero())
	assert.True(t, first.ExpireDate.Expired())

	encoded, err := json.Marshal(first)
	require.NoError(t, err)
	assert.JSONEq(t, `{"expire_date":"1970-01-01"}`, string(encoded))

	var second dateHolder
	require.NoError(t, json.Unmarshal(encoded, &second))
	assert.True(t, second.ExpireDate.Equal(ExpiredISODate.Time))

	reencoded, err := json.Marshal(second)
	require.NoError(t, err)

	var third dateHolder
	require.NoError(t, json.Unmarshal(reencoded, &third))
	assert.Equal(t, second, third)
}
