package app

import "github.com/ch1lam/autocv/internal/ports"

type ProviderControlService struct {
	controller ports.ProviderRequestController
}

type ProviderCancelResult struct {
	Cancelled bool   `json:"cancelled"`
	Task      string `json:"task"`
	Message   string `json:"message"`
}

func NewProviderControlService(
	controller ports.ProviderRequestController,
) *ProviderControlService {
	return &ProviderControlService{controller: controller}
}

func (service *ProviderControlService) CancelActive() ProviderCancelResult {
	task, cancelled := service.controller.CancelActive()
	if !cancelled {
		return ProviderCancelResult{
			Message: "当前没有正在运行的 Provider 请求。",
		}
	}
	return ProviderCancelResult{
		Cancelled: true,
		Task:      task,
		Message:   "已发送取消请求；当前步骤结束后可以直接重试。",
	}
}
