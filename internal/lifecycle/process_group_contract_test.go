package lifecycle

import (
	"errors"
	"testing"
)

func TestProcessGroupPIDError(t *testing.T) {
	for _, tc := range []struct {
		name      string
		pid       int
		supported bool
		wantErr   bool
	}{
		{name: "no PID is harmless", pid: 0, supported: false},
		{name: "negative PID is harmless", pid: -1, supported: false},
		{name: "supported positive PID", pid: 42, supported: true},
		{name: "unsupported positive PID", pid: 42, supported: false, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := processGroupPIDError(tc.pid, tc.supported)
			if got := err != nil; got != tc.wantErr {
				t.Fatalf("processGroupPIDError(%d, %v) error = %v, want error=%v", tc.pid, tc.supported, err, tc.wantErr)
			}
			if tc.wantErr && !errors.Is(err, errProcessGroupsUnsupported) {
				t.Fatalf("error = %v, want errProcessGroupsUnsupported", err)
			}
		})
	}
}

func TestSignalableProcessGroupPID(t *testing.T) {
	for _, pid := range []int{-1, 0} {
		if signalableProcessGroupPID(pid) {
			t.Fatalf("pid %d is signalable", pid)
		}
	}
	if !signalableProcessGroupPID(1) {
		t.Fatal("positive PID is not signalable")
	}
}
