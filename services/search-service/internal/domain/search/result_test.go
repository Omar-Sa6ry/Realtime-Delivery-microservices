package search

import (
	"encoding/base64"
	"testing"
)

func TestEncodeDecodeCursor_RoundTrip(t *testing.T) {
	sortValues := []interface{}{float64(1712345678000), "doc-1"}
	cursor, err := EncodeCursor(sortValues)
	if err != nil {
		t.Fatalf("EncodeCursor: %v", err)
	}
	if cursor == "" {
		t.Fatal("empty cursor")
	}
	got, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("decoded len = %d, want 2: %v", len(got), got)
	}
	if got[0] != float64(1712345678000) || got[1] != "doc-1" {
		t.Errorf("decoded = %v", got)
	}
}

func TestDecodeCursor_Empty(t *testing.T) {
	got, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := DecodeCursor("!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeCursor_InvalidJSON(t *testing.T) {
	bad := base64.StdEncoding.EncodeToString([]byte("not-json"))
	_, err := DecodeCursor(bad)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEncodeCursor_MarshalError(t *testing.T) {
	_, err := EncodeCursor([]interface{}{make(chan int)})
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestSortOrderConstants(t *testing.T) {
	if SortAsc != "asc" || SortDesc != "desc" {
		t.Errorf("sort order constants wrong: %q %q", SortAsc, SortDesc)
	}
}
