package authpassword

import (
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h := NewBcryptHasher()

	hashed, err := h.Hash("mypassword")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hashed == "" {
		t.Fatal("expected non-empty hash")
	}
	if hashed == "mypassword" {
		t.Fatal("hash should not equal plain text")
	}

	if !h.Verify(hashed, "mypassword") {
		t.Fatal("expected password to verify")
	}

	if h.Verify(hashed, "wrongpassword") {
		t.Fatal("expected wrong password to not verify")
	}
}

func TestHash_DifferentEachTime(t *testing.T) {
	h := NewBcryptHasher()

	h1, _ := h.Hash("samepassword")
	h2, _ := h.Hash("samepassword")
	if h1 == h2 {
		t.Fatal("bcrypt hashes should differ due to random salt")
	}

	if !h.Verify(h1, "samepassword") {
		t.Fatal("first hash should verify")
	}
	if !h.Verify(h2, "samepassword") {
		t.Fatal("second hash should verify")
	}
}

// bcrypt rejects passwords longer than 72 bytes, exercising Hash's error path.
func TestHash_TooLongPasswordErrors(t *testing.T) {
	long := make([]byte, 100)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := NewBcryptHasher().Hash(string(long)); err == nil {
		t.Fatal("expected an error hashing a >72 byte password")
	}
}
