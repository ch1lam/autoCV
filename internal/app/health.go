package app

const applicationName = "AutoCV"

type HealthStatus struct {
	Application string `json:"application"`
	Status      string `json:"status"`
}

type HealthService struct{}

func NewHealthService() *HealthService {
	return &HealthService{}
}

func (s *HealthService) Check() HealthStatus {
	return HealthStatus{
		Application: applicationName,
		Status:      "ready",
	}
}
