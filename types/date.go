package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const day = 24 * time.Hour

// ErrInvalidDate is returned by ParseISODate and UnmarshalText for a malformed date.
var ErrInvalidDate = errors.New("invalid ISO date")

// ExpiredISODate is the date bxapi sends for a service that is not provisioned, and the
// value a service reported as JSON null decodes to.
var ExpiredISODate = ISODate{Time: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)}

// ISODate is a calendar date that serializes as TimeDateLayoutISO ("2006-01-02"),
// the format bxapi uses for every expire_date in the account model.
type ISODate struct {
	time.Time
}

// NewISODate returns the UTC calendar date of t, dropping the time of day, location and
// monotonic reading, so it compares, marshals and keys a map the same as a parsed date.
func NewISODate(t time.Time) ISODate {
	return ISODate{Time: t.UTC().Truncate(day)}
}

// ParseISODate parses a "2006-01-02" date string.
func ParseISODate(s string) (ISODate, error) {
	t, err := time.Parse(TimeDateLayoutISO, s)
	if err != nil {
		return ISODate{}, fmt.Errorf("%w: parse %q as %q: %w", ErrInvalidDate, s, TimeDateLayoutISO, err)
	}

	return ISODate{Time: t}, nil
}

// Expired reports whether expire_date < today, comparing calendar dates in UTC. It is the
// negation of bxapi's is_service_valid, so the expiry day itself is still valid.
func (d ISODate) Expired() bool {
	return d.expiredAt(time.Now().UTC())
}

// UnmarshalJSON implements deserialization for ISODate. Null, an empty string and any value
// that is not a usable date yield ExpiredISODate, because bxapi treats a date it cannot read
// as an invalid service rather than an invalid account.
func (d *ISODate) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*d = ExpiredISODate
		return nil
	}

	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		*d = ExpiredISODate
		return nil
	}

	parsed, err := ParseISODate(s)
	if err != nil {
		*d = ExpiredISODate
		return nil
	}

	*d = parsed

	return nil
}

// String formats the date as TimeDateLayoutISO
func (d ISODate) String() string {
	return d.UTC().Format(TimeDateLayoutISO)
}

// MarshalJSON implements serialization for ISODate.
func (d ISODate) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// MarshalText implements encoding.TextMarshaler so that an ISODate used as a JSON map
// key, or handed to any encoder that looks for a TextMarshaler, uses the same format as
// MarshalJSON instead of the RFC 3339 one promoted from time.Time.
func (d ISODate) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler, the counterpart to MarshalText. Unlike
// UnmarshalJSON it reports every malformed value, the empty one included, since a caller that
// reaches it is decoding a lone value rather than a bxapi service model. Note that decoding a
// JSON map key does not reach this method: encoding/json prefers a json.Unmarshaler, so the
// lenient UnmarshalJSON runs there instead.
func (d *ISODate) UnmarshalText(b []byte) error {
	parsed, err := ParseISODate(string(b))
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}

// expiredAt is Expired with the clock injected, so the day boundary can be tested.
func (d ISODate) expiredAt(now time.Time) bool {
	return d.UTC().Truncate(day).Before(now.UTC().Truncate(day))
}
