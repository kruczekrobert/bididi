package bdd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type MockExpectation struct {
	Status          string                 `json:"status,omitempty"`
	Payload         map[string]interface{} `json:"payload,omitempty"`
	Headers         map[string]string      `json:"headers,omitempty"`
	Query           url.Values             `json:"query,omitempty"`
	ResponseHeaders map[string]string      `json:"response_headers,omitempty"`
}

type MockCall struct {
	Method          string                 `json:"method"`
	Path            string                 `json:"path"`
	Status          int                    `json:"status"`
	Query           url.Values             `json:"query"`
	Headers         map[string]string      `json:"headers"`
	ResponseHeaders map[string]string      `json:"response_headers"`
	Payload         map[string]interface{} `json:"payload"`
	CalledAt        string                 `json:"called_at"`
}

type MockResponse struct {
	Status   int                    `json:"status"`
	Response map[string]interface{} `json:"response"`
}

type MockConfig struct {
	Called      bool                   `json:"called"`
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Response    map[string]interface{} `json:"response,omitempty"`
	Status      int                    `json:"status,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty"`
	CallHistory []MockCall             `json:"call_history,omitempty"`
}

func (f *FeaturesContext) registerMockConfig(name string, mockConfig MockConfig) error {
	if mockConfig.Method == "" || mockConfig.Path == "" {
		return fmt.Errorf("mock %q requires method and path", name)
	}
	if mockConfig.Status < 100 || mockConfig.Status > 999 {
		return fmt.Errorf("mock %q has invalid HTTP status %d", name, mockConfig.Status)
	}

	if mockConfig.Path != "" {
		processedPath, err := f.wildcardProcessor.SubstituteWildcards(mockConfig.Path)
		if err != nil {
			return fmt.Errorf("[18294295] error processing wildcards in mock path: %v", err)
		}
		path, ok := processedPath.(string)
		if !ok {
			return fmt.Errorf("mock path must resolve to a string, got %T", processedPath)
		}
		mockConfig.Path = path
	}

	if mockConfig.Response != nil {
		processedResponse, err := f.wildcardProcessor.SubstituteWildcards(mockConfig.Response)
		if err != nil {
			return fmt.Errorf("[5151515] error processing wildcards in mock response: %v", err)
		}
		mockConfig.Response = processedResponse.(map[string]interface{})
	}
	if mockConfig.Headers != nil {
		processedHeaders := make(map[string]string, len(mockConfig.Headers))
		for key, value := range mockConfig.Headers {
			processedKey, err := f.wildcardProcessor.SubstituteWildcards(key)
			if err != nil {
				return fmt.Errorf("failed to process mock response header name: %w", err)
			}
			processedValue, err := f.wildcardProcessor.SubstituteWildcards(value)
			if err != nil {
				return fmt.Errorf("failed to process mock response header value: %w", err)
			}
			name, ok := processedKey.(string)
			if !ok {
				return fmt.Errorf("mock response header name is not a string: %T", processedKey)
			}
			text, ok := processedValue.(string)
			if !ok {
				return fmt.Errorf("mock response header value is not a string: %T", processedValue)
			}
			processedHeaders[name] = text
		}
		mockConfig.Headers = processedHeaders
	}

	mockConfig.CallHistory = make([]MockCall, 0)

	f.mockRegistry[name] = &mockConfig

	return nil
}

func (f *FeaturesContext) jsonMockWasCalled(name string) error {
	if f.mockRegistry == nil {
		return fmt.Errorf("[1820392040] mock registry not initialized")
	}

	mock, ok := f.mockRegistry[name]
	if !ok {
		return fmt.Errorf("[1820392039] no mock '%s' in registry", name)
	}

	if !mock.Called {
		return fmt.Errorf("[1823172317] mock '%s' was not called", name)
	}

	return nil
}

func (f *FeaturesContext) jsonMockWasNotCalled(name string) error {
	if f.mockRegistry == nil {
		return fmt.Errorf("[1953435344] mock registry not initialized")
	}

	mock, ok := f.mockRegistry[name]
	if !ok {
		return fmt.Errorf("[1953435343] no mock '%s' in registry", name)
	}

	if mock.Called {
		return fmt.Errorf("[1953415341] mock '%s' was called, but it shouldn't be called", name)
	}
	return nil
}

func (f *FeaturesContext) assertMockCall(name string, expected MockExpectation, expectedText string) error {
	mock, ok := f.mockRegistry[name]
	if !ok {
		return fmt.Errorf("mock '%s' not found in registry", name)
	}
	if !mock.Called || len(mock.CallHistory) == 0 {
		return fmt.Errorf("mock '%s' has no call history", name)
	}
	var mismatches []string
	for i, call := range mock.CallHistory {
		matches, err := f.matchesMockCall(expected, call)
		if err != nil {
			mismatches = append(mismatches, fmt.Sprintf("Call #%d: %v", i+1, err))
			continue
		}
		if matches {
			return nil
		}
	}

	errorMsg := fmt.Sprintf("❌ No matching call found for mock '%s'\n", name)
	errorMsg += fmt.Sprintf("🎯 Expected:\n%s\n\n", f.jsonComparator.formatJSON(expectedText, "🎯"))
	errorMsg += fmt.Sprintf("📞 Actual calls (%d total):\n", len(mock.CallHistory))

	for i, call := range mock.CallHistory {
		callJSON, _ := json.Marshal(MockExpectation{
			Status:          fmt.Sprint(call.Status),
			Payload:         call.Payload,
			Headers:         call.Headers,
			Query:           call.Query,
			ResponseHeaders: call.ResponseHeaders,
		})
		errorMsg += fmt.Sprintf("   Call #%d:\n%s\n", i+1, f.jsonComparator.formatJSON(string(callJSON), "📞"))
	}
	for _, mismatch := range mismatches {
		errorMsg += mismatch + "\n"
	}

	return fmt.Errorf("%s", errorMsg)
}

func (f *FeaturesContext) matchesMockCall(expected MockExpectation, call MockCall) (bool, error) {
	if expected.Status != "" {
		if err := f.compareHTTPStatus(expected.Status, call.Status); err != nil {
			return false, fmt.Errorf("status: %w", err)
		}
	}
	if expected.Payload != nil {
		expectedJSON, err := json.Marshal(expected.Payload)
		if err != nil {
			return false, fmt.Errorf("failed to marshal expected payload: %w", err)
		}
		actualJSON, err := json.Marshal(call.Payload)
		if err != nil {
			return false, fmt.Errorf("failed to marshal actual payload: %w", err)
		}
		if err := f.jsonComparator.CompareJSON(string(expectedJSON), string(actualJSON)); err != nil {
			return false, fmt.Errorf("payload: %w", err)
		}
	}

	for key, expectedValue := range expected.Headers {
		actualValue, ok := call.Headers[http.CanonicalHeaderKey(key)]
		if !ok {
			return false, fmt.Errorf("missing header %q", key)
		}
		if err := f.compareHTTPText(expectedValue, actualValue); err != nil {
			return false, fmt.Errorf("header %q: %w", key, err)
		}
	}
	for key, values := range expected.Query {
		actualValues := call.Query[key]
		if len(actualValues) < len(values) {
			return false, fmt.Errorf("missing query value for %q", key)
		}
		for i, value := range values {
			if err := f.compareHTTPText(value, actualValues[i]); err != nil {
				return false, fmt.Errorf("query %q: %w", key, err)
			}
		}
	}
	for key, expectedValue := range expected.ResponseHeaders {
		actualValue, ok := call.ResponseHeaders[http.CanonicalHeaderKey(key)]
		if !ok {
			return false, fmt.Errorf("missing response header %q", key)
		}
		if err := f.compareHTTPText(expectedValue, actualValue); err != nil {
			return false, fmt.Errorf("response header %q: %w", key, err)
		}
	}

	return true, nil
}
