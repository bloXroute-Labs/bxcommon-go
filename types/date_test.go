package types

import (
	"encoding/json"
	"fmt"
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
		zero bool
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
				b, err := json.Marshal(h)
				require.NoError(t, err)
				assert.JSONEq(t, `{"expire_date":"0001-01-01"}`, string(b))

				return
			}

			assert.Equal(t, tc.want, h.ExpireDate.Format(TimeDateLayoutISO))
			assert.Equal(t, time.UTC, h.ExpireDate.Location())
		})
	}
}

func TestISODateUnmarshalFallsBackOnMalformed(t *testing.T) {
	for _, in := range []string{
		`{"expire_date":"2026-13-45"}`,
		`{"expire_date":"23/03/2076"}`,
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

func TestParseISODateIsStrict(t *testing.T) {
	_, err := ParseISODate("2026-13-45")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidDate)

	var d ISODate
	assert.ErrorIs(t, d.UnmarshalText([]byte("nonsense")), ErrInvalidDate)

	assert.ErrorIs(t, d.UnmarshalText(nil), ErrInvalidDate)
	assert.ErrorIs(t, d.UnmarshalText([]byte("")), ErrInvalidDate)
}

func TestISODateAsJSONMapKey(t *testing.T) {
	date, err := ParseISODate("2076-03-23")
	require.NoError(t, err)

	b, err := json.Marshal(map[ISODate]int{date: 1})
	require.NoError(t, err)
	assert.JSONEq(t, `{"2076-03-23":1}`, string(b))

	var round map[ISODate]int
	require.NoError(t, json.Unmarshal(b, &round))
	assert.Equal(t, 1, round[date])

	var collapsed map[ISODate]int
	require.NoError(t, json.Unmarshal([]byte(`{"":1,"1970-01-01":2}`), &collapsed))
	assert.Len(t, collapsed, 1, "the empty key collapses onto the expired sentinel")
	assert.Equal(t, 2, collapsed[ExpiredISODate])
}

func TestISODateExpired(t *testing.T) {
	kyiv, err := time.LoadLocation("Europe/Kyiv")
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

func TestISODateString(t *testing.T) {
	date, err := ParseISODate("2026-09-17")
	require.NoError(t, err)

	assert.Equal(t, "2026-09-17", date.String())
	assert.Equal(t, "2026-09-17", fmt.Sprintf("%v", date))
	assert.Equal(t, "2026-09-17", fmt.Sprintf("%s", date))
	assert.Equal(t, "account is expired ExpireDate: 2026-09-17",
		fmt.Sprintf("account is expired ExpireDate: %v", date))

	assert.Equal(t, ExpiredDate, ExpiredISODate.String())

	var zero ISODate
	assert.Equal(t, "0001-01-01", zero.String(), "a date that was never set stays visible")
}

func TestISODateMarshal(t *testing.T) {
	date, err := ParseISODate("2076-03-23")
	require.NoError(t, err)

	b, err := json.Marshal(dateHolder{ExpireDate: date})
	require.NoError(t, err)
	assert.JSONEq(t, `{"expire_date":"2076-03-23"}`, string(b))

	b, err = json.Marshal(dateHolder{})
	require.NoError(t, err)
	assert.JSONEq(t, `{"expire_date":"0001-01-01"}`, string(b))
}

func TestISODateDecodeIsIdempotent(t *testing.T) {
	for _, in := range []string{
		`{"expire_date":"2999-01-01"}`,
		`{"expire_date":"1970-01-01"}`,
		`{"expire_date":null}`,
		`{}`,
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
		assert.Equal(t, first, second, in)
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

func TestNewISODateIsUTCDate(t *testing.T) {
	kyiv, err := time.LoadLocation("Europe/Kyiv")
	require.NoError(t, err)

	parsed, err := ParseISODate("2026-09-21")
	require.NoError(t, err)

	for _, in := range []time.Time{
		time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 21, 23, 59, 59, 999999999, time.UTC),
		time.Date(2026, 9, 22, 1, 0, 0, 0, kyiv),
		time.Date(2026, 9, 21, 2, 59, 0, 0, kyiv).Add(time.Hour),
	} {
		assert.Equal(t, parsed, NewISODate(in), in)
	}

	assert.Equal(t, NewISODate(time.Now().UTC()), NewISODate(time.Now()), "monotonic reading is dropped")
}

func TestISODateStringAgreesWithExpired(t *testing.T) {
	kyiv, err := time.LoadLocation("Europe/Kyiv")
	require.NoError(t, err)

	now := time.Date(2026, 9, 22, 0, 30, 0, 0, time.UTC)
	for _, d := range []ISODate{
		NewISODate(time.Date(2026, 9, 22, 1, 0, 0, 0, kyiv)),
		{Time: time.Date(2026, 9, 22, 1, 0, 0, 0, kyiv)},
	} {
		assert.Equal(t, "2026-09-21", d.String())

		b, err := json.Marshal(d)
		require.NoError(t, err)

		var back ISODate
		require.NoError(t, json.Unmarshal(b, &back))
		assert.Equal(t, d.expiredAt(now), back.expiredAt(now))
		assert.True(t, back.expiredAt(now))
	}
}

func TestISODateMapKeyByUTCDate(t *testing.T) {
	parsed, err := ParseISODate("2026-09-21")
	require.NoError(t, err)

	m := map[ISODate]int{
		parsed: 1,
		NewISODate(time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)):  2,
		NewISODate(time.Date(2026, 9, 21, 15, 0, 0, 0, time.UTC)): 3,
	}
	assert.Len(t, m, 1)
}
