package handler

import "github.com/jackc/pgx/v5/pgtype"

// pgtypeText is a small helper for building a valid pgtype.Text in tests.
func pgtypeText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}
