package bdd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

// Supports type prefixes: str:, int:, float:, bool:, auto: (default)
// Examples:
//
//	set variable "price" to "100.50"        -> auto-converts to float64(100.5)
//	set variable "price" to "str:100.50"    -> keeps as string "100.50"
//	set variable "count" to "int:42"        -> converts to int(42)
//	set variable "rate" to "float:3.14"     -> converts to float64(3.14)
func (f *FeaturesContext) setVariable(varName, value string) error {
	processedName, err := f.wildcardProcessor.ResolveWildcards(varName)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in variable name: %w", err)
	}
	name, ok := processedName.(string)
	if !ok {
		return fmt.Errorf("variable name is not a string: %T", processedName)
	}
	processedValue, err := f.wildcardProcessor.SubstituteWildcards(value)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in variable value: %v%s", err, f.wildcardProcessor.GetVariablesDebugInfo())
	}

	finalValue := f.convertWithTypePrefix(processedValue)

	f.wildcardProcessor.SetVariable(name, finalValue)

	return nil
}

func (f *FeaturesContext) checkVariable(varName, op, expected string) error {
	expectedResolved, err := f.wildcardProcessor.ResolveWildcards(expected)
	if err != nil {
		return fmt.Errorf("cannot resolve wildcards in variable value: %v%s", err, f.wildcardProcessor.GetVariablesDebugInfo())
	}

	varNameResolved, err := f.wildcardProcessor.ResolveWildcards(varName)
	if err != nil {
		return fmt.Errorf("cannot resolve var name in variable value: %v%s", err, f.wildcardProcessor.GetVariablesDebugInfo())
	}

	resolvedName, ok := varNameResolved.(string)
	if !ok {
		return fmt.Errorf("variable name must resolve to a string, got %T", varNameResolved)
	}
	actualVariable, ok := f.wildcardProcessor.GetVariable(resolvedName)
	if !ok {
		return fmt.Errorf("cannot get actual variable from memory: %s", f.wildcardProcessor.GetVariablesDebugInfo())
	}

	if actualVariable == nil || expectedResolved == nil {
		switch op {
		case "equals":
			if actualVariable == nil && expectedResolved == nil {
				return nil
			}
		case "not equals":
			if (actualVariable == nil) != (expectedResolved == nil) {
				return nil
			}
		case "contains", "not contains":
			return fmt.Errorf("cannot use %q with NULL value", op)
		default:
			return fmt.Errorf("unknown op %q", op)
		}
		return fmt.Errorf("variable value %v does not match expected value %v", actualVariable, expectedResolved)
	}

	actualVariableStr := fmt.Sprintf("%v", actualVariable)
	expectedResolvedStr := fmt.Sprintf("%v", expectedResolved)

	switch op {
	case "equals":
		if expectedResolvedStr != actualVariableStr {
			return fmt.Errorf("variable value %s does not match expected value %s", actualVariable, expectedResolved)
		}
		return nil
	case "not equals":
		if expectedResolvedStr == actualVariableStr {
			return fmt.Errorf("variable value %s does match expected value %s which is wrong", actualVariable, expectedResolved)
		}
		return nil
	case "contains":
		if !strings.Contains(actualVariableStr, expectedResolvedStr) {
			return fmt.Errorf("variable value %s does not contain expected value %s", actualVariable, expected)
		}
		return nil
	case "not contains":
		if strings.Contains(actualVariableStr, expectedResolvedStr) {
			return fmt.Errorf("variable value %s should not contain expected value %s", actualVariable, expected)
		}
		return nil
	default:
		return fmt.Errorf("unknown op %q", op)
	}
}

func (f *FeaturesContext) checkVariableJSON(varName string, expected *godog.DocString) error {
	resolved, err := f.wildcardProcessor.ResolveWildcards(varName)
	if err != nil {
		return fmt.Errorf("cannot resolve variable name: %w", err)
	}
	name, ok := resolved.(string)
	if !ok {
		return fmt.Errorf("variable name must resolve to a string, got %T", resolved)
	}
	actual, ok := f.wildcardProcessor.GetVariable(name)
	if !ok {
		return fmt.Errorf("cannot get actual variable from memory: %s", f.wildcardProcessor.GetVariablesDebugInfo())
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return fmt.Errorf("cannot encode variable %q as JSON: %w", name, err)
	}
	if err := f.jsonComparator.CompareJSON(expected.Content, string(actualJSON)); err != nil {
		return fmt.Errorf("variable %q: %w", name, err)
	}
	return nil
}

func (f *FeaturesContext) convertWithTypePrefix(value interface{}) interface{} {
	str, ok := value.(string)
	if !ok {
		return value
	}

	if strings.HasPrefix(str, "str:") {
		return strings.TrimPrefix(str, "str:")
	}

	if strings.HasPrefix(str, "int:") {
		valueStr := strings.TrimPrefix(str, "int:")
		if intVal, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
			return int(intVal)
		}
		return valueStr
	}

	if strings.HasPrefix(str, "float:") {
		valueStr := strings.TrimPrefix(str, "float:")
		if floatVal, err := strconv.ParseFloat(valueStr, 64); err == nil {
			return floatVal
		}
		return valueStr
	}

	if strings.HasPrefix(str, "bool:") {
		valueStr := strings.TrimPrefix(str, "bool:")
		if boolVal, err := strconv.ParseBool(valueStr); err == nil {
			return boolVal
		}
		return valueStr
	}

	if strings.HasPrefix(str, "auto:") {
		return f.convertStringToAppropriateType(strings.TrimPrefix(str, "auto:"))
	}

	return f.convertStringToAppropriateType(str)
}

func (f *FeaturesContext) convertStringToAppropriateType(value interface{}) interface{} {
	str, ok := value.(string)
	if !ok {
		return value
	}

	if lowerStr := strings.ToLower(str); lowerStr == "true" || lowerStr == "false" {
		if boolVal, err := strconv.ParseBool(lowerStr); err == nil {
			return boolVal
		}
	}

	if !strings.Contains(str, ".") {
		if intVal, err := strconv.ParseInt(str, 10, 64); err == nil {
			return int(intVal)
		}
	}

	if floatVal, err := strconv.ParseFloat(str, 64); err == nil {
		return floatVal
	}

	return str
}

func (f *FeaturesContext) convertValueWithTypePrefix(value interface{}, typePrefix string) interface{} {
	switch typePrefix {
	case "str":
		return fmt.Sprintf("%v", value)

	case "int":
		if intVal, err := f.convertToInt(value); err == nil {
			return intVal
		}
		return value

	case "float":
		if floatVal, err := f.convertToFloat(value); err == nil {
			return floatVal
		}
		return value

	case "bool":

		if boolVal, err := f.convertToBool(value); err == nil {
			return boolVal
		}
		return value

	case "auto":
		return f.convertStringToAppropriateType(value)

	default:
		return value
	}
}

func (f *FeaturesContext) convertToInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		if intVal, err := strconv.ParseInt(v, 10, 64); err == nil {
			return int(intVal), nil
		}
		return 0, fmt.Errorf("cannot convert string %s to int", v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

func (f *FeaturesContext) convertToFloat(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
			return floatVal, nil
		}
		return 0, fmt.Errorf("cannot convert string %s to float", v)
	default:
		return 0, fmt.Errorf("cannot convert %T to float", value)
	}
}

func (f *FeaturesContext) convertToBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		if boolVal, err := strconv.ParseBool(v); err == nil {
			return boolVal, nil
		}
		return false, fmt.Errorf("cannot convert string %s to bool", v)
	case int:
		return v != 0, nil
	case float64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}

// Supports type prefixes in variable name: str:varname, int:varname, float:varname, bool:varname, auto:varname
// Examples:
//
//	extract "price" from response as "amount"         -> auto-converts extracted value
//	extract "price" from response as "str:amount"     -> forces string type
//	extract "count" from response as "int:items"      -> forces integer type
func (f *FeaturesContext) extractVariableFromResponse(path, varName string) error {
	if f.apiResponse == nil {
		return fmt.Errorf("no API response available%s", f.wildcardProcessor.GetVariablesDebugInfo())
	}

	actualVarName, typePrefix := f.parseVariableNameWithPrefix(varName)

	value, err := f.jsonComparator.ExtractValueFromJSON(f.apiResponse.Body.String(), path)
	if err != nil {
		return fmt.Errorf("failed to extract value from response at path %s: %v%s", path, err, f.wildcardProcessor.GetVariablesDebugInfo())
	}

	finalValue := f.convertValueWithTypePrefix(value, typePrefix)

	f.wildcardProcessor.SetVariable(actualVarName, finalValue)
	return nil
}

func (f *FeaturesContext) parseVariableNameWithPrefix(varName string) (string, string) {
	prefixes := []string{"str:", "int:", "float:", "bool:", "auto:"}

	for _, prefix := range prefixes {
		if strings.HasPrefix(varName, prefix) {
			actualName := strings.TrimPrefix(varName, prefix)
			typePrefix := strings.TrimSuffix(prefix, ":")
			return actualName, typePrefix
		}
	}

	return varName, ""
}
