package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// ExpiredISODate is the date bxapi sends for a service that is not provisioned, and the
// value a service reported as JSON null decodes to.
var ExpiredISODate = ISODate{Time: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)}

// ISODate is a calendar date that serializes as TimeDateLayoutISO ("2006-01-02"),
// the format bxapi uses for every expire_date in the account model.
type ISODate struct {
	time.Time
}

// NewISODate returns an ISODate carrying t. The time-of-day is preserved in memory
// and dropped when the value is marshalled.
func NewISODate(t time.Time) ISODate {
	return ISODate{Time: t}
}

// ParseISODate parses a "2006-01-02" date string.
func ParseISODate(s string) (ISODate, error) {
	t, err := time.Parse(TimeDateLayoutISO, s)
	if err != nil {
		return ISODate{}, fmt.Errorf("parse date %q as %q: %w", s, TimeDateLayoutISO, err)
	}

	return ISODate{Time: t}, nil
}

// UnmarshalJSON implements deserialization for ISODate. A null or empty value yields
// ExpiredISODate; anything else must be a well-formed TimeDateLayoutISO date, and a
// malformed one is reported rather than silently swallowed as the zero value.
func (d *ISODate) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*d = ExpiredISODate
		return nil
	}

	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("date must be a JSON string: %w", err)
	}

	if s == "" {
		*d = ExpiredISODate
		return nil
	}

	parsed, err := ParseISODate(s)
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}

// MarshalJSON implements serialization for ISODate. The zero value is written as
// ExpiredDate, so a service left at its zero value in Go lands on the wire as the
// expired date rather than as year 1.
func (d ISODate) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return json.Marshal(ExpiredDate)
	}

	return json.Marshal(d.Format(TimeDateLayoutISO))
}

// MarshalText implements encoding.TextMarshaler so that an ISODate used as a JSON map
// key, or handed to any encoder that looks for a TextMarshaler, uses the same format as
// MarshalJSON instead of the RFC 3339 one promoted from time.Time.
func (d ISODate) MarshalText() ([]byte, error) {
	if d.IsZero() {
		return []byte(ExpiredDate), nil
	}

	return []byte(d.Format(TimeDateLayoutISO)), nil
}

// UnmarshalText implements encoding.TextUnmarshaler, the counterpart to MarshalText.
func (d *ISODate) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		*d = ExpiredISODate
		return nil
	}

	parsed, err := ParseISODate(string(b))
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}
