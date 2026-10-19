package bdd

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"regexp"
	"testing"
	"time"
)

// Test WildcardProcessor core functionality
func TestWildcardProcessor_ProcessWildcards(t *testing.T) {
	processor := NewWildcardProcessor()

	tests := []struct {
		name     string
		input    interface{}
		context  Context
		validate func(t *testing.T, result interface{}, err error)
	}{
		{
			name:    "simple string without wildcards",
			input:   "hello world",
			context: ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "hello world", result)
			},
		},
		{
			name:    "UUID wildcard in comparison context",
			input:   "$UUID$",
			context: ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "$UUID$", result) // Should remain as wildcard
			},
		},
		{
			name:    "UUID wildcard in generation context",
			input:   "$UUID$",
			context: ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				uuidStr, ok := result.(string)
				require.True(t, ok, "Expected string result")

				// Validate UUID format
				uuidRegex := `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
				matched, _ := regexp.MatchString(uuidRegex, uuidStr)
				assert.True(t, matched, "Should generate valid UUID")
			},
		},
		{
			name:    "NOW wildcard in comparison context",
			input:   "$NOW$",
			context: ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "$NOW$", result) // Should remain as wildcard
			},
		},
		{
			name:    "NOW wildcard in generation context",
			input:   "$NOW$",
			context: ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				timeStr, ok := result.(string)
				require.True(t, ok, "Expected string result")

				// Should be valid RFC3339 time
				_, parseErr := time.Parse(time.RFC3339, timeStr)
				assert.NoError(t, parseErr, "Should generate valid RFC3339 time")
			},
		},
		{
			name:    "NOW with duration wildcard",
			input:   "$NOW+1h$",
			context: ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				timeStr, ok := result.(string)
				require.True(t, ok, "Expected string result")

				parsedTime, parseErr := time.Parse(time.RFC3339, timeStr)
				assert.NoError(t, parseErr, "Should generate valid RFC3339 time")

				// Should be approximately 1 hour from now
				expectedTime := time.Now().Add(time.Hour)
				timeDiff := parsedTime.Sub(expectedTime)
				if timeDiff < 0 {
					timeDiff = -timeDiff
				}
				assert.True(t, timeDiff < time.Minute, "Should be within 1 minute of expected time")
			},
		},
		{
			name:    "Variable wildcard - undefined variable",
			input:   "$VAR(undefined_var)$",
			context: ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "variable 'undefined_var' not found")
			},
		},
		{
			name:    "NULL wildcard in comparison context",
			input:   "$NULL$",
			context: ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Nil(t, result) // Should return nil for NULL
			},
		},
		{
			name:    "NULL wildcard in generation context",
			input:   "$NULL$",
			context: ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Nil(t, result) // Should return nil for NULL
			},
		},
		{
			name: "Map with wildcards",
			input: map[string]interface{}{
				"id":        "$UUID$",
				"timestamp": "$NOW$",
				"name":      "test",
				"nullable":  "$NULL$",
			},
			context: ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				resultMap, ok := result.(map[string]interface{})
				require.True(t, ok, "Expected map result")

				// Validate UUID
				uuidStr, ok := resultMap["id"].(string)
				require.True(t, ok, "Expected string UUID")
				uuidRegex := `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
				matched, _ := regexp.MatchString(uuidRegex, uuidStr)
				assert.True(t, matched, "Should generate valid UUID")

				// Validate timestamp
				timeStr, ok := resultMap["timestamp"].(string)
				require.True(t, ok, "Expected string timestamp")
				_, parseErr := time.Parse(time.RFC3339, timeStr)
				assert.NoError(t, parseErr, "Should generate valid RFC3339 time")

				// Validate unchanged value
				assert.Equal(t, "test", resultMap["name"])

				// Validate NULL value
				assert.Nil(t, resultMap["nullable"])
			},
		},
		{
			name:    "Array with wildcards",
			input:   []interface{}{"$UUID$", "$NOW$", "static", "$NULL$"},
			context: ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				resultArray, ok := result.([]interface{})
				require.True(t, ok, "Expected array result")
				require.Len(t, resultArray, 4)

				// Validate UUID
				uuidStr, ok := resultArray[0].(string)
				require.True(t, ok, "Expected string UUID")
				uuidRegex := `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
				matched, _ := regexp.MatchString(uuidRegex, uuidStr)
				assert.True(t, matched, "Should generate valid UUID")

				// Validate timestamp
				timeStr, ok := resultArray[1].(string)
				require.True(t, ok, "Expected string timestamp")
				_, parseErr := time.Parse(time.RFC3339, timeStr)
				assert.NoError(t, parseErr, "Should generate valid RFC3339 time")

				// Validate static value
				assert.Equal(t, "static", resultArray[2])

				// Validate NULL value
				assert.Nil(t, resultArray[3])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}
			var err error

			if tt.context == ContextGeneration {
				result, err = processor.SubstituteWildcards(tt.input)
			} else {
				result, err = processor.ResolveWildcards(tt.input)
			}

			tt.validate(t, result, err)
		})
	}
}

func TestWildcardProcessor_NonStringMapKey(t *testing.T) {
	processor := NewWildcardProcessor()
	processor.SetVariable("key", 42)
	_, err := processor.SubstituteWildcards(map[string]interface{}{"$VAR(key)$": "value"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "map key must resolve to a string")
}

// Test VariableHandler functionality
func TestVariableHandler_SetAndGetVariable(t *testing.T) {
	handler := NewVariableHandler()

	tests := []struct {
		name        string
		setName     string
		setValue    interface{}
		getName     string
		expectFound bool
		expectValue interface{}
	}{
		{
			name:        "set and get string variable",
			setName:     "test_string",
			setValue:    "hello world",
			getName:     "test_string",
			expectFound: true,
			expectValue: "hello world",
		},
		{
			name:        "set and get integer variable",
			setName:     "test_int",
			setValue:    42,
			getName:     "test_int",
			expectFound: true,
			expectValue: 42,
		},
		{
			name:        "set and get float variable",
			setName:     "test_float",
			setValue:    3.14,
			getName:     "test_float",
			expectFound: true,
			expectValue: 3.14,
		},
		{
			name:        "set and get boolean variable",
			setName:     "test_bool",
			setValue:    true,
			getName:     "test_bool",
			expectFound: true,
			expectValue: true,
		},
		{
			name:        "get non-existent variable",
			setName:     "existing",
			setValue:    "value",
			getName:     "non_existent",
			expectFound: false,
			expectValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler.SetVariable(tt.setName, tt.setValue)
			value, found := handler.GetVariable(tt.getName)

			assert.Equal(t, tt.expectFound, found)
			if tt.expectFound {
				assert.Equal(t, tt.expectValue, value)
			}
		})
	}
}

// Test VariableHandler wildcard processing
func TestVariableHandler_Process(t *testing.T) {
	handler := NewVariableHandler()

	// Set up test variables
	handler.SetVariable("string_var", "test_value")
	handler.SetVariable("int_var", 123)
	handler.SetVariable("float_var", 45.67)
	handler.SetVariable("bool_var", true)

	tests := []struct {
		name     string
		wildcard string
		context  Context
		validate func(t *testing.T, result interface{}, err error)
	}{
		{
			name:     "valid string variable",
			wildcard: "$VAR(string_var)$",
			context:  ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "test_value", result)
			},
		},
		{
			name:     "valid integer variable",
			wildcard: "$VAR(int_var)$",
			context:  ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, 123, result)
			},
		},
		{
			name:     "valid float variable",
			wildcard: "$VAR(float_var)$",
			context:  ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, 45.67, result)
			},
		},
		{
			name:     "valid boolean variable",
			wildcard: "$VAR(bool_var)$",
			context:  ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, true, result)
			},
		},
		{
			name:     "undefined variable",
			wildcard: "$VAR(undefined)$",
			context:  ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "variable 'undefined' not found")
				assert.Contains(t, err.Error(), "Currently stored variables:")
			},
		},
		{
			name:     "invalid wildcard format - missing closing",
			wildcard: "$VAR(test",
			context:  ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid variable wildcard")
			},
		},
		{
			name:     "invalid wildcard format - empty name",
			wildcard: "$VAR()$",
			context:  ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid variable wildcard")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.Process(tt.wildcard, tt.context)
			tt.validate(t, result, err)
		})
	}
}

// Test GenericHandler functionality
func TestGenericHandler_Process(t *testing.T) {
	handler := NewGenericHandler()

	tests := []struct {
		name     string
		wildcard string
		context  Context
		validate func(t *testing.T, result interface{}, err error)
	}{
		{
			name:     "ANY wildcard in comparison context",
			wildcard: "$ANY$",
			context:  ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "$ANY$", result)
			},
		},
		{
			name:     "ANY_INT wildcard",
			wildcard: "$ANY_INT$",
			context:  ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "$ANY_INT$", result)
			},
		},
		{
			name:     "ANY_FLOAT wildcard",
			wildcard: "$ANY_FLOAT$",
			context:  ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "$ANY_FLOAT$", result)
			},
		},
		{
			name:     "ANY_BOOL wildcard",
			wildcard: "$ANY_BOOL$",
			context:  ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "$ANY_BOOL$", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.Process(tt.wildcard, tt.context)
			tt.validate(t, result, err)
		})
	}
}

// Test NullHandler functionality
func TestNullHandler_Process(t *testing.T) {
	handler := NewNullHandler()

	tests := []struct {
		name     string
		wildcard string
		context  Context
		validate func(t *testing.T, result interface{}, err error)
	}{
		{
			name:     "NULL wildcard in comparison context",
			wildcard: "$NULL$",
			context:  ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Nil(t, result) // Should return nil for NULL
			},
		},
		{
			name:     "NULL wildcard in generation context",
			wildcard: "$NULL$",
			context:  ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Nil(t, result) // Should return nil for NULL
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.Process(tt.wildcard, tt.context)
			tt.validate(t, result, err)
		})
	}
}

// Test NullHandler CanHandle functionality
func TestNullHandler_CanHandle(t *testing.T) {
	handler := NewNullHandler()

	tests := []struct {
		name     string
		wildcard string
		expected bool
	}{
		{
			name:     "NULL wildcard",
			wildcard: "$NULL$",
			expected: true,
		},
		{
			name:     "some wildcard",
			wildcard: "$SOME$",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.CanHandle(tt.wildcard)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test NullHandler ShouldBeQuotedInJSON functionality
func TestNullHandler_ShouldBeQuotedInJSON(t *testing.T) {
	handler := NewNullHandler()

	tests := []struct {
		name           string
		wildcard       string
		processedValue interface{}
		expected       bool
	}{
		{
			name:           "NULL wildcard with nil value",
			wildcard:       "$NULL$",
			processedValue: nil,
			expected:       false, // null values should not be quoted in JSON
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.ShouldBeQuotedInJSON(tt.wildcard, tt.processedValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}
