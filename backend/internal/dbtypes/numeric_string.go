// Package dbtypes holds hand-written wrapper types referenced by sqlc.yaml's
// go_type overrides — sqlc itself never generates or edits this package.
package dbtypes

import (
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// NumericString wraps pgtype.Numeric so every amount_xaf column (the only
// NUMERIC columns in the schema) marshals as a JSON string instead of a JSON
// number, matching the frontend's `amount_xaf: string` contract and avoiding
// floating-point-unsafe currency values on the wire. Scan/Value are promoted
// from the embedded pgtype.Numeric, so database round-tripping and existing
// decimal-conversion helpers are unaffected.
type NumericString struct {
	pgtype.Numeric
}

func (n NumericString) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	v, err := n.Value()
	if err != nil {
		return nil, err
	}
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("dbtypes: unexpected numeric value type %T", v)
	}
	return json.Marshal(s)
}

func (n *NumericString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Numeric = pgtype.Numeric{Valid: false}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return n.Scan(s)
}
