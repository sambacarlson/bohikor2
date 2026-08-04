package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

func TestListRequestsNeedingReview_Success(t *testing.T) {
	store := &mockPlatformStore{
		needsReview: []db.ListRequestsNeedingReviewAcrossCompaniesRow{
			{ID: uuid.New(), CompanySlug: "acme", CompanyName: "Acme", UserEmail: "e@acme.com"},
		},
	}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/requests/needs-review", h.ListRequestsNeedingReview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/requests/needs-review", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := mustUnmarshalDataArray(t, w.Body.Bytes())
	if len(data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(data))
	}
	row := data[0].(map[string]interface{})
	if row["company_slug"] != "acme" {
		t.Fatalf("expected company_slug acme, got %v", row["company_slug"])
	}
}

func TestListRequestsNeedingReview_Empty(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/requests/needs-review", h.ListRequestsNeedingReview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/requests/needs-review", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if data := mustUnmarshalDataArray(t, w.Body.Bytes()); len(data) != 0 {
		t.Fatalf("expected empty array, got %d", len(data))
	}
}

func TestListRequestsNeedingReview_Error(t *testing.T) {
	store := &mockPlatformStore{needsReviewErr: errors.New("boom")}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/requests/needs-review", h.ListRequestsNeedingReview)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/requests/needs-review", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestRequestsHealth_Success(t *testing.T) {
	store := &mockPlatformStore{
		health: []db.ListCompanyRequestHealthRow{
			{CompanyID: uuid.New(), CompanySlug: "acme", CompanyName: "Acme", ProcessingCount: 2, PendingCount: 1, NeedsReviewCount: 3},
		},
	}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/requests/health", h.RequestsHealth)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/requests/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := mustUnmarshalDataArray(t, w.Body.Bytes())
	if len(data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(data))
	}
	row := data[0].(map[string]interface{})
	if row["needs_review_count"].(float64) != 3 {
		t.Fatalf("expected needs_review_count 3, got %v", row["needs_review_count"])
	}
}

func TestRequestsHealth_Empty(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/requests/health", h.RequestsHealth)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/requests/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if data := mustUnmarshalDataArray(t, w.Body.Bytes()); len(data) != 0 {
		t.Fatalf("expected empty array, got %d", len(data))
	}
}

func TestRequestsHealth_Error(t *testing.T) {
	store := &mockPlatformStore{healthErr: errors.New("boom")}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/requests/health", h.RequestsHealth)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/requests/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
