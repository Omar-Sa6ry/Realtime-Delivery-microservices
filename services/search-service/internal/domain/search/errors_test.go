package search

import (
	"errors"
	"testing"
)

func TestSentinelErrors_AreDistinctAndNonNil(t *testing.T) {
	errs := []error{
		ErrInvalidQuery,
		ErrInvalidCursor,
		ErrIndexNotFound,
		ErrDocumentNotFound,
		ErrVersionConflict,
		ErrSearchUnavailable,
		ErrUnauthorized,
		ErrReindexInProgress,
	}
	seen := map[string]bool{}
	for _, err := range errs {
		if err == nil {
			t.Fatal("nil sentinel error")
		}
		if seen[err.Error()] {
			t.Errorf("duplicate error message: %s", err.Error())
		}
		seen[err.Error()] = true
	}
	if !errors.Is(ErrSearchUnavailable, ErrSearchUnavailable) {
		t.Error("errors.Is should match sentinel to itself")
	}
}
