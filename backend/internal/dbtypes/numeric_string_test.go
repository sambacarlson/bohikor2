package dbtypes

import (
	"encoding/json"
	"testing"
)

func TestNumericString_MarshalJSON_ValidValue(t *testing.T) {
	var n NumericString
	if err := n.Scan("15000.50"); err != nil {
		t.Fatalf("scan: %v", err)
	}

	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(b), `"15000.50"`; got != want {
		t.Fatalf("MarshalJSON() = %s, want %s", got, want)
	}
}

func TestNumericString_MarshalJSON_Invalid(t *testing.T) {
	var n NumericString // zero value: Valid = false

	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(b), `null`; got != want {
		t.Fatalf("MarshalJSON() = %s, want %s", got, want)
	}
}

func TestNumericString_MarshalJSON_InStruct(t *testing.T) {
	type wrapper struct {
		AmountXaf NumericString `json:"amount_xaf"`
	}
	var w wrapper
	if err := w.AmountXaf.Scan("-2500.00"); err != nil {
		t.Fatalf("scan: %v", err)
	}

	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(b), `{"amount_xaf":"-2500.00"}`; got != want {
		t.Fatalf("marshal struct = %s, want %s", got, want)
	}
}

func TestNumericString_UnmarshalJSON_RoundTrip(t *testing.T) {
	var n NumericString
	if err := json.Unmarshal([]byte(`"3200.75"`), &n); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(b), `"3200.75"`; got != want {
		t.Fatalf("round trip = %s, want %s", got, want)
	}
}

func TestNumericString_UnmarshalJSON_Null(t *testing.T) {
	n := NumericString{}
	if err := n.Scan("1.00"); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if err := json.Unmarshal([]byte(`null`), &n); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if n.Valid {
		t.Fatalf("expected Valid=false after unmarshaling null, got %+v", n)
	}
}
