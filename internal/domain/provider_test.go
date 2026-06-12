package domain

import "testing"

func TestProviderConfigValidate(t *testing.T) {
	for _, test := range []struct {
		name    string
		config  ProviderConfig
		wantErr bool
	}{
		{
			name: "fake provider",
			config: ProviderConfig{
				Provider: ProviderFake,
				Model:    "fixture-v1",
			},
		},
		{
			name: "openai provider",
			config: ProviderConfig{
				Provider:  ProviderOpenAI,
				Model:     "gpt-5.5",
				SecretRef: "openai-api-key",
			},
		},
		{
			name: "unknown provider",
			config: ProviderConfig{
				Provider: "other",
				Model:    "model",
			},
			wantErr: true,
		},
		{
			name: "missing model",
			config: ProviderConfig{
				Provider: ProviderFake,
			},
			wantErr: true,
		},
		{
			name: "missing openai secret",
			config: ProviderConfig{
				Provider: ProviderOpenAI,
				Model:    "gpt-5.5",
			},
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if test.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("validate provider config: %v", err)
			}
		})
	}
}
