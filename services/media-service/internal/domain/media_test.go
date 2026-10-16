package domain

import "testing"

var allMediaStatuses = []MediaStatus{
	MediaStatusPending,
	MediaStatusUploading,
	MediaStatusUploaded,
	MediaStatusScanning,
	MediaStatusProcessing,
	MediaStatusReady,
	MediaStatusFailed,
	MediaStatusQuarantined,
	MediaStatusDeleting,
	MediaStatusDeleted,
	MediaStatusAborted,
}

// expectedTransitions is an independent copy of the state machine used to
// verify every pair of the 11x11 status matrix.
var expectedTransitions = map[MediaStatus]map[MediaStatus]bool{
	MediaStatusPending: {
		MediaStatusUploading: true,
		MediaStatusFailed:    true,
		MediaStatusAborted:   true,
	},
	MediaStatusUploading: {
		MediaStatusUploaded: true,
		MediaStatusAborted:  true,
		MediaStatusFailed:   true,
	},
	MediaStatusUploaded: {
		MediaStatusScanning: true,
		MediaStatusFailed:   true,
	},
	MediaStatusScanning: {
		MediaStatusProcessing:  true,
		MediaStatusQuarantined: true,
		MediaStatusFailed:      true,
	},
	MediaStatusProcessing: {
		MediaStatusReady:  true,
		MediaStatusFailed: true,
	},
	MediaStatusReady: {
		MediaStatusDeleting: true,
	},
	MediaStatusFailed: {
		MediaStatusDeleting: true,
	},
	MediaStatusQuarantined: {
		MediaStatusDeleting: true,
	},
	MediaStatusDeleting: {
		MediaStatusDeleted: true,
		MediaStatusFailed:  true,
	},
	MediaStatusDeleted: {},
	MediaStatusAborted: {},
}

func TestMedia_CanTransitionTo_FullMatrix(t *testing.T) {
	for _, from := range allMediaStatuses {
		for _, to := range allMediaStatuses {
			t.Run(string(from)+"_to_"+string(to), func(t *testing.T) {
				m := &Media{Status: from}
				want := expectedTransitions[from][to]
				if got := m.CanTransitionTo(to); got != want {
					t.Errorf("CanTransitionTo(%s -> %s) = %v, want %v", from, to, got, want)
				}
			})
		}
	}
}

func TestMedia_CanTransitionTo_UnknownStatusRejected(t *testing.T) {
	m := &Media{Status: MediaStatus("SOMETHING_NEW")}
	for _, to := range allMediaStatuses {
		if m.CanTransitionTo(to) {
			t.Errorf("unknown status must not transition to %s", to)
		}
	}
	if m.CanTransitionTo(MediaStatus("ALSO_NEW")) {
		t.Error("unknown status must not transition to unknown status")
	}
}
