package bdd

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type JSONComparator struct {
	processor *WildcardProcessor
}

func NewJSONComparator(processor *WildcardProcessor) *JSONComparator {
	return &JSONComparator{
		processor: processor,
	}
}

func (jc *JSONComparator) CompareJSON(expected, actual string) error {
	// First, process wildcards in the raw JSON string before parsing
	// This preserves original types for variables like $VAR(price)$
	processedExpectedJSON, err := jc.ProcessWildcardsInJSONString(expected)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in expected JSON string: %v", err)
	}

	var expectedData, actualData interface{}

	if err := json.Unmarshal([]byte(processedExpectedJSON), &expectedData); err != nil {
		return fmt.Errorf("failed to unmarshal expected JSON: %v", err)
	}

	if err := json.Unmarshal([]byte(actual), &actualData); err != nil {
		return fmt.Errorf("failed to unmarshal actual JSON: %v", err)
	}

	err = jc.compareValues(expectedData, actualData, "")
	if err != nil {
		return jc.createEnhancedDiff(expected, actual, err)
	}
	return nil
}

func (jc *JSONComparator) compareValues(expected, actual interface{}, path string) error {
	switch expectedVal := expected.(type) {
	case string:
		switch expectedVal {
		case "$NOT_NULL$":
			if actual == nil {
				return fmt.Errorf("expected non-null value at %s, got null", path)
			}
			return nil
		case "$ANY_ARRAY$":
			if _, ok := actual.([]interface{}); !ok {
				return fmt.Errorf("expected array at %s, got %T", path, actual)
			}
			return nil
		}
		if matcher, ok := jc.processor.FindHandlerForWildcard(expectedVal).(WildcardMatcher); ok {
			return matcher.Match(expectedVal, actual)
		}
		if jc.isNumericWildcard(expectedVal) {
			return jc.compareNumericWildcard(expectedVal, actual, path)
		}
		return jc.compareString(expectedVal, actual, path)
	case map[string]interface{}:
		return jc.compareMaps(expectedVal, actual, path)
	case []interface{}:
		return jc.compareArraysUnordered(expectedVal, actual, path)
	case float64:
		// Handle float values from variables
		return jc.compareFloatValue(expectedVal, actual, path)
	case float32:
		// Handle float32 values from variables
		return jc.compareFloatValue(float64(expectedVal), actual, path)
	case int, int8, int16, int32, int64:
		// Handle integer values from variables
		return jc.compareIntegerValue(expectedVal, actual, path)
	case uint, uint8, uint16, uint32, uint64:
		// Handle unsigned integer values from variables
		return jc.compareIntegerValue(expectedVal, actual, path)
	case bool:
		// Handle boolean values from variables
		return jc.compareBooleanValue(expectedVal, actual, path)
	default:
		if expected != actual {
			return fmt.Errorf("value mismatch at '%s': expected %v (%T) but got %v (%T)", path, expected, expected, actual, actual)
		}
		return nil
	}
}

func (jc *JSONComparator) compareString(expected, actual interface{}, path string) error {
	expectedStr, ok := expected.(string)
	if !ok {
		return fmt.Errorf("expected string at %s, got %T", path, expected)
	}
	if expectedStr == "$ANY$" {
		return nil
	}

	actualStr, ok := actual.(string)
	if !ok {
		return fmt.Errorf("expected string at %s, got %T", path, actual)
	}

	// Handle wildcards
	switch expectedStr {
	case "$ANY_STR$":
		// Accept any string value (including empty)
		return nil
	case "$ANY_STR_FULL$":
		// Accept any non-empty string value
		if actualStr == "" {
			return fmt.Errorf("expected non-empty string at %s, got empty", path)
		}
		return nil
	case "$UUID$":
		return jc.compareUUIDWildcard(expectedStr, actualStr, path)
	}

	// Handle $NOW$ comparison with time tolerance
	if expectedStr == "$NOW$" || strings.HasPrefix(expectedStr, "$NOW") {
		return jc.compareTimeWildcard(expectedStr, actualStr, path)
	}

	// Handle $VAR(variable_name) - should have been processed already
	if strings.HasPrefix(expectedStr, "$VAR(") || strings.HasPrefix(expectedStr, "$NOW") {
		return fmt.Errorf("unprocessed variable/ now wildcard at %s: %s", path, expectedStr)
	}

	// Exact match
	if expectedStr != actualStr {
		return fmt.Errorf("string mismatch at '%s': expected '%s' but got '%s'", path, expectedStr, actualStr)
	}

	return nil
}

func (jc *JSONComparator) isNumericWildcard(s string) bool {
	return s == "$ANY_INT$" || s == "$ANY_FLOAT$" || s == "$ANY_BOOL$"
}

func (jc *JSONComparator) compareNumericWildcard(expected, actual interface{}, path string) error {
	expectedStr := expected.(string)

	switch expectedStr {
	case "$ANY_INT$":
		// Accept any integer value
		switch actual.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return nil
		case float64:
			// Check if it's a whole number
			if actual.(float64) == float64(int64(actual.(float64))) {
				return nil
			}
			return fmt.Errorf("expected integer at %s, got float %v", path, actual)
		default:
			return fmt.Errorf("expected integer at %s, got %T", path, actual)
		}

	case "$ANY_FLOAT$":
		// Accept any numeric value (int or float)
		switch actual.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return nil
		default:
			return fmt.Errorf("expected numeric value at %s, got %T", path, actual)
		}

	case "$ANY_BOOL$":
		// Accept any boolean value
		switch actual.(type) {
		case bool:
			return nil
		default:
			return fmt.Errorf("expected boolean at %s, got %T", path, actual)
		}
	}

	return fmt.Errorf("unknown numeric wildcard: %s", expectedStr)
}

func (jc *JSONComparator) compareFloatValue(expected float64, actual interface{}, path string) error {
	switch actualVal := actual.(type) {
	case float64:
		if expected != actualVal {
			return fmt.Errorf("float mismatch at '%s': expected %g but got %g", path, expected, actualVal)
		}
		return nil
	case float32:
		// Convert float32 to float64 for comparison
		if expected != float64(actualVal) {
			return fmt.Errorf("float mismatch at '%s': expected %g but got %g", path, expected, actualVal)
		}
		return nil
	default:
		return fmt.Errorf("type mismatch at '%s': expected float %g but got %T %v", path, expected, actual, actual)
	}
}

func (jc *JSONComparator) compareIntegerValue(expected interface{}, actual interface{}, path string) error {
	// Convert expected to int64 for unified comparison
	expectedInt64, err := toInt64(expected)
	if err != nil {
		return fmt.Errorf("invalid expected integer at '%s': %v", path, err)
	}

	// Check if actual is an integer type and convert to int64
	actualInt64, err := toInt64(actual)
	if err != nil {
		return fmt.Errorf("type mismatch at '%s': expected integer %d but got %T %v", path, expectedInt64, actual, actual)
	}

	if expectedInt64 != actualInt64 {
		return fmt.Errorf("integer mismatch at '%s': expected %d but got %d", path, expectedInt64, actualInt64)
	}
	return nil
}

func toInt64(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int8:
		return int64(val), nil
	case int16:
		return int64(val), nil
	case int32:
		return int64(val), nil
	case int64:
		return val, nil
	case uint:
		if val > uint(^uint64(0)>>1) {
			return 0, fmt.Errorf("uint value %d too large for int64", val)
		}
		return int64(val), nil
	case uint8:
		return int64(val), nil
	case uint16:
		return int64(val), nil
	case uint32:
		return int64(val), nil
	case uint64:
		if val > ^uint64(0)>>1 {
			return 0, fmt.Errorf("uint64 value %d too large for int64", val)
		}
		return int64(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

func (jc *JSONComparator) compareBooleanValue(expected bool, actual interface{}, path string) error {
	switch actualVal := actual.(type) {
	case bool:
		if expected != actualVal {
			return fmt.Errorf("boolean mismatch at '%s': expected %t but got %t", path, expected, actualVal)
		}
		return nil
	default:
		return fmt.Errorf("type mismatch at '%s': expected boolean %t but got %T %v", path, expected, actual, actual)
	}
}

func (jc *JSONComparator) compareTimeWildcard(expected, actual string, path string) error {
	now := time.Now()
	expectedTime, tolerance, err := expectedTimeAndTolerance(expected, now)
	if err != nil {
		return err
	}
	actualTime, err := parseWildcardTime(actual)
	if err != nil {
		return fmt.Errorf("invalid time format at %s: %v", path, err)
	}
	diff := expectedTime.Sub(actualTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > tolerance {
		return fmt.Errorf("time mismatch at %s: expected time within %s of %s, got %s (diff: %v)", path, tolerance, expectedTime.Format(time.RFC3339Nano), actual, diff)
	}
	return nil
}

func parseWildcardTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return parsed, nil
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999 -0700 MST",
		"2006-01-02 15:04:05.999999 +0000 UTC",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", value)
}

func (jc *JSONComparator) compareUUIDWildcard(expected, actual string, path string) error {
	if !isValidUUID(actual) {
		return fmt.Errorf("invalid UUID format at %s: got %s", path, actual)
	}
	return nil
}

func (jc *JSONComparator) compareMaps(expected, actual interface{}, path string) error {
	expectedMap, ok := expected.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map at %s, got %T", path, expected)
	}

	actualMap, ok := actual.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map at %s, got %T", path, actual)
	}

	// EXACT mode: Check all expected keys exist in actual
	for key, expectedValue := range expectedMap {
		actualValue, exists := actualMap[key]
		if !exists {
			return fmt.Errorf("missing required field '%s' at path '%s' - this field exists in expected JSON but is missing from actual response", key, path)
		}

		if err := jc.compareValues(expectedValue, actualValue, fmt.Sprintf("%s.%s", path, key)); err != nil {
			return err
		}
	}

	// EXACT mode: check that actual doesn't have extra keys
	for key := range actualMap {
		if _, exists := expectedMap[key]; !exists {
			keyPath := key
			if path != "" {
				keyPath = fmt.Sprintf("%s.%s", path, key)
			}
			return fmt.Errorf("unexpected extra field '%s' found in response - this field exists in actual JSON but was not expected", keyPath)
		}
	}

	return nil
}

func (jc *JSONComparator) compareArraysUnordered(expected, actual interface{}, path string) error {
	expectedArray, ok := expected.([]interface{})
	if !ok {
		return fmt.Errorf("expected array at %s, got %T", path, expected)
	}

	actualArray, ok := actual.([]interface{})
	if !ok {
		return fmt.Errorf("expected array at %s, got %T", path, actual)
	}

	if len(expectedArray) != len(actualArray) {
		return fmt.Errorf(
			"array size mismatch at '%s': expected %d elements but got %d elements",
			path, len(expectedArray), len(actualArray),
		)
	}

	used := make([]bool, len(actualArray))
	var match func(int) bool
	match = func(i int) bool {
		if i == len(expectedArray) {
			return true
		}
		for j, actualValue := range actualArray {
			if used[j] {
				continue
			}
			if err := jc.compareValues(expectedArray[i], actualValue, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				continue
			}
			used[j] = true
			if match(i + 1) {
				return true
			}
			used[j] = false
		}
		return false
	}
	if match(0) {
		return nil
	}
	return fmt.Errorf("no matching element assignment for array at '%s'", path)
}

func (jc *JSONComparator) ExtractValueFromJSON(jsonStr, path string) (interface{}, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	return jc.extractValueByPath(data, path)
}

func (jc *JSONComparator) extractValueByPath(data interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			value, exists := v[part]
			if !exists {
				return nil, fmt.Errorf("key %s not found at path %s", part, strings.Join(parts[:i+1], "."))
			}
			current = value
		case []interface{}:
			index, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid array index %s at path %s", part, strings.Join(parts[:i+1], "."))
			}
			if index < 0 || index >= len(v) {
				return nil, fmt.Errorf("array index %d out of bounds at path %s", index, strings.Join(parts[:i+1], "."))
			}
			current = v[index]
		default:
			return nil, fmt.Errorf("cannot access %s on %T at path %s", part, current, strings.Join(parts[:i+1], "."))
		}
	}

	return current, nil
}

func (jc *JSONComparator) createEnhancedDiff(expected, actual string, originalError error) error {
	var result strings.Builder

	result.WriteString("❌ JSON comparison failed\n\n")

	// Show the original error with better formatting
	errorMsg := originalError.Error()
	result.WriteString("🔍 Issue detected:\n")
	result.WriteString(fmt.Sprintf("   %s\n\n", jc.formatError(errorMsg)))

	// Show expected and actual JSON in a beautiful format
	result.WriteString("📄 Expected JSON:\n")
	result.WriteString(jc.formatJSON(expected, "🎯"))
	result.WriteString("\n")

	result.WriteString("📄 Actual JSON:\n")
	result.WriteString(jc.formatJSON(actual, "📊"))
	result.WriteString("\n")

	return fmt.Errorf("%s", result.String())
}

func (jc *JSONComparator) formatError(errorMsg string) string {
	return "🐛 " + errorMsg
}

func (jc *JSONComparator) formatJSON(jsonStr, emoji string) string {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return fmt.Sprintf("   %s Invalid JSON: %s\n", emoji, jsonStr)
	}

	formatted, err := json.MarshalIndent(data, "   ", "  ")
	if err != nil {
		return fmt.Sprintf("   %s Error formatting JSON: %s\n", emoji, jsonStr)
	}

	// Add emoji to each line
	lines := strings.Split(string(formatted), "\n")
	var result strings.Builder
	for i, line := range lines {
		if i == 0 {
			result.WriteString(fmt.Sprintf("   %s %s\n", emoji, line))
		} else {
			result.WriteString(fmt.Sprintf("     %s\n", line))
		}
	}
	return result.String()
}

// ProcessWildcardsInJSONString processes wildcards in raw JSON string before parsing
// This preserves original data types using the extensible wildcard handler architecture
func (jc *JSONComparator) ProcessWildcardsInJSONString(jsonStr string) (string, error) {
	result := jsonStr
	// Find all wildcard patterns in the JSON string (quoted wildcards)
	// Use a general pattern that captures any $...$ pattern within quotes
	wildcardPattern := regexp.MustCompile(`"(\$[^"$]*\$)"`)
	matches := wildcardPattern.FindAllStringSubmatch(result, -1)

	for _, match := range matches {
		quotedWildcard := match[0] // e.g., "$VAR(price)$"
		wildcard := match[1]       // e.g., $VAR(price)$

		// Handle variable wildcards directly for better reliability, a solution for type problems
		// expected from test: `{"user_id": "$VAR(user_id)$", "price": "$VAR(price)$"}`,
		// actual value: `{"user_id": "123", "price": 99.99}`,
		// without it actual value will be: `{"user_id": "123", "price": "99.99"}` -> which is not true/valid
		if strings.HasPrefix(wildcard, "$VAR(") && strings.HasSuffix(wildcard, ")$") {
			// Extract variable name using regex
			re := regexp.MustCompile(`\$VAR\(([^)]+)\)\$`)
			varMatches := re.FindStringSubmatch(wildcard)
			if len(varMatches) >= 2 {
				varName := varMatches[1]
				if val, ok := jc.processor.GetVariable(varName); ok {
					encoded, err := json.Marshal(val)
					if err != nil {
						return "", fmt.Errorf("failed to encode variable %s: %w", varName, err)
					}
					result = strings.ReplaceAll(result, quotedWildcard, string(encoded))
					continue
				}
			}
		}

		// Use the wildcard processor to process this wildcard
		// We need to use a wrapper since ResolveWildcards expects interface{} data
		tempData := map[string]interface{}{"temp": wildcard}
		processedData, err := jc.processor.ResolveWildcards(tempData)
		if err != nil {
			return "", fmt.Errorf("failed to process wildcard %s: %v", wildcard, err)
		}

		processedMap := processedData.(map[string]interface{})
		processed := processedMap["temp"]

		// Handle nil values (like $NULL$) specially
		if processed == nil {
			// For nil values, replace with "null" (JSON representation of nil)
			result = strings.ReplaceAll(result, quotedWildcard, "null")
			continue
		}

		// Check if wildcard was actually processed (not just returned as-is)
		if processed == wildcard {
			continue
			//TODO: check
			//panic(fmt.Sprintf(
			//	"ResolveWildcards returned the same value as input for wildcard '%s'. "+
			//		"Expected it to be processed into a concrete value.",
			//	wildcard,
			//))
		}

		// Find the appropriate handler to determine JSON formatting (same as handle variable wildcards directly for better reliability, a solution for type problems but for status wildcards)
		handler := jc.processor.FindHandlerForWildcard(wildcard)
		shouldQuote := true // Safe default
		if handler != nil {
			shouldQuote = handler.ShouldBeQuotedInJSON(wildcard, processed)
		}

		var encoded []byte
		if shouldQuote {
			encoded, err = json.Marshal(fmt.Sprint(processed))
		} else {
			encoded, err = json.Marshal(processed)
		}
		if err != nil {
			return "", fmt.Errorf("failed to encode wildcard %s: %w", wildcard, err)
		}
		result = strings.ReplaceAll(result, quotedWildcard, string(encoded))
	}

	return result, nil
}
