package bdd

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestFeaturesContext_MatchesUUIDWildcard(t *testing.T) {
	f := &FeaturesContext{}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid UUID lowercase",
			input:    "550e8400-e29b-41d4-a716-446655440000",
			expected: true,
		},
		{
			name:     "valid UUID uppercase",
			input:    "550E8400-E29B-41D4-A716-446655440000",
			expected: true,
		},
		{
			name:     "valid UUID mixed case",
			input:    "550e8400-E29b-41D4-a716-446655440000",
			expected: true,
		},
		{
			name:     "invalid UUID - too short",
			input:    "550e8400-e29b-41d4-a716-44665544000",
			expected: false,
		},
		{
			name:     "invalid UUID - too long",
			input:    "550e8400-e29b-41d4-a716-4466554400000",
			expected: false,
		},
		{
			name:     "invalid UUID - wrong format",
			input:    "550e8400_e29b_41d4_a716_446655440000",
			expected: false,
		},
		{
			name:     "invalid UUID - contains invalid characters",
			input:    "550e8400-e29b-41d4-a716-44665544000g",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "not a UUID",
			input:    "just-a-string",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.matchesUUIDWildcard(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFeaturesContext_MatchesTimeWildcard(t *testing.T) {
	f := &FeaturesContext{}
	now := time.Now()

	tests := []struct {
		name     string
		expected string
		actual   string
		result   bool
	}{
		{
			name:     "NOW with current time RFC3339",
			expected: "$NOW$",
			actual:   now.Format(time.RFC3339),
			result:   true,
		},
		{
			name:     "NOW with current time RFC3339Nano",
			expected: "$NOW$",
			actual:   now.Format(time.RFC3339Nano),
			result:   true,
		},
		{
			name:     "NOW with database timestamp format",
			expected: "$NOW$",
			actual:   now.Format("2006-01-02 15:04:05"),
			result:   true,
		},
		{
			name:     "NOW with ISO format",
			expected: "$NOW$",
			actual:   now.UTC().Format("2006-01-02T15:04:05Z"),
			result:   true,
		},
		{
			name:     "NOW with Go UTC format",
			expected: "$NOW$",
			actual:   now.UTC().Format("2006-01-02 15:04:05.999999 +0000 UTC"),
			result:   true,
		},
		{
			name:     "NOW with time 1 minute ago",
			expected: "$NOW$",
			actual:   now.Add(-59 * time.Second).Format(time.RFC3339),
			result:   true,
		},
		{
			name:     "NOW with time 2 minutes ago",
			expected: "$NOW$",
			actual:   now.Add(61 * time.Second).Format(time.RFC3339),
			result:   false,
		},
		{
			name:     "NOW+1h with correct time",
			expected: "$NOW+1h$",
			actual:   now.Add(time.Hour).Format(time.RFC3339),
			result:   true,
		},
		{
			name:     "NOW-30m with correct time",
			expected: "$NOW-30m$",
			actual:   now.Add(-30 * time.Minute).Format(time.RFC3339),
			result:   true,
		},
		{
			name:     "NOW+1h with wrong time",
			expected: "$NOW+1h$",
			actual:   now.Add(2 * time.Hour).Format(time.RFC3339),
			result:   false,
		},
		{
			name:     "NOW with invalid time format",
			expected: "$NOW$",
			actual:   "not-a-time",
			result:   false,
		},
		{
			name:     "invalid duration format",
			expected: "$NOW+invalid$",
			actual:   now.Format(time.RFC3339),
			result:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.matchesTimeWildcard(tt.expected, tt.actual)
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestFeaturesContext_MatchesTimeWildcardWithLocalOffset(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("UTC+2", 2*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	f := &FeaturesContext{}
	now := time.Now()
	const layout = "2006-01-02 15:04:05"
	assert.True(t, f.matchesTimeWildcard("$NOW+1h$", now.Add(time.Hour).Format(layout)))
	assert.True(t, f.matchesTimeWildcard("$NOW-30m$", now.Add(-30*time.Minute).Format(layout)))
}

func TestFeaturesContext_MatchesPartialRecordWithDetails(t *testing.T) {
	f := newFeaturesContext()

	tests := []struct {
		name          string
		expected      map[string]interface{}
		actual        map[string]interface{}
		expectMatch   bool
		expectedError string
	}{
		{
			name: "successful match",
			expected: map[string]interface{}{
				"name": "John",
				"age":  30,
			},
			actual: map[string]interface{}{
				"name": "John",
				"age":  30,
				"city": "New York",
			},
			expectMatch: true,
		},
		{
			name: "missing field error",
			expected: map[string]interface{}{
				"name": "John",
				"age":  30,
			},
			actual: map[string]interface{}{
				"name": "John",
			},
			expectMatch:   false,
			expectedError: "Field 'age' missing in database record",
		},
		{
			name: "string mismatch error",
			expected: map[string]interface{}{
				"name": "John",
			},
			actual: map[string]interface{}{
				"name": "Jane",
			},
			expectMatch:   false,
			expectedError: "Field 'name': expected 'John' but got 'Jane'",
		},
		{
			name: "UUID validation error",
			expected: map[string]interface{}{
				"id": "$UUID$",
			},
			actual: map[string]interface{}{
				"id": "invalid-uuid",
			},
			expectMatch:   false,
			expectedError: "Field 'id': expected UUID format but got 'invalid-uuid'",
		},
		{
			name: "time wildcard error",
			expected: map[string]interface{}{
				"timestamp": "$NOW$",
			},
			actual: map[string]interface{}{
				"timestamp": "2020-01-01T00:00:00Z",
			},
			expectMatch:   false,
			expectedError: "Field 'timestamp': time wildcard '$NOW$' doesn't match actual time '2020-01-01T00:00:00Z'",
		},
		{
			name: "type mismatch for ANY_STR",
			expected: map[string]interface{}{
				"text": "$ANY_STR$",
			},
			actual: map[string]interface{}{
				"text": 123,
			},
			expectMatch:   false,
			expectedError: "Field 'text': expected string type but got 123 (int)",
		},
		{
			name: "empty string for ANY_STR_FULL",
			expected: map[string]interface{}{
				"text": "$ANY_STR_FULL$",
			},
			actual: map[string]interface{}{
				"text": "",
			},
			expectMatch:   false,
			expectedError: "Field 'text': expected non-empty string but got  (string)",
		},
		{
			name: "type mismatch for ANY_INT",
			expected: map[string]interface{}{
				"count": "$ANY_INT$",
			},
			actual: map[string]interface{}{
				"count": "not a number",
			},
			expectMatch:   false,
			expectedError: "Field 'count': expected integer type but got not a number (string)",
		},
		{
			name: "type mismatch for ANY_FLOAT",
			expected: map[string]interface{}{
				"price": "$ANY_FLOAT$",
			},
			actual: map[string]interface{}{
				"price": "not a number",
			},
			expectMatch:   false,
			expectedError: "Field 'price': expected float type but got not a number (string)",
		},
		{
			name: "type mismatch for ANY_BOOL",
			expected: map[string]interface{}{
				"active": "$ANY_BOOL$",
			},
			actual: map[string]interface{}{
				"active": "not a bool",
			},
			expectMatch:   false,
			expectedError: "Field 'active': expected boolean type but got not a bool (string)",
		},
		{
			name: "numeric value mismatch",
			expected: map[string]interface{}{
				"value": "123.45",
			},
			actual: map[string]interface{}{
				"value": 123.46,
			},
			expectMatch:   false,
			expectedError: "Field 'value': expected numeric value 123.45 but got 123.46",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.matchesPartialRecordWithDetails(tt.expected, tt.actual)

			assert.Equal(t, tt.expectMatch, result.matches)

			if !tt.expectMatch && tt.expectedError != "" {
				assert.Contains(t, result.errorMessage, tt.expectedError)
			}
		})
	}
}

func TestFeaturesContext_MatchDistinctRecords(t *testing.T) {
	f := newFeaturesContext()
	actual := []map[string]interface{}{{"name": "Ada"}, {"name": "Bob"}}
	assert.False(t, f.matchDistinctRecords(
		[]map[string]interface{}{{"name": "Ada"}, {"name": "Ada"}},
		actual,
	))
	assert.True(t, f.matchDistinctRecords(
		[]map[string]interface{}{{"name": "$ANY_STR$"}, {"name": "Ada"}},
		actual,
	))
}

func TestFeaturesContext_FormatRecords(t *testing.T) {
	f := &FeaturesContext{}

	tests := []struct {
		name     string
		records  []map[string]interface{}
		expected string
	}{
		{
			name:     "empty records",
			records:  []map[string]interface{}{},
			expected: "  (no records)",
		},
		{
			name: "single record",
			records: []map[string]interface{}{
				{
					"id":   1,
					"name": "John",
					"age":  30,
				},
			},
			expected: "Record 1:",
		},
		{
			name: "multiple records",
			records: []map[string]interface{}{
				{
					"id":   1,
					"name": "John",
				},
				{
					"id":   2,
					"name": "Jane",
				},
			},
			expected: "Record 1:",
		},
		{
			name: "record with null value",
			records: []map[string]interface{}{
				{
					"id":    1,
					"name":  "John",
					"email": nil,
				},
			},
			expected: "email: <null>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.formatRecordsAsSimpleList(tt.records)
			if tt.name == "empty records" {
				assert.Equal(t, tt.expected, result)
			} else {
				assert.Contains(t, result, tt.expected)
			}
		})
	}
}

func TestFeaturesContext_FormatRecordAsYAML(t *testing.T) {
	f := &FeaturesContext{}

	tests := []struct {
		name     string
		record   map[string]interface{}
		title    string
		expected string
	}{
		{
			name:     "empty record",
			record:   map[string]interface{}{},
			title:    "Test Record",
			expected: "  Test Record: (empty)",
		},
		{
			name: "simple record",
			record: map[string]interface{}{
				"id":   1,
				"name": "John",
			},
			title:    "User Record",
			expected: "User Record:",
		},
		{
			name: "record with null",
			record: map[string]interface{}{
				"id":    1,
				"email": nil,
			},
			title:    "Record with Null",
			expected: "email: <null>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.formatRecordAsYAML(tt.record, tt.title)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestConvertToStringInterfaceMap(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    map[string]interface{}
		shouldError bool
	}{
		{
			name: "map[string]interface{} input",
			input: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			shouldError: false,
		},
		{
			name: "map[string]string input",
			input: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			shouldError: false,
		},
		{
			name:        "invalid input type",
			input:       "not a map",
			expected:    nil,
			shouldError: true,
		},
		{
			name:        "nil input",
			input:       nil,
			expected:    nil,
			shouldError: true,
		},
		{
			name:        "map with non-string keys",
			input:       map[int]string{1: "value"},
			expected:    nil,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToStringInterfaceMap(tt.input)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
