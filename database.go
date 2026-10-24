package bdd

import (
	"fmt"
	"github.com/cucumber/godog"
	"sort"
	"strconv"
	"strings"
	"time"
)

type MatchResult struct {
	matches      bool
	errorMessage string
}

type Database interface {
	Exec(query string, args ...interface{}) error
	Query(query string, args ...interface{}) ([]map[string]interface{}, error)
	Count(table string) (int64, error)
}

func (f *FeaturesContext) formatRecordsAsSimpleList(records []map[string]interface{}) string {
	if len(records) == 0 {
		return "  (no records)"
	}

	var result strings.Builder
	for i, record := range records {
		result.WriteString(fmt.Sprintf("  📋 Record %d:\n", i+1))

		keys := make([]string, 0, len(record))
		for key := range record {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			value := "<null>"
			if val, exists := record[key]; exists && val != nil {
				value = fmt.Sprintf("%v", val)
			}
			result.WriteString(fmt.Sprintf("     %s: %s\n", key, value))
		}
		result.WriteString("\n")
	}
	return result.String()
}

func (f *FeaturesContext) formatRecordAsYAML(record map[string]interface{}, title string) string {
	if len(record) == 0 {
		return fmt.Sprintf("  %s: (empty)\n", title)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("  %s:\n", title))

	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := "<null>"
		if val, exists := record[key]; exists && val != nil {
			value = fmt.Sprintf("%v", val)
		}
		result.WriteString(fmt.Sprintf("    %s: %s\n", key, value))
	}
	return result.String()
}

func (f *FeaturesContext) addRecords(tableName string, data *godog.Table) error {
	if f.db == nil {
		return fmt.Errorf("database not initialized")
	}

	records := f.tableToMaps(data)

	if len(records) == 0 {
		return nil
	}

	for _, record := range records {

		processedRecord, err := f.wildcardProcessor.SubstituteWildcards(record)
		if err != nil {
			return fmt.Errorf("failed to process wildcards in record: %v", err)
		}

		recordMap, err := convertToStringInterfaceMap(processedRecord)
		if err != nil {
			return fmt.Errorf("failed to convert processed record: %v", err)
		}

		columns := make([]string, 0, len(recordMap))
		placeholders := make([]string, 0, len(recordMap))
		values := make([]interface{}, 0, len(recordMap))

		for column, value := range recordMap {
			if value == "" {
				continue
			}
			if value == "$EMPTY$" {
				columns = append(columns, column)
				placeholders = append(placeholders, "?")
				values = append(values, "")
				continue
			}

			if value == nil {
				columns = append(columns, column)
				placeholders = append(placeholders, "NULL")
			} else {
				columns = append(columns, column)
				placeholders = append(placeholders, "?")
				values = append(values, value)
			}
		}

		if len(columns) == 0 {
			continue
		}

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			tableName,
			strings.Join(columns, ", "),
			strings.Join(placeholders, ", "))

		err = f.db.Exec(query, values...)
		if err != nil {
			return fmt.Errorf("failed to insert record into %s: %v", tableName, err)
		}
	}

	return nil
}

func (f *FeaturesContext) haveRecords(tableName string, data *godog.Table) error {
	if f.db == nil {
		return fmt.Errorf("database not initialized")
	}

	expectedRecords := f.tableToMaps(data)

	if len(expectedRecords) == 0 {
		return nil
	}

	allResults, err := f.db.Query(fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		return fmt.Errorf("failed to query %s: %v", tableName, err)
	}

	processedRecords := make([]map[string]interface{}, 0, len(expectedRecords))
	for i, expectedRecord := range expectedRecords {
		processedRecord, err := f.wildcardProcessor.ResolveWildcards(expectedRecord)
		if err != nil {
			return fmt.Errorf("failed to process wildcards in expected record %d: %v", i, err)
		}

		recordMap, err := convertToStringInterfaceMap(processedRecord)
		if err != nil {
			return fmt.Errorf("failed to convert processed record %d: %v", i, err)
		}
		processedRecords = append(processedRecords, recordMap)

		found := false
		var detailedErrors []string
		for j, result := range allResults {
			matchResult := f.matchesPartialRecordWithDetails(recordMap, result)
			if matchResult.matches {
				found = true
				break
			} else {
				detailedErrors = append(detailedErrors, fmt.Sprintf("Record %d: %s", j+1, matchResult.errorMessage))
			}
		}

		if !found {
			errorMsg := fmt.Sprintf("❌ Expected record %d not found in table '%s'\n\n", i+1, tableName)

			errorMsg += f.formatRecordAsYAML(recordMap, "🎯 Expected")
			errorMsg += "\n"

			errorMsg += fmt.Sprintf("🗃️  Available records in table '%s':\n", tableName)
			if len(allResults) == 0 {
				errorMsg += "  (table is empty)\n"
			} else {
				errorMsg += f.formatRecordsAsSimpleList(allResults)
			}

			errorMsg += "🔍 Field-by-field comparison:\n"
			for j, detailErr := range detailedErrors {
				errorMsg += fmt.Sprintf("   Record %d: %s\n", j+1, detailErr)
			}

			errorMsg += "\n💡 Tip: Check wildcards ($UUID$, $NOW$, etc.) and data types.\n"
			return fmt.Errorf("%s", errorMsg)
		}
	}
	if !f.matchDistinctRecords(processedRecords, allResults) {
		return fmt.Errorf("expected records cannot be matched to distinct rows in table '%s'\n%s", tableName, f.formatRecordsAsSimpleList(allResults))
	}

	return nil
}

func (f *FeaturesContext) haveRecordsWithin(timeoutValue, tableName string, data *godog.Table) error {
	return waitForAssertion(timeoutValue, fmt.Sprintf("table %q to contain the expected records", tableName), func() error {
		return f.haveRecords(tableName, data)
	})
}

func (f *FeaturesContext) matchDistinctRecords(expected, actual []map[string]interface{}) bool {
	used := make([]bool, len(actual))
	var match func(int) bool
	match = func(i int) bool {
		if i == len(expected) {
			return true
		}
		for j, row := range actual {
			if used[j] || !f.matchesPartialRecordWithDetails(expected[i], row).matches {
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
	return match(0)
}

func (f *FeaturesContext) haveEmptyTable(tableName string) error {
	if f.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return f.haveRecordsCount("0", tableName)
}

func (f *FeaturesContext) haveRecordsCount(expectedCount string, tableName string) error {
	if f.db == nil {
		return fmt.Errorf("database not initialized")
	}

	exCountResolved, err := f.wildcardProcessor.ResolveWildcards(expectedCount)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in expected record count: %v", err)
	}

	actualCount, err := f.db.Count(tableName)
	if err != nil {
		return fmt.Errorf("failed to count records in %s: %v", tableName, err)
	}

	exCount, err := strconv.Atoi(fmt.Sprint(exCountResolved))
	if err != nil {
		return fmt.Errorf("failed to convert record count to integer: %v", err)
	}
	if int64(exCount) != actualCount {
		return fmt.Errorf("table %s has %d records, expected %s", tableName, actualCount, expectedCount)
	}

	return nil
}

func (f *FeaturesContext) executeRawSQL(query string) error {
	if f.db == nil {
		return fmt.Errorf("database not initialized")
	}

	processedQuery, err := f.wildcardProcessor.SubstituteWildcards(query)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in SQL query: %w", err)
	}
	queryStr, ok := processedQuery.(string)
	if !ok {
		return fmt.Errorf("processed SQL query is not a string: %T", processedQuery)
	}
	err = f.db.Exec(queryStr)
	if err != nil {
		return fmt.Errorf("failed to execute SQL query: %v", err)
	}

	return nil
}

func (f *FeaturesContext) matchesPartialRecordWithDetails(expected, actual map[string]interface{}) MatchResult {
	var errors []string

	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			errors = append(errors, fmt.Sprintf("❌ Field '%s' missing in database record", key))
			continue
		}

		expectedStr := fmt.Sprintf("%v", expectedValue)
		actualStr := fmt.Sprintf("%v", actualValue)

		if expectedStr == "<nil>" {
			if actualValue == nil {
				continue
			} else {
				errors = append(errors, fmt.Sprintf("널 Field '%s': expected NULL but got '%s'", key, actualStr))
				continue
			}
		}
		if expectedStr == "$ANY$" {
			continue
		}

		if actualValue == nil {
			errors = append(errors, fmt.Sprintf("널 Field '%s': expected '%s' but got NULL", key, expectedStr))
			continue
		}
		if matcher, ok := f.wildcardProcessor.FindHandlerForWildcard(expectedStr).(WildcardMatcher); ok {
			if err := matcher.Match(expectedStr, actualValue); err != nil {
				errors = append(errors, fmt.Sprintf("Field '%s': %v", key, err))
			}
			continue
		}

		if strings.HasPrefix(expectedStr, "$NOW") {
			if f.matchesTimeWildcard(expectedStr, actualStr) {
				continue
			} else {
				errors = append(errors, fmt.Sprintf("⏰ Field '%s': time wildcard '%s' doesn't match actual time '%s'", key, expectedStr, actualStr))
				continue
			}
		}

		if expectedStr == "$UUID$" {
			if f.matchesUUIDWildcard(actualStr) {
				continue
			} else {
				errors = append(errors, fmt.Sprintf("🆔 Field '%s': expected UUID format but got '%s'", key, actualStr))
				continue
			}
		}

		if expectedStr == "$NULL$" {
			if actualValue == nil {
				continue
			} else {
				errors = append(errors, fmt.Sprintf("널 Field '%s': expected NULL but got '%s'", key, actualStr))
				continue
			}
		}

		switch expectedStr {
		case "$ANY$":
			continue
		case "$NOT_NULL$":
			if actualValue == nil {
				errors = append(errors, fmt.Sprintf("Field '%s': expected non-null value but got NULL", key))
			}
			continue
		case "$ANY_ARRAY$":
			if _, ok := actualValue.([]interface{}); !ok {
				errors = append(errors, fmt.Sprintf("Field '%s': expected array type but got %v (%T)", key, actualValue, actualValue))
			}
			continue
		case "$ANY_STR$":
			if _, ok := actualValue.(string); !ok {
				errors = append(errors, fmt.Sprintf("📝 Field '%s': expected string type but got %v (%T)", key, actualValue, actualValue))
			}
			continue
		case "$ANY_STR_FULL$":
			s, ok := actualValue.(string)
			if !ok || s == "" {
				errors = append(errors, fmt.Sprintf("📝 Field '%s': expected non-empty string but got %v (%T)", key, actualValue, actualValue))
			}
			continue
		case "$ANY_INT$":
			if !IsInteger(actualValue) {
				errors = append(errors, fmt.Sprintf("🔢 Field '%s': expected integer type but got %v (%T)", key, actualValue, actualValue))
			}
			continue
		case "$ANY_FLOAT$":
			if !IsFloat(actualValue) {
				errors = append(errors, fmt.Sprintf("🔢 Field '%s': expected float type but got %v (%T)", key, actualValue, actualValue))
			}
			continue
		case "$ANY_BOOL$":
			if _, ok := actualValue.(bool); !ok {
				errors = append(errors, fmt.Sprintf("✅ Field '%s': expected boolean type but got %v (%T)", key, actualValue, actualValue))
			}
			continue
		}

		expectedFloat, err1 := strconv.ParseFloat(expectedStr, 64)
		actualFloat, err2 := ToFloat64(actualValue)
		if err1 == nil && err2 == nil {
			if expectedFloat != actualFloat {
				errors = append(errors, fmt.Sprintf("🔢 Field '%s': expected numeric value %g but got %g", key, expectedFloat, actualFloat))
			}
			continue
		}

		if expectedStr != actualStr {
			errors = append(errors, fmt.Sprintf("📝 Field '%s': expected '%s' but got '%s'", key, expectedStr, actualStr))
		}
	}

	if len(errors) > 0 {
		return MatchResult{
			matches:      false,
			errorMessage: strings.Join(errors, "; "),
		}
	}

	return MatchResult{
		matches:      true,
		errorMessage: "",
	}
}

func (f *FeaturesContext) matchesUUIDWildcard(actual string) bool {
	return isValidUUID(actual)
}

func (f *FeaturesContext) matchesTimeWildcard(expected, actual string) bool {
	now := time.Now()
	expectedTime, tolerance, err := expectedTimeAndTolerance(expected, now)
	if err != nil {
		return false
	}
	actualTime, err := parseWildcardTime(actual)
	if err != nil {
		return false
	}
	diff := expectedTime.Sub(actualTime)
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}

// Supports type prefixes in variable name: str:varname, int:varname, float:varname, bool:varname, auto:varname
// Examples:
//
//	extract from database "SELECT price FROM products LIMIT 1" as "amount"         -> preserves original type
//	extract from database "SELECT price FROM products LIMIT 1" as "str:amount"     -> forces string type
//	extract from database "SELECT count FROM stats LIMIT 1" as "int:total"        -> forces integer type
func (f *FeaturesContext) extractVariableFromDatabase(query, variableName string) error {
	if f.db == nil {
		return fmt.Errorf("database not initialized%s", f.wildcardProcessor.GetVariablesDebugInfo())
	}

	actualVarName, typePrefix := f.parseVariableNameWithPrefix(variableName)

	processedQuery, err := f.wildcardProcessor.SubstituteWildcards(query)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in query: %v%s", err, f.wildcardProcessor.GetVariablesDebugInfo())
	}

	queryStr, ok := processedQuery.(string)
	if !ok {
		return fmt.Errorf("processed query is not a string: %T%s", processedQuery, f.wildcardProcessor.GetVariablesDebugInfo())
	}

	results, err := f.db.Query(queryStr)
	if err != nil {
		return fmt.Errorf("failed to execute query '%s': %v%s", queryStr, err, f.wildcardProcessor.GetVariablesDebugInfo())
	}

	if len(results) == 0 {
		return fmt.Errorf("query '%s' returned no results%s", queryStr, f.wildcardProcessor.GetVariablesDebugInfo())
	}

	if len(results) > 1 {
		return fmt.Errorf("query '%s' returned %d results, expected exactly 1%s", queryStr, len(results), f.wildcardProcessor.GetVariablesDebugInfo())
	}

	resultMap := results[0]

	var finalValue interface{}

	if len(resultMap) == 1 {
		for _, value := range resultMap {
			finalValue = value
			break
		}
	} else {
		finalValue = resultMap
	}

	if value, ok := finalValue.(*interface{}); ok {
		finalValue = *value
	}
	convertedValue := f.convertValueWithTypePrefix(finalValue, typePrefix)

	f.wildcardProcessor.SetVariable(actualVarName, convertedValue)
	return nil
}

func (f *FeaturesContext) convertStringToAppropriateTypeForDB(value interface{}) interface{} {
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
