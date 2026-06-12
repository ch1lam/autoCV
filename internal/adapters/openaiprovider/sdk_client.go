package openaiprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

type SDKClient struct {
	client openai.Client
	model  string
}

func NewSDKClient(apiKey string, baseURL string, model string) (*SDKClient, error) {
	return newSDKClient(apiKey, baseURL, model)
}

func newSDKClient(
	apiKey string,
	baseURL string,
	model string,
	extraOptions ...option.RequestOption,
) (*SDKClient, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("OpenAI API key is empty")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, errors.New("OpenAI model is empty")
	}

	options := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithMaxRetries(0),
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL != "" {
		if err := validateBaseURL(baseURL); err != nil {
			return nil, err
		}
		options = append(options, option.WithBaseURL(baseURL))
	}
	options = append(options, extraOptions...)
	return &SDKClient{
		client: openai.NewClient(options...),
		model:  model,
	}, nil
}

func (client *SDKClient) Generate(
	ctx context.Context,
	request Request,
) (Response, error) {
	schema, err := requestSchema(request.Schema)
	if err != nil {
		return Response{}, err
	}
	input, err := requestInput(request)
	if err != nil {
		return Response{}, err
	}

	format := responses.ResponseFormatTextConfigParamOfJSONSchema(
		request.SchemaName,
		schema,
	)
	format.OfJSONSchema.Strict = openai.Bool(true)
	response, err := client.client.Responses.New(
		ctx,
		responses.ResponseNewParams{
			Instructions: openai.String(request.Prompt),
			Input: responses.ResponseNewParamsInputUnion{
				OfString: openai.String(input),
			},
			Model: shared.ResponsesModel(client.model),
			Store: openai.Bool(false),
			Text: responses.ResponseTextConfigParam{
				Format: format,
			},
		},
	)
	if err != nil {
		return Response{}, classifySDKError(err)
	}
	if response.Status != responses.ResponseStatusCompleted {
		return Response{}, fmt.Errorf(
			"OpenAI response status is %q",
			response.Status,
		)
	}

	output := strings.TrimSpace(response.OutputText())
	if output == "" {
		return Response{}, errors.New("OpenAI response output is empty")
	}
	return Response{
		Output: []byte(output),
		Usage: Usage{
			InputTokens:  int(response.Usage.InputTokens),
			OutputTokens: int(response.Usage.OutputTokens),
			TotalTokens:  int(response.Usage.TotalTokens),
		},
	}, nil
}

func validateBaseURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("parse OpenAI base URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return errors.New("OpenAI base URL must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("OpenAI base URL host is empty")
	}
	return nil
}

func requestSchema(contents json.RawMessage) (map[string]any, error) {
	var schema map[string]any
	if err := json.Unmarshal(contents, &schema); err != nil {
		return nil, fmt.Errorf("decode OpenAI response schema: %w", err)
	}
	if len(schema) == 0 {
		return nil, errors.New("OpenAI response schema is empty")
	}
	return schema, nil
}

func requestInput(request Request) (string, error) {
	if !json.Valid(request.Input) {
		return "", errors.New("OpenAI request input is not valid JSON")
	}
	if request.Repair == nil {
		return string(request.Input), nil
	}

	payload := struct {
		TaskInput json.RawMessage `json:"task_input"`
		Repair    struct {
			InvalidOutput   string `json:"invalid_output"`
			ValidationError string `json:"validation_error"`
			Instruction     string `json:"instruction"`
		} `json:"repair"`
	}{
		TaskInput: request.Input,
	}
	payload.Repair.InvalidOutput = request.Repair.InvalidOutput
	payload.Repair.ValidationError = request.Repair.ValidationError
	payload.Repair.Instruction = "Return one corrected object that follows the response schema."
	contents, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode OpenAI repair input: %w", err)
	}
	return string(contents), nil
}

type sdkAPIError struct {
	statusCode int
	cause      error
}

func (err *sdkAPIError) Error() string {
	return fmt.Sprintf(
		"OpenAI API returned HTTP %d: %v",
		err.statusCode,
		err.cause,
	)
}

func (err *sdkAPIError) Unwrap() error {
	return err.cause
}

func (err *sdkAPIError) Temporary() bool {
	return err.statusCode == http.StatusRequestTimeout ||
		err.statusCode == http.StatusConflict ||
		err.statusCode == http.StatusTooManyRequests ||
		err.statusCode >= http.StatusInternalServerError
}

func classifySDKError(err error) error {
	var apiError *openai.Error
	if errors.As(err, &apiError) {
		return &sdkAPIError{
			statusCode: apiError.StatusCode,
			cause:      err,
		}
	}
	return err
}

var _ Client = (*SDKClient)(nil)
