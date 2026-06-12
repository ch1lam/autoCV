package app

import "testing"

type fakeProviderRequestController struct {
	task      string
	cancelled bool
}

func (controller fakeProviderRequestController) CancelActive() (string, bool) {
	return controller.task, controller.cancelled
}

func TestProviderControlServiceCancelsActiveRequest(t *testing.T) {
	service := NewProviderControlService(fakeProviderRequestController{
		task:      "jd_analysis",
		cancelled: true,
	})
	result := service.CancelActive()
	if !result.Cancelled ||
		result.Task != "jd_analysis" ||
		result.Message == "" {
		t.Fatalf("unexpected cancellation result %#v", result)
	}
}

func TestProviderControlServiceReportsNoActiveRequest(t *testing.T) {
	service := NewProviderControlService(fakeProviderRequestController{})
	result := service.CancelActive()
	if result.Cancelled || result.Message == "" {
		t.Fatalf("unexpected idle result %#v", result)
	}
}
