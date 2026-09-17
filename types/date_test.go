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

func TestISODateUnmarshalRejectsMalformed(t *testing.T) {
	for _, in := range []string{
		`{"expire_date":"2026-13-45"}`,
		`{"expire_date":"23/03/2076"}`,
		// RFC 3339 is deliberately not accepted: bxapi sends calendar dates, and
		// silently widening the format would hide a contract change.
		`{"expire_date":"2026-01-02T00:00:00Z"}`,
		`{"expire_date":12345}`,
	} {
		var h dateHolder
		assert.Error(t, json.Unmarshal([]byte(in), &h), in)
	}
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

	assert.True(t, time.Now().Before(date.Time))
	assert.Equal(t, soon.Format(TimeDateLayoutISO), date.Format(TimeDateLayoutISO))
}

// TestISODateAbsentKeyNormalizesOnce documents the one case that is not equal on the first
// pass: a key missing from the payload never reaches UnmarshalJSON, so the field keeps Go's
// zero time, which encodes as the sentinel and decodes back as the sentinel. Both values are
// expired, and everything is stable from the second decode on. The previous codecs had the
// same gap and additionally put "0001-01-01" on the wire.
func TestISODateAbsentKeyNormalizesOnce(t *testing.T) {
	var first dateHolder
	require.NoError(t, json.Unmarshal([]byte(`{}`), &first))
	assert.True(t, first.ExpireDate.IsZero())

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
