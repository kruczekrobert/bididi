package bdd

import (
	"github.com/cucumber/godog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Test FeaturesContext mock matching functionality
func TestFeaturesContext_MatchesMockCall(t *testing.T) {
	f := NewFeaturesContext(t)

	tests := []struct {
		name     string
		expected MockExpectation
		actual   MockCall
		result   bool
		err      bool
	}{
		{
			name: "exact payload match",
			expected: MockExpectation{
				Payload: map[string]interface{}{
					"name": "John",
					"age":  30,
				},
			},
			actual: MockCall{
				Payload: map[string]interface{}{
					"name": "John",
					"age":  30,
				},
			},
			result: true,
		},
		{
			name: "payload mismatch",
			expected: MockExpectation{
				Payload: map[string]interface{}{
					"name": "John",
					"age":  30,
				},
			},
			actual: MockCall{
				Payload: map[string]interface{}{
					"name": "Jane",
					"age":  float64(30),
				},
			},
			result: false,
			err:    true,
		},
		{
			name: "headers mismatch",
			expected: MockExpectation{
				Headers: map[string]string{
					"Authorization": "Bearer token123",
				},
			},
			actual: MockCall{
				Headers: map[string]string{
					"Authorization": "Bearer different_token",
				},
			},
			result: false,
			err:    true,
		},
		{
			name: "missing expected header",
			expected: MockExpectation{
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"Content-Type":  "application/json",
				},
			},
			actual: MockCall{
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					// Missing Content-Type
				},
			},
			result: false,
			err:    true,
		},
		{
			name: "both payload and headers match",
			expected: MockExpectation{
				Payload: map[string]interface{}{
					"action": "create",
				},
				Headers: map[string]string{
					"Authorization": "Bearer token123",
				},
			},
			actual: MockCall{
				Payload: map[string]interface{}{
					"action": "create",
				},
				Headers: map[string]string{
					"Authorization": "Bearer token123",
				},
			},
			result: true,
		},
		{
			name: "payload matches, headers don't",
			expected: MockExpectation{
				Payload: map[string]interface{}{
					"action": "create",
				},
				Headers: map[string]string{
					"Authorization": "Bearer token123",
				},
			},
			actual: MockCall{
				Payload: map[string]interface{}{
					"action": "create",
				},
				Headers: map[string]string{
					"Authorization": "Bearer different_token",
				},
			},
			result: false,
			err:    true,
		},
		{
			name: "empty payload expectation matches empty payload",
			expected: MockExpectation{
				Payload: map[string]interface{}{},
			},
			actual: MockCall{
				Payload: map[string]interface{}{},
			},
			result: true,
		},
		{
			name: "empty headers expectation matches empty headers",
			expected: MockExpectation{
				Headers: map[string]string{},
			},
			actual: MockCall{
				Headers: map[string]string{},
			},
			result: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.matchesMockCall(tt.expected, tt.actual)
			if tt.err {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.result, result)
		})
	}
}

// Test JSON mock creation and parsing
func TestFeaturesContext_JsonMock(t *testing.T) {
	f := NewFeaturesContext(t)

	tests := []struct {
		name        string
		mockName    string
		jsonData    string
		shouldError bool
		validate    func(t *testing.T, f *FeaturesContext)
	}{
		{
			name:     "basic mock creation",
			mockName: "test_mock",
			jsonData: `{
				"method": "GET",
				"path": "/api/test",
				"status": 200,
				"response": {
					"success": true,
					"data": "test response"
				}
			}`,
			shouldError: false,
			validate: func(t *testing.T, f *FeaturesContext) {
				mock, exists := f.mockRegistry["test_mock"]
				require.True(t, exists)
				assert.Equal(t, "GET", mock.Method)
				assert.Equal(t, "/api/test", mock.Path)
				assert.Equal(t, 200, mock.Status)
				assert.Equal(t, true, mock.Response["success"])
				assert.Equal(t, "test response", mock.Response["data"])
				assert.False(t, mock.Called)
				assert.Empty(t, mock.CallHistory)
			},
		},
		{
			name:     "mock with headers",
			mockName: "mock_with_headers",
			jsonData: `{
				"method": "POST",
				"path": "/api/users",
				"status": 201,
				"headers": {
					"Content-Type": "application/json",
					"X-Custom-Header": "custom-value"
				},
				"response": {
					"id": 123,
					"name": "John Doe"
				}
			}`,
			shouldError: false,
			validate: func(t *testing.T, f *FeaturesContext) {
				mock, exists := f.mockRegistry["mock_with_headers"]
				require.True(t, exists)
				assert.Equal(t, "POST", mock.Method)
				assert.Equal(t, "/api/users", mock.Path)
				assert.Equal(t, 201, mock.Status)
				assert.Equal(t, "application/json", mock.Headers["Content-Type"])
				assert.Equal(t, "custom-value", mock.Headers["X-Custom-Header"])
				assert.Equal(t, float64(123), mock.Response["id"])
				assert.Equal(t, "John Doe", mock.Response["name"])
			},
		},
		{
			name:     "mock with variable in path and response",
			mockName: "mock_with_variable",
			jsonData: `{
				"method": "GET",
				"path": "/api/users/$VAR(user_id)$",
				"status": 200,
				"response": {
					"id": "$VAR(user_id)$",
					"name": "Test User"
				}
			}`,
			shouldError: false,
			validate: func(t *testing.T, f *FeaturesContext) {
				mock, exists := f.mockRegistry["mock_with_variable"]
				require.True(t, exists)
				assert.Equal(t, "GET", mock.Method)
				// Path should have variables processed
				assert.Contains(t, mock.Path, "123")
				assert.Contains(t, mock.Response["id"], "123")
			},
		},
		{
			name:        "invalid JSON",
			mockName:    "invalid_mock",
			jsonData:    `{"invalid": json}`,
			shouldError: true,
			validate:    func(t *testing.T, f *FeaturesContext) {},
		},
		{
			name:        "empty mock config",
			mockName:    "empty_mock",
			jsonData:    `{}`,
			shouldError: true,
			validate:    func(t *testing.T, f *FeaturesContext) {},
		},
		{
			name:        "invalid response status",
			mockName:    "invalid_status",
			jsonData:    `{"method":"GET","path":"events","status":0}`,
			shouldError: true,
			validate:    func(t *testing.T, f *FeaturesContext) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f.wildcardProcessor.SetVariable("user_id", "123")

			docString := &godog.DocString{
				Content: tt.jsonData,
			}

			err := f.jsonMock(tt.mockName, docString)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validate(t, &f)
			}
		})
	}
}

func TestMockInvalidArguments(t *testing.T) {
	f := NewFeaturesContext(t)
	assert.Error(t, f.jsonMock("missing", nil))
	assert.Error(t, f.mockWasCalledWith("missing", nil))
	f.wildcardProcessor.SetVariable("numeric_path", 42)
	err := f.jsonMock("numeric_path", &godog.DocString{Content: `{"method":"GET","path":"$VAR(numeric_path)$","status":200}`})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock path must resolve to a string")
}

// Test mock verification methods
func TestFeaturesContext_MockVerification(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(f *FeaturesContext)
		operation   func(f *FeaturesContext) error
		shouldError bool
		errorText   string
	}{
		{
			name: "mock was called - success",
			setup: func(f *FeaturesContext) {
				f.mockRegistry = map[string]*MockConfig{
					"test_mock": {
						Called: true,
						CallHistory: []MockCall{
							{
								Method:  "GET",
								Path:    "/api/test",
								Payload: map[string]interface{}{},
								Headers: map[string]string{},
							},
						},
					},
				}
			},
			operation: func(f *FeaturesContext) error {
				return f.jsonMockWasCalled("test_mock")
			},
			shouldError: false,
		},
		{
			name: "mock was called - not called",
			setup: func(f *FeaturesContext) {
				f.mockRegistry = map[string]*MockConfig{
					"test_mock": {
						Called: false,
					},
				}
			},
			operation: func(f *FeaturesContext) error {
				return f.jsonMockWasCalled("test_mock")
			},
			shouldError: true,
			errorText:   "was not called",
		},
		{
			name: "mock was called - mock not found",
			setup: func(f *FeaturesContext) {
				f.mockRegistry = map[string]*MockConfig{}
			},
			operation: func(f *FeaturesContext) error {
				return f.jsonMockWasCalled("nonexistent_mock")
			},
			shouldError: true,
			errorText:   "no mock 'nonexistent_mock' in registry",
		},
		{
			name: "mock was not called - success",
			setup: func(f *FeaturesContext) {
				f.mockRegistry = map[string]*MockConfig{
					"test_mock": {
						Called: false,
					},
				}
			},
			operation: func(f *FeaturesContext) error {
				return f.jsonMockWasNotCalled("test_mock")
			},
			shouldError: false,
		},
		{
			name: "mock was not called - was actually called",
			setup: func(f *FeaturesContext) {
				f.mockRegistry = map[string]*MockConfig{
					"test_mock": {
						Called: true,
					},
				}
			},
			operation: func(f *FeaturesContext) error {
				return f.jsonMockWasNotCalled("test_mock")
			},
			shouldError: true,
			errorText:   "was called, but it shouldn't be called",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFeaturesContext(t)
			tt.setup(&f)

			err := tt.operation(&f)

			if tt.shouldError {
				assert.Error(t, err)
				if tt.errorText != "" {
					assert.Contains(t, err.Error(), tt.errorText)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test mock call verification with payload and headers
func TestFeaturesContext_MockWasCalledWith(t *testing.T) {
	f := NewFeaturesContext(t)

	// Set up test variables
	f.wildcardProcessor.SetVariable("user_id", "123")
	f.wildcardProcessor.SetVariable("action", "create")

	tests := []struct {
		name         string
		setup        func()
		expectedJSON string
		shouldError  bool
		errorText    string
	}{
		{
			name: "exact payload match",
			setup: func() {
				f.mockRegistry = map[string]*MockConfig{
					"test_mock": {
						Called: true,
						CallHistory: []MockCall{
							{
								Payload: map[string]interface{}{
									"name": "John",
									"age":  float64(30),
								},
								Headers: map[string]string{},
							},
						},
					},
				}
			},
			expectedJSON: `{
				"payload": {
					"name": "John",
					"age": 30
				}
			}`,
			shouldError: false,
		},
		{
			name: "payload with variables",
			setup: func() {
				f.mockRegistry = map[string]*MockConfig{
					"test_mock": {
						Called: true,
						CallHistory: []MockCall{
							{
								Payload: map[string]interface{}{
									"user_id": "123",
									"action":  "create",
								},
								Headers: map[string]string{},
							},
						},
					},
				}
			},
			expectedJSON: `{
				"payload": {
					"user_id": "$VAR(user_id)$",
					"action": "$VAR(action)$"
				}
			}`,
			shouldError: false,
		},
		{
			name: "headers match",
			setup: func() {
				f.mockRegistry = map[string]*MockConfig{
					"test_mock": {
						Called: true,
						CallHistory: []MockCall{
							{
								Payload: map[string]interface{}{},
								Headers: map[string]string{
									"Authorization": "Bearer token123",
									"Content-Type":  "application/json",
								},
							},
						},
					},
				}
			},
			expectedJSON: `{
				"headers": {
					"Authorization": "Bearer token123",
					"Content-Type": "application/json"
				}
			}`,
			shouldError: false,
		},
		{
			name: "payload and headers match",
			setup: func() {
				f.mockRegistry = map[string]*MockConfig{
					"test_mock": {
						Called: true,
						CallHistory: []MockCall{
							{
								Payload: map[string]interface{}{
									"data": "test",
								},
								Headers: map[string]string{
									"Authorization": "Bearer token123",
								},
							},
						},
					},
				}
			},
			expectedJSON: `{
				"payload": {
					"data": "test"
				},
				"headers": {
					"Authorization": "Bearer token123"
				}
			}`,
			shouldError: false,
		},
		{
			name: "payload mismatch",
			setup: func() {
				f.mockRegistry = map[string]*MockConfig{
					"test_mock": {
						Called: true,
						CallHistory: []MockCall{
							{
								Payload: map[string]interface{}{
									"name": "Jane", // Different value
								},
								Headers: map[string]string{},
							},
						},
					},
				}
			},
			expectedJSON: `{
				"payload": {
					"name": "John"
				}
			}`,
			shouldError: true,
		},
		{
			name: "mock not called",
			setup: func() {
				f.mockRegistry = map[string]*MockConfig{
					"test_mock": {
						Called: false,
					},
				}
			},
			expectedJSON: `{
				"payload": {
					"any": "value"
				}
			}`,
			shouldError: true,
			errorText:   "was not called",
		},
		{
			name: "no call history",
			setup: func() {
				f.mockRegistry = map[string]*MockConfig{
					"test_mock": {
						Called:      true,
						CallHistory: []MockCall{},
					},
				}
			},
			expectedJSON: `{
				"payload": {
					"any": "value"
				}
			}`,
			shouldError: true,
			errorText:   "has no call history",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			docString := &godog.DocString{
				Content: tt.expectedJSON,
			}

			err := f.mockWasCalledWith("test_mock", docString)

			if tt.shouldError {
				assert.Error(t, err)
				if tt.errorText != "" {
					assert.Contains(t, err.Error(), tt.errorText)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
