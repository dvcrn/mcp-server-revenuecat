package revenuecat

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// Query builds URL query parameters using RevenueCat v2 conventions (repeated
// bare keys for array parameters such as expand and aggregate).
type Query struct {
	values url.Values
}

// NewQuery returns an empty query builder.
func NewQuery() *Query {
	return &Query{values: url.Values{}}
}

// SetString sets key when val is non-empty, rejecting control characters.
func (q *Query) SetString(key, val string) error {
	if val == "" {
		return nil
	}
	if hasControl(val) {
		return fmt.Errorf("revenuecat: invalid control character in %s", key)
	}
	q.values.Set(key, val)
	return nil
}

// SetRequiredString sets key and returns an error when val is empty or whitespace.
func (q *Query) SetRequiredString(key, val string) error {
	if strings.TrimSpace(val) == "" {
		return fmt.Errorf("%s is required", key)
	}
	return q.SetString(key, val)
}

// SetInt sets key to val formatted in base 10.
func (q *Query) SetInt(key string, val int) {
	q.values.Set(key, strconv.Itoa(val))
}

// SetOptInt sets key when val is non-nil.
func (q *Query) SetOptInt(key string, val *int) {
	if val != nil {
		q.values.Set(key, strconv.Itoa(*val))
	}
}

// SetOptBool sets key to "true" or "false" when val is non-nil.
func (q *Query) SetOptBool(key string, val *bool) {
	if val != nil {
		q.values.Set(key, strconv.FormatBool(*val))
	}
}

// AddStrings appends each element as a repeated bare query parameter
// (key=a&key=b), which RevenueCat v2 uses for expand and aggregate.
func (q *Query) AddStrings(key string, vals []string) error {
	for _, v := range vals {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return fmt.Errorf("revenuecat: %s cannot contain empty values", key)
		}
		if hasControl(trimmed) {
			return fmt.Errorf("revenuecat: invalid control character in %s", key)
		}
		q.values.Add(key, trimmed)
	}
	return nil
}

// Values returns the built url.Values.
func (q *Query) Values() url.Values {
	return q.values
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f || unicode.IsControl(r) {
			return true
		}
	}
	return false
}
