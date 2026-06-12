package ports

type ProviderRequestController interface {
	CancelActive() (task string, cancelled bool)
}
