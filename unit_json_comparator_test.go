package bdd

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Test JSONComparator functionality
func TestJSONComparator_CompareJSON(t *testing.T) {
	processor := NewWildcardProcessor()

	// Set up test variables
	processor.SetVariable("user_id", "123")
	processor.SetVariable("price", 99.99)
	processor.SetVariable("enabled", true)
	processor.SetVariable("count", 42)

	comparator := NewJSONComparator(processor)

	tests := []struct {
		name        string
		expected    string
		actual      string
		shouldError bool
		errorText   string
	}{
		{
			name:        "exact JSON match",
			expected:    `{"name": "test", "value": 123}`,
			actual:      `{"name": "test", "value": 123}`,
			shouldError: false,
		},
		{
			name:        "JSON with variable substitution",
			expected:    `{"user_id": "$VAR(user_id)$", "price": "$VAR(price)$"}`,
			actual:      `{"user_id": "123", "price": 99.99}`,
			shouldError: false,
		},
		{
			name:        "JSON with boolean variable",
			expected:    `{"enabled": "$VAR(enabled)$", "count": "$VAR(count)$"}`,
			actual:      `{"enabled": true, "count": 42}`,
			shouldError: false,
		},
		{
			name:        "JSON with UUID wildcard",
			expected:    `{"id": "$UUID$", "name": "test"}`,
			actual:      `{"id": "550e8400-e29b-41d4-a716-446655440000", "name": "test"}`,
			shouldError: false,
		},
		{
			name:        "JSON with uppercase UUID wildcard",
			expected:    `{"id": "$UUID$", "name": "test"}`,
			actual:      `{"id": "550E8400-E29B-41D4-A716-446655440000", "name": "test"}`,
			shouldError: false,
		},
		{
			name:        "JSON with invalid UUID",
			expected:    `{"id": "$UUID$"}`,
			actual:      `{"id": "invalid-uuid"}`,
			shouldError: true,
			errorText:   "invalid UUID format",
		},
		{
			name:        "JSON with NOW wildcard",
			expected:    `{"timestamp": "$NOW$"}`,
			actual:      `{"timestamp": "` + time.Now().Format(time.RFC3339Nano) + `"}`,
			shouldError: false,
		},
		{
			name:        "JSON with NOW plus duration and local timestamp",
			expected:    `{"timestamp": "$NOW+1h$"}`,
			actual:      `{"timestamp": "` + time.Now().Add(time.Hour).Round(time.Second).Format("2006-01-02 15:04:05") + `"}`,
			shouldError: false,
		},
		{
			name:        "JSON with ANY wildcards",
			expected:    `{"any_field": "$ANY$", "any_string": "$ANY_STR$", "any_int": "$ANY_INT$"}`,
			actual:      `{"any_field": "whatever", "any_string": "hello", "any_int": 456}`,
			shouldError: false,
		},
		{
			name:        "JSON with ANY_INT mismatch",
			expected:    `{"number": "$ANY_INT$"}`,
			actual:      `{"number": "not_a_number"}`,
			shouldError: true,
			errorText:   "expected integer",
		},
		{
			name:        "JSON with ANY_FLOAT validation",
			expected:    `{"value": "$ANY_FLOAT$"}`,
			actual:      `{"value": 3.14}`,
			shouldError: false,
		},
		{
			name:        "JSON with ANY_BOOL validation",
			expected:    `{"flag": "$ANY_BOOL$"}`,
			actual:      `{"flag": true}`,
			shouldError: false,
		},
		{
			name:        "JSON array comparison",
			expected:    `[{"id": "$UUID$"}, {"value": "$ANY_INT$"}]`,
			actual:      `[{"id": "550e8400-e29b-41d4-a716-446655440000"}, {"value": 123}]`,
			shouldError: false,
		},
		{
			name:        "JSON array size mismatch",
			expected:    `[1, 2, 3]`,
			actual:      `[1, 2]`,
			shouldError: true,
			errorText:   "array size mismatch",
		},
		{
			name:        "Missing field in actual",
			expected:    `{"name": "test", "age": 25}`,
			actual:      `{"name": "test"}`,
			shouldError: true,
			errorText:   "missing required field",
		},
		{
			name:        "Extra field in actual",
			expected:    `{"name": "test"}`,
			actual:      `{"name": "test", "extra": "field"}`,
			shouldError: true,
			errorText:   "unexpected extra field",
		},
		{
			name:        "Type mismatch",
			expected:    `{"value": 123}`,
			actual:      `{"value": "123"}`,
			shouldError: true,
			errorText:   "type mismatch",
		},
		{
			name:        "Nested object comparison",
			expected:    `{"user": {"id": "$VAR(user_id)$", "active": "$VAR(enabled)$"}}`,
			actual:      `{"user": {"id": "123", "active": true}}`,
			shouldError: false,
		},
		{
			name: "Complex nested structure with wildcards",
			expected: `{
				"batch_id": "$UUID$",
				"user_id": "$VAR(user_id)$",
				"processed_at": "$NOW$",
				"items": [
					{"id": "$UUID$", "amount": "$ANY_FLOAT$"},
					{"id": "$UUID$", "amount": "$ANY_FLOAT$"}
				],
				"summary": {
					"total": "$ANY_FLOAT$",
					"count": "$VAR(count)$"
				}
			}`,
			actual: `{
				"batch_id": "550e8400-e29b-41d4-a716-446655440000",
				"user_id": "123",
				"processed_at": "` + time.Now().Format(time.RFC3339Nano) + `",
				"items": [
					{"id": "550e8400-e29b-41d4-a716-446655440001", "amount": 100.50},
					{"id": "550e8400-e29b-41d4-a716-446655440002", "amount": 200.75}
				],
				"summary": {
					"total": 301.25,
					"count": 42
				}
			}`,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := comparator.CompareJSON(tt.expected, tt.actual)

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

// Test ExtractValueFromJSON functionality
func TestJSONComparator_ExtractValueFromJSON(t *testing.T) {
	comparator := NewJSONComparator(NewWildcardProcessor())

	jsonStr := `{
		"user": {
			"id": "123",
			"profile": {
				"name": "John Doe",
				"age": 30,
				"active": true
			},
			"scores": [10, 20, 30.5]
		},
		"metadata": {
			"version": "1.0",
			"tags": ["test", "production"]
		}
	}`

	tests := []struct {
		name        string
		path        string
		expected    interface{}
		shouldError bool
	}{
		{
			name:        "extract root string",
			path:        "user.id",
			expected:    "123",
			shouldError: false,
		},
		{
			name:        "extract nested string",
			path:        "user.profile.name",
			expected:    "John Doe",
			shouldError: false,
		},
		{
			name:        "extract nested number",
			path:        "user.profile.age",
			expected:    float64(30), // JSON numbers are float64
			shouldError: false,
		},
		{
			name:        "extract nested boolean",
			path:        "user.profile.active",
			expected:    true,
			shouldError: false,
		},
		{
			name:        "extract array element",
			path:        "user.scores.0",
			expected:    float64(10),
			shouldError: false,
		},
		{
			name:        "extract array element float",
			path:        "user.scores.2",
			expected:    30.5,
			shouldError: false,
		},
		{
			name:        "extract from nested array",
			path:        "metadata.tags.1",
			expected:    "production",
			shouldError: false,
		},
		{
			name: "extract entire object",
			path: "metadata",
			expected: map[string]interface{}{
				"version": "1.0",
				"tags":    []interface{}{"test", "production"},
			},
			shouldError: false,
		},
		{
			name:        "extract entire array",
			path:        "user.scores",
			expected:    []interface{}{float64(10), float64(20), 30.5},
			shouldError: false,
		},
		{
			name:        "non-existent path",
			path:        "user.nonexistent",
			expected:    nil,
			shouldError: true,
		},
		{
			name:        "invalid array index",
			path:        "user.scores.10",
			expected:    nil,
			shouldError: true,
		},
		{
			name:        "invalid array index format",
			path:        "user.scores.abc",
			expected:    nil,
			shouldError: true,
		},
		{
			name:        "access field on primitive",
			path:        "user.id.nonexistent",
			expected:    nil,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := comparator.ExtractValueFromJSON(jsonStr, tt.path)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// Test ProcessWildcardsInJSONString functionality
func TestJSONComparator_ProcessWildcardsInJSONString(t *testing.T) {
	processor := NewWildcardProcessor()

	// Set up test variables
	processor.SetVariable("user_id", "user123")
	processor.SetVariable("price", 99.99)
	processor.SetVariable("count", 42)
	processor.SetVariable("enabled", true)

	comparator := NewJSONComparator(processor)

	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, result string, err error)
	}{
		{
			name:  "string variable substitution",
			input: `{"user_id": "$VAR(user_id)$", "name": "test"}`,
			validate: func(t *testing.T, result string, err error) {
				assert.NoError(t, err)
				expected := `{"user_id": "user123", "name": "test"}`

				// Parse both to compare structure
				var expectedObj, resultObj map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(expected), &expectedObj))
				require.NoError(t, json.Unmarshal([]byte(result), &resultObj))
				assert.Equal(t, expectedObj, resultObj)
			},
		},
		{
			name:  "numeric variable substitution",
			input: `{"price": "$VAR(price)$", "count": "$VAR(count)$"}`,
			validate: func(t *testing.T, result string, err error) {
				assert.NoError(t, err)

				// Should preserve numeric types without quotes
				assert.Contains(t, result, `"price": 99.99`)
				assert.Contains(t, result, `"count": 42`)

				// Validate it's valid JSON
				var resultObj map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(result), &resultObj))
				assert.Equal(t, 99.99, resultObj["price"])
				assert.Equal(t, float64(42), resultObj["count"]) // JSON numbers are float64
			},
		},
		{
			name:  "mixed wildcards",
			input: `{"id": "$UUID$", "user": "$VAR(user_id)$", "timestamp": "$NOW$"}`,
			validate: func(t *testing.T, result string, err error) {
				assert.NoError(t, err)

				// Parse to validate structure
				var resultObj map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(result), &resultObj))

				// UUID should remain as wildcard (comparison context)
				assert.Equal(t, "$UUID$", resultObj["id"])

				// Variable should be substituted
				assert.Equal(t, "user123", resultObj["user"])

				// NOW should remain as wildcard
				assert.Equal(t, "$NOW$", resultObj["timestamp"])
			},
		},
		{
			name:  "no wildcards",
			input: `{"name": "test", "value": 123}`,
			validate: func(t *testing.T, result string, err error) {
				assert.NoError(t, err)
				assert.Equal(t, `{"name": "test", "value": 123}`, result)
			},
		},
		{
			name:  "array with wildcards",
			input: `[{"id": "$VAR(user_id)$"}, {"count": "$VAR(count)$"}]`,
			validate: func(t *testing.T, result string, err error) {
				assert.NoError(t, err)

				var resultArray []map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(result), &resultArray))

				assert.Equal(t, "user123", resultArray[0]["id"])
				assert.Equal(t, float64(42), resultArray[1]["count"])
			},
		},
		{
			name:  "nested object with wildcards",
			input: `{"user": {"id": "$VAR(user_id)$", "profile": {"enabled": "$VAR(enabled)$"}}}`,
			validate: func(t *testing.T, result string, err error) {
				assert.NoError(t, err)

				var resultObj map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(result), &resultObj))

				userObj := resultObj["user"].(map[string]interface{})
				assert.Equal(t, "user123", userObj["id"])

				profileObj := userObj["profile"].(map[string]interface{})
				assert.Equal(t, true, profileObj["enabled"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := comparator.ProcessWildcardsInJSONString(tt.input)
			tt.validate(t, result, err)
		})
	}
}

// Test wildcard comparison edge cases
func TestJSONComparator_WildcardEdgeCases(t *testing.T) {
	processor := NewWildcardProcessor()
	comparator := NewJSONComparator(processor)

	tests := []struct {
		name        string
		expected    string
		actual      string
		shouldError bool
		errorText   string
	}{
		{
			name:        "ANY_STR_FULL with empty string",
			expected:    `{"name": "$ANY_STR_FULL$"}`,
			actual:      `{"name": ""}`,
			shouldError: true,
			errorText:   "expected non-empty string",
		},
		{
			name:        "ANY_STR_FULL with non-empty string",
			expected:    `{"name": "$ANY_STR_FULL$"}`,
			actual:      `{"name": "John"}`,
			shouldError: false,
		},
		{
			name:        "ANY_INT with float",
			expected:    `{"value": "$ANY_INT$"}`,
			actual:      `{"value": 3.14}`,
			shouldError: true,
			errorText:   "expected integer",
		},
		{
			name:        "ANY_INT with whole number float",
			expected:    `{"value": "$ANY_INT$"}`,
			actual:      `{"value": 42.0}`,
			shouldError: false, // Should accept whole number floats
		},
		{
			name:        "ANY_FLOAT with integer",
			expected:    `{"value": "$ANY_FLOAT$"}`,
			actual:      `{"value": 42}`,
			shouldError: false, // Should accept integers as numeric
		},
		{
			name:        "NOW with old timestamp",
			expected:    `{"timestamp": "$NOW$"}`,
			actual:      `{"timestamp": "2020-01-01T00:00:00Z"}`,
			shouldError: true,
			errorText:   "time mismatch",
		},
		{
			name:        "NOW with invalid time format",
			expected:    `{"timestamp": "$NOW$"}`,
			actual:      `{"timestamp": "not-a-time"}`,
			shouldError: true,
			errorText:   "invalid time format",
		},
		{
			name:        "NOW plus duration with RFC3339Nano timestamp",
			expected:    `{"timestamp": "$NOW+1h$"}`,
			actual:      `{"timestamp": "` + time.Now().Add(time.Hour).Format(time.RFC3339Nano) + `"}`,
			shouldError: false,
		},
		{
			name:        "UUID with invalid format",
			expected:    `{"id": "$UUID$"}`,
			actual:      `{"id": "not-a-uuid"}`,
			shouldError: true,
			errorText:   "invalid UUID format",
		},
		{
			name:        "Empty objects",
			expected:    `{}`,
			actual:      `{}`,
			shouldError: false,
		},
		{
			name:        "Empty arrays",
			expected:    `[]`,
			actual:      `[]`,
			shouldError: false,
		},
		{
			name:        "Null values",
			expected:    `{"value": null}`,
			actual:      `{"value": null}`,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := comparator.CompareJSON(tt.expected, tt.actual)

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

func TestJSONComparator_NotNullAnyArrayAndNowWithinWildcards(t *testing.T) {
	comparator := NewJSONComparator(NewWildcardProcessor())

	require.NoError(t, comparator.CompareJSON(
		`{"value":"$NOT_NULL$"}`,
		`{"value":{"nested":true}}`,
	))
	require.ErrorContains(t, comparator.CompareJSON(
		`{"value":"$NOT_NULL$"}`,
		`{"value":null}`,
	), "expected non-null value")

	require.NoError(t, comparator.CompareJSON(
		`{"value":"$ANY_ARRAY$"}`,
		`{"value":[1,"two",null]}`,
	))
	require.ErrorContains(t, comparator.CompareJSON(
		`{"value":"$ANY_ARRAY$"}`,
		`{"value":{"not":"an array"}}`,
	), "expected array")

	// $ANY$ already accepts arrays and every other JSON type, including null.
	require.NoError(t, comparator.CompareJSON(
		`{"value":"$ANY$"}`,
		`{"value":[1,2,3]}`,
	))

	require.NoError(t, comparator.CompareJSON(
		`{"created_at":"$NOW_WITHIN(2s)$"}`,
		`{"created_at":"`+time.Now().Add(-500*time.Millisecond).Format(time.RFC3339Nano)+`"}`,
	))
	require.ErrorContains(t, comparator.CompareJSON(
		`{"created_at":"$NOW_WITHIN(100ms)$"}`,
		`{"created_at":"`+time.Now().Add(-time.Second).Format(time.RFC3339Nano)+`"}`,
	), "time mismatch")
	require.ErrorContains(t, comparator.CompareJSON(
		`{"created_at":"$NOW_WITHIN(invalid)$"}`,
		`{"created_at":"`+time.Now().Format(time.RFC3339Nano)+`"}`,
	), "invalid time tolerance")

	require.NoError(t, comparator.CompareJSON(
		`{"scheduled_at":"$NOW+1h_WITHIN(10s)$"}`,
		`{"scheduled_at":"`+time.Now().Add(time.Hour+9*time.Second).Format(time.RFC3339Nano)+`"}`,
	))
	require.ErrorContains(t, comparator.CompareJSON(
		`{"scheduled_at":"$NOW+1h_WITHIN(10s)$"}`,
		`{"scheduled_at":"`+time.Now().Add(time.Hour+11*time.Second).Format(time.RFC3339Nano)+`"}`,
	), "time mismatch")
}
