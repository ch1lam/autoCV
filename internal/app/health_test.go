package app

import "testing"

func TestHealthServiceCheck(t *testing.T) {
	status := NewHealthService().Check()

	if status.Application != applicationName {
		t.Fatalf("expected application %q, got %q", applicationName, status.Application)
	}
	if status.Status != "ready" {
		t.Fatalf("expected ready status, got %q", status.Status)
	}
}
