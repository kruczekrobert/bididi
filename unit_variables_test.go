package bdd

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"regexp"
	"strings"
	"testing"
)

func TestFeaturesContext_ConvertWithTypePrefix(t *testing.T) {
	f := &FeaturesContext{}

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "string without prefix",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "string with str prefix",
			input:    "str:123.45",
			expected: "123.45",
		},
		{
			name:     "string with int prefix - valid",
			input:    "int:42",
			expected: 42,
		},
		{
			name:     "string with int prefix - invalid",
			input:    "int:abc",
			expected: "abc",
		},
		{
			name:     "string with float prefix - valid",
			input:    "float:3.14",
			expected: 3.14,
		},
		{
			name:     "string with float prefix - invalid",
			input:    "float:xyz",
			expected: "xyz",
		},
		{
			name:     "string with bool prefix - true",
			input:    "bool:true",
			expected: true,
		},
		{
			name:     "string with bool prefix - false",
			input:    "bool:false",
			expected: false,
		},
		{
			name:     "string with bool prefix - invalid",
			input:    "bool:maybe",
			expected: "maybe",
		},
		{
			name:     "string with auto prefix",
			input:    "auto:123",
			expected: 123,
		},
		{
			name:     "auto-detection - integer",
			input:    "456",
			expected: 456,
		},
		{
			name:     "auto-detection - float",
			input:    "78.9",
			expected: 78.9,
		},
		{
			name:     "auto-detection - boolean true",
			input:    "true",
			expected: true,
		},
		{
			name:     "auto-detection - boolean false",
			input:    "false",
			expected: false,
		},
		{
			name:     "auto-detection - string",
			input:    "not_a_number",
			expected: "not_a_number",
		},
		{
			name:     "non-string input",
			input:    123,
			expected: 123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.convertWithTypePrefix(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFeaturesContext_ParseVariableNameWithPrefix(t *testing.T) {
	f := &FeaturesContext{}

	tests := []struct {
		name           string
		input          string
		expectedName   string
		expectedPrefix string
	}{
		{
			name:           "no prefix",
			input:          "simple_var",
			expectedName:   "simple_var",
			expectedPrefix: "",
		},
		{
			name:           "string prefix",
			input:          "str:my_var",
			expectedName:   "my_var",
			expectedPrefix: "str",
		},
		{
			name:           "int prefix",
			input:          "int:count",
			expectedName:   "count",
			expectedPrefix: "int",
		},
		{
			name:           "float prefix",
			input:          "float:price",
			expectedName:   "price",
			expectedPrefix: "float",
		},
		{
			name:           "bool prefix",
			input:          "bool:enabled",
			expectedName:   "enabled",
			expectedPrefix: "bool",
		},
		{
			name:           "auto prefix",
			input:          "auto:value",
			expectedName:   "value",
			expectedPrefix: "auto",
		},
		{
			name:           "variable name with colon",
			input:          "api:endpoint:url",
			expectedName:   "api:endpoint:url",
			expectedPrefix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, prefix := f.parseVariableNameWithPrefix(tt.input)
			assert.Equal(t, tt.expectedName, name)
			assert.Equal(t, tt.expectedPrefix, prefix)
		})
	}
}

func TestFeaturesContext_ConvertValueWithTypePrefix(t *testing.T) {
	f := &FeaturesContext{}

	tests := []struct {
		name       string
		value      interface{}
		typePrefix string
		expected   interface{}
	}{
		{
			name:       "force string conversion",
			value:      123,
			typePrefix: "str",
			expected:   "123",
		},
		{
			name:       "force int conversion from string",
			value:      "456",
			typePrefix: "int",
			expected:   456,
		},
		{
			name:       "force int conversion from float",
			value:      78.9,
			typePrefix: "int",
			expected:   78,
		},
		{
			name:       "force float conversion from string",
			value:      "3.14",
			typePrefix: "float",
			expected:   3.14,
		},
		{
			name:       "force float conversion from int",
			value:      42,
			typePrefix: "float",
			expected:   42.0,
		},
		{
			name:       "force bool conversion from string true",
			value:      "true",
			typePrefix: "bool",
			expected:   true,
		},
		{
			name:       "force bool conversion from string false",
			value:      "false",
			typePrefix: "bool",
			expected:   false,
		},
		{
			name:       "force bool conversion from int non-zero",
			value:      1,
			typePrefix: "bool",
			expected:   true,
		},
		{
			name:       "force bool conversion from int zero",
			value:      0,
			typePrefix: "bool",
			expected:   false,
		},
		{
			name:       "auto conversion",
			value:      "123",
			typePrefix: "auto",
			expected:   123,
		},
		{
			name:       "no prefix - preserve original",
			value:      "test",
			typePrefix: "",
			expected:   "test",
		},
		{
			name:       "unknown prefix - preserve original",
			value:      "test",
			typePrefix: "unknown",
			expected:   "test",
		},
		{
			name:       "invalid int conversion",
			value:      "not_a_number",
			typePrefix: "int",
			expected:   "not_a_number",
		},
		{
			name:       "invalid float conversion",
			value:      "not_a_float",
			typePrefix: "float",
			expected:   "not_a_float",
		},
		{
			name:       "invalid bool conversion",
			value:      "maybe",
			typePrefix: "bool",
			expected:   "maybe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.convertValueWithTypePrefix(tt.value, tt.typePrefix)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertToInt(t *testing.T) {
	f := &FeaturesContext{}

	tests := []struct {
		name        string
		input       interface{}
		expected    int
		shouldError bool
	}{
		{
			name:        "int input",
			input:       42,
			expected:    42,
			shouldError: false,
		},
		{
			name:        "int64 input",
			input:       int64(123),
			expected:    123,
			shouldError: false,
		},
		{
			name:        "float64 input",
			input:       78.9,
			expected:    78,
			shouldError: false,
		},
		{
			name:        "string numeric input",
			input:       "456",
			expected:    456,
			shouldError: false,
		},
		{
			name:        "string non-numeric input",
			input:       "abc",
			expected:    0,
			shouldError: true,
		},
		{
			name:        "unsupported type",
			input:       []int{1, 2, 3},
			expected:    0,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.convertToInt(tt.input)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConvertToFloat(t *testing.T) {
	f := &FeaturesContext{}

	tests := []struct {
		name        string
		input       interface{}
		expected    float64
		shouldError bool
	}{
		{
			name:        "float64 input",
			input:       3.14,
			expected:    3.14,
			shouldError: false,
		},
		{
			name:        "float32 input",
			input:       float32(2.5),
			expected:    2.5,
			shouldError: false,
		},
		{
			name:        "int input",
			input:       42,
			expected:    42.0,
			shouldError: false,
		},
		{
			name:        "int64 input",
			input:       int64(123),
			expected:    123.0,
			shouldError: false,
		},
		{
			name:        "string numeric input",
			input:       "45.67",
			expected:    45.67,
			shouldError: false,
		},
		{
			name:        "string non-numeric input",
			input:       "not_a_number",
			expected:    0,
			shouldError: true,
		},
		{
			name:        "unsupported type",
			input:       []float64{1.1, 2.2},
			expected:    0,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.convertToFloat(tt.input)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConvertToBool(t *testing.T) {
	f := &FeaturesContext{}

	tests := []struct {
		name        string
		input       interface{}
		expected    bool
		shouldError bool
	}{
		{
			name:        "bool true input",
			input:       true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "bool false input",
			input:       false,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "string true input",
			input:       "true",
			expected:    true,
			shouldError: false,
		},
		{
			name:        "string false input",
			input:       "false",
			expected:    false,
			shouldError: false,
		},
		{
			name:        "string True input",
			input:       "True",
			expected:    true,
			shouldError: false,
		},
		{
			name:        "string FALSE input",
			input:       "FALSE",
			expected:    false,
			shouldError: false,
		},
		{
			name:        "int non-zero input",
			input:       1,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "int zero input",
			input:       0,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "float non-zero input",
			input:       3.14,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "float zero input",
			input:       0.0,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "string invalid input",
			input:       "maybe",
			expected:    false,
			shouldError: true,
		},
		{
			name:        "unsupported type",
			input:       []bool{true, false},
			expected:    false,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.convertToBool(tt.input)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFeaturesContext_SetVariable(t *testing.T) {
	f := newFeaturesContext()

	tests := []struct {
		name          string
		varName       string
		value         string
		expectedValue interface{}
		shouldError   bool
	}{
		{
			name:          "set string variable",
			varName:       "test_string",
			value:         "hello world",
			expectedValue: "hello world",
			shouldError:   false,
		},
		{
			name:          "set integer variable with auto-detection",
			varName:       "test_int",
			value:         "123",
			expectedValue: 123,
			shouldError:   false,
		},
		{
			name:          "set float variable with auto-detection",
			varName:       "test_float",
			value:         "45.67",
			expectedValue: 45.67,
			shouldError:   false,
		},
		{
			name:          "set boolean variable with auto-detection",
			varName:       "test_bool",
			value:         "true",
			expectedValue: true,
			shouldError:   false,
		},
		{
			name:          "set string variable with str prefix",
			varName:       "test_str_prefix",
			value:         "str:123",
			expectedValue: "123",
			shouldError:   false,
		},
		{
			name:          "set integer variable with int prefix",
			varName:       "test_int_prefix",
			value:         "int:456",
			expectedValue: 456,
			shouldError:   false,
		},
		{
			name:          "set float variable with float prefix",
			varName:       "test_float_prefix",
			value:         "float:78.9",
			expectedValue: 78.9,
			shouldError:   false,
		},
		{
			name:          "set boolean variable with bool prefix",
			varName:       "test_bool_prefix",
			value:         "bool:false",
			expectedValue: false,
			shouldError:   false,
		},
		{
			name:          "set variable with wildcard in value",
			varName:       "test_with_uuid",
			value:         "$UUID$",
			expectedValue: nil,
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := f.setVariable(tt.varName, tt.value)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.name != "set variable with wildcard in value" {
					value, exists := f.wildcardProcessor.GetVariable(tt.varName)
					assert.True(t, exists, "Variable should exist")
					assert.Equal(t, tt.expectedValue, value)
				}
			}
		})
	}
}

func TestWildcardProcessor_ProcessMultipleWildcards(t *testing.T) {
	processor := NewWildcardProcessor()

	processor.SetVariable("company", "acme")
	processor.SetVariable("dept", "engineering")
	processor.SetVariable("id", "123")

	tests := []struct {
		name     string
		input    string
		context  Context
		validate func(t *testing.T, result interface{}, err error)
	}{
		{
			name:    "multiple variables in string",
			input:   "$VAR(company)$-$VAR(dept)$-$VAR(id)$",
			context: ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "acme-engineering-123", result)
			},
		},
		{
			name:    "mixed wildcards and static text",
			input:   "user-$VAR(company)$-dep-$VAR(dept)$-num-$VAR(id)$",
			context: ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "user-acme-dep-engineering-num-123", result)
			},
		},
		{
			name:    "UUID and NOW in same string",
			input:   "log.$UUID$.$NOW$",
			context: ContextGeneration,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				str, ok := result.(string)
				require.True(t, ok, "Expected string result")

				assert.True(t, strings.HasPrefix(str, "log."))

				parts := strings.Split(str, ".")
				require.GreaterOrEqual(t, len(parts), 2, "Should have at least 2 parts")

				uuidPart := parts[1]
				uuidRegex := `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
				matched, _ := regexp.MatchString(uuidRegex, uuidPart)
				assert.True(t, matched, "Should contain valid UUID")
			},
		},
		{
			name:    "mixed wildcards in comparison context",
			input:   "$VAR(company)$-$UUID$-$NOW$",
			context: ContextComparison,
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Equal(t, "acme-$UUID$-$NOW$", result)
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
