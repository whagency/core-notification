package domain

import "testing"

func TestIDHelpers(t *testing.T) {
	t.Parallel()

	id, err := NewID(uint64(42))
	if err != nil {
		t.Fatalf("NewID(uint64) returned error: %v", err)
	}
	if id.Uint64() != 42 || id.String() != "42" {
		t.Fatalf("unexpected id conversions: uint=%d string=%q", id.Uint64(), id.String())
	}

	stringID, err := NewID("77")
	if err != nil {
		t.Fatalf("NewID(string) returned error: %v", err)
	}
	if stringID != NewIDFromUint64(77) {
		t.Fatalf("string id = %d; want 77", stringID)
	}

	parsed, err := NewIDFromString("99")
	if err != nil {
		t.Fatalf("NewIDFromString returned error: %v", err)
	}
	if parsed.Uint64() != 99 {
		t.Fatalf("parsed id = %d", parsed)
	}

	if _, err := NewIDFromString("abc"); err == nil {
		t.Fatal("expected invalid string error")
	}
}

func TestNewNullableIDFromUint64(t *testing.T) {
	t.Parallel()

	if got := NewNullableIDFromUint64(nil); got != nil {
		t.Fatalf("nil id = %#v", got)
	}

	value := uint64(42)
	got := NewNullableIDFromUint64(&value)
	if got == nil {
		t.Fatal("expected id pointer")
	}
	if *got != NewIDFromUint64(42) {
		t.Fatalf("id = %d; want 42", *got)
	}
}
