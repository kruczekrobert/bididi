package bdd

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Context string

const (
	ContextGeneration Context = "generation" // For generating real values
	ContextComparison Context = "comparison" // For keeping wildcards as placeholders
)

type WildcardHandler interface {
	CanHandle(wildcard string) bool
	Process(wildcard string, context Context) (interface{}, error)
	ShouldBeQuotedInJSON(wildcard string, processedValue interface{}) bool
}

type WildcardMatcher interface {
	Match(wildcard string, actual interface{}) error
}

type WildcardRegistry struct {
	handlers []WildcardHandler
}

func NewWildcardRegistry() *WildcardRegistry {
	return &WildcardRegistry{
		handlers: make([]WildcardHandler, 0),
	}
}

func (wr *WildcardRegistry) RegisterHandler(handler WildcardHandler) {
	wr.handlers = append(wr.handlers, handler)
}

func (wr *WildcardRegistry) ProcessWildcard(wildcard string, context Context) (interface{}, error) {
	for _, handler := range wr.handlers {
		if handler.CanHandle(wildcard) {
			return handler.Process(wildcard, context)
		}
	}
	return wildcard, nil
}

type VariableHandler struct {
	variables map[string]interface{}
}

func NewVariableHandler() *VariableHandler {
	return &VariableHandler{
		variables: make(map[string]interface{}),
	}
}

func (vh *VariableHandler) CanHandle(wildcard string) bool {
	return strings.HasPrefix(wildcard, "$VAR(") && strings.HasSuffix(wildcard, ")$")
}

func (vh *VariableHandler) Process(wildcard string, context Context) (interface{}, error) {
	re := regexp.MustCompile(`\$VAR\(([^)]+)\)\$`)
	matches := re.FindStringSubmatch(wildcard)
	if len(matches) < 2 {
		return wildcard, fmt.Errorf("invalid variable wildcard: %s", wildcard)
	}

	name := matches[1]
	if val, ok := vh.variables[name]; ok {
		return val, nil
	}

	return nil, fmt.Errorf("variable '%s' not found - make sure to set it with 'set variable' step before using%s", name, vh.getVariablesDebugInfo())
}

func (vh *VariableHandler) SetVariable(name string, value interface{}) {
	vh.variables[name] = value
}

func (vh *VariableHandler) GetVariable(name string) (interface{}, bool) {
	value, exists := vh.variables[name]
	return value, exists
}

func (vh *VariableHandler) ClearVariables() {
	vh.variables = make(map[string]interface{})
}

func (vh *VariableHandler) getVariablesDebugInfo() string {
	if len(vh.variables) == 0 {
		return ". No variables are currently set."
	}

	var debugInfo strings.Builder
	debugInfo.WriteString("\n\n🔧 Currently stored variables:")

	var names []string
	for name := range vh.variables {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		value := vh.variables[name]
		typeInfo := fmt.Sprintf("%T", value)
		valueStr := fmt.Sprintf("%v", value)

		debugInfo.WriteString(fmt.Sprintf("\n   • %s = %s (%s)", name, valueStr, typeInfo))
	}

	debugInfo.WriteString("\n\n💡 Use 'set variable \"name\" to \"value\"' to set variables before using $VAR(name)$")
	return debugInfo.String()
}

func (vh *VariableHandler) ShouldBeQuotedInJSON(wildcard string, processedValue interface{}) bool {
	switch processedValue.(type) {
	case string:
		return true // Strings need quotes
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return false // Numbers don't need quotes
	case bool:
		return false // Booleans don't need quotes
	default:
		return true // Fallback to quoted (safe default)
	}
}

type TimeHandler struct{}

func NewTimeHandler() *TimeHandler {
	return &TimeHandler{}
}

func (th *TimeHandler) CanHandle(wildcard string) bool {
	return wildcard == "$NOW$" || regexp.MustCompile(`^\$NOW[+-][^$]+\$$`).MatchString(wildcard)
}

func (th *TimeHandler) Process(wildcard string, context Context) (interface{}, error) {
	if context == ContextComparison {
		return wildcard, nil
	}

	now := time.Now()

	if wildcard == "$NOW$" {
		return now.Format(time.RFC3339), nil
	}

	// Handle $NOW+duration$ or $NOW-duration$
	re := regexp.MustCompile(`^\$NOW([+-])(.+)\$$`)
	matches := re.FindStringSubmatch(wildcard)
	if len(matches) == 3 {
		operator := matches[1]
		durationStr := matches[2]

		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			return nil, fmt.Errorf("invalid duration %s: %v", durationStr, err)
		}

		if operator == "+" {
			return now.Add(duration).Format(time.RFC3339), nil
		} else {
			return now.Add(-duration).Format(time.RFC3339), nil
		}
	}

	return wildcard, nil
}

func (th *TimeHandler) ShouldBeQuotedInJSON(wildcard string, processedValue interface{}) bool {
	return true
}

type UUIDHandler struct{}

func NewUUIDHandler() *UUIDHandler {
	return &UUIDHandler{}
}

func (uh *UUIDHandler) CanHandle(wildcard string) bool {
	return wildcard == "$UUID$"
}

func (uh *UUIDHandler) Process(wildcard string, context Context) (interface{}, error) {
	if context == ContextComparison {
		return wildcard, nil
	}
	return uuid.New().String(), nil
}

func (uh *UUIDHandler) ShouldBeQuotedInJSON(wildcard string, processedValue interface{}) bool {
	return true
}

type NullHandler struct{}

func NewNullHandler() *NullHandler {
	return &NullHandler{}
}

func (nh *NullHandler) CanHandle(wildcard string) bool {
	return wildcard == "$NULL$"
}

func (nh *NullHandler) Process(wildcard string, context Context) (interface{}, error) {
	return nil, nil
}

func (nh *NullHandler) ShouldBeQuotedInJSON(wildcard string, processedValue interface{}) bool {
	return false
}

type GenericHandler struct{}

func NewGenericHandler() *GenericHandler {
	return &GenericHandler{}
}

func (gh *GenericHandler) CanHandle(wildcard string) bool {
	switch wildcard {
	case "$ANY$", "$ANY_STR$", "$ANY_STR_FULL$", "$ANY_INT$", "$ANY_FLOAT$", "$ANY_BOOL$":
		return true
	default:
		return false
	}
}

func (gh *GenericHandler) Process(wildcard string, context Context) (interface{}, error) {
	if context == ContextComparison {
		return wildcard, nil
	}

	switch wildcard {
	case "$ANY$", "$ANY_STR$", "$ANY_STR_FULL$", "$ANY_INT$", "$ANY_FLOAT$", "$ANY_BOOL$":
		return wildcard, fmt.Errorf("$ANY* wildcards are only supported for comparisons/validation, not for data generation")
	}

	return wildcard, nil
}

func (gh *GenericHandler) ShouldBeQuotedInJSON(wildcard string, processedValue interface{}) bool {
	return true
}

type WildcardProcessor struct {
	registry        *WildcardRegistry
	variableHandler *VariableHandler
	patterns        []*regexp.Regexp
}

func NewWildcardProcessor() *WildcardProcessor {
	registry := NewWildcardRegistry()
	variableHandler := NewVariableHandler()

	registry.RegisterHandler(variableHandler)
	registry.RegisterHandler(NewTimeHandler())
	registry.RegisterHandler(NewUUIDHandler())
	registry.RegisterHandler(NewGenericHandler())
	registry.RegisterHandler(NewNullHandler())

	return &WildcardProcessor{
		registry:        registry,
		variableHandler: variableHandler,
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`\$VAR\([^)]+\)\$`),
			regexp.MustCompile(`\$NOW[^$]*\$`),
			regexp.MustCompile(`\$UUID\$`),
			regexp.MustCompile(`\$ANY[^$]*\$`),
			regexp.MustCompile(`\$NULL\$`),
		},
	}
}

func (wp *WildcardProcessor) RegisterWildcard(pattern string, handler WildcardHandler) error {
	if handler == nil {
		return fmt.Errorf("wildcard handler is nil")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid wildcard pattern: %w", err)
	}
	wp.registry.RegisterHandler(handler)
	wp.patterns = append(wp.patterns, re)
	return nil
}

func (wp *WildcardProcessor) ResolveWildcards(data interface{}) (interface{}, error) {
	return wp.processData(data, ContextComparison)
}

func (wp *WildcardProcessor) SubstituteWildcards(data interface{}) (interface{}, error) {
	return wp.processData(data, ContextGeneration)
}

func (wp *WildcardProcessor) processData(data interface{}, context Context) (interface{}, error) {
	switch v := data.(type) {
	case string:
		return wp.processString(v, context)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			processedValue, err := wp.processData(value, context)
			if err != nil {
				return nil, err
			}
			processedKey, err := wp.processString(key, context)
			if err != nil {
				return nil, err
			}
			keyString, ok := processedKey.(string)
			if !ok {
				return nil, fmt.Errorf("wildcard in map key must resolve to a string, got %T", processedKey)
			}
			result[keyString] = processedValue
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, value := range v {
			processedValue, err := wp.processData(value, context)
			if err != nil {
				return nil, err
			}
			result[i] = processedValue
		}
		return result, nil
	case MockResponse:
		processedMap, err := wp.processData(v.Response, context)
		if err != nil {
			return nil, err
		}
		v.Response = processedMap.(map[string]interface{})
		return v, nil
	default:
		return data, nil
	}
}

func (wp *WildcardProcessor) processString(s string, context Context) (interface{}, error) {
	switch wp.countWildcards(s) {
	case 0:
		return s, nil
	case 1:
		if result, err := wp.registry.ProcessWildcard(s, context); err != nil {
			return nil, err
		} else if result != s {
			return result, nil
		}
		return wp.processMultipleWildcards(s, context)
	default:
		return wp.processMultipleWildcards(s, context)
	}
}

func (wp *WildcardProcessor) countWildcards(s string) int {
	wildcards := 0
	for _, re := range wp.patterns {
		wildcards += len(re.FindAllString(s, -1))
	}

	return wildcards
}

func (wp *WildcardProcessor) processMultipleWildcards(s string, context Context) (interface{}, error) {
	result := s

	for _, re := range wp.patterns {
		matches := re.FindAllString(result, -1)

		for _, match := range matches {
			processed, err := wp.registry.ProcessWildcard(match, context)
			if err != nil {
				return nil, err
			}

			// For nil values (like $NULL$), we need to handle them specially
			// If the entire string is just the $NULL$ wildcard, return nil directly
			if processed == nil && match == s && len(matches) == 1 {
				return nil, nil
			}

			// Convert processed value to string for replacement in mixed strings
			// This is necessary when wildcards are part of a larger string
			var replacement string
			switch v := processed.(type) {
			case string:
				replacement = v
			case nil:
				replacement = "" // For $ANY$ and $NULL$ wildcards that return nil
			case float64:
				replacement = fmt.Sprintf("%g", v)
			case float32:
				replacement = fmt.Sprintf("%g", v)
			case int, int8, int16, int32, int64:
				replacement = fmt.Sprintf("%d", v)
			case uint, uint8, uint16, uint32, uint64:
				replacement = fmt.Sprintf("%d", v)
			case bool:
				replacement = fmt.Sprintf("%t", v)
			default:
				replacement = fmt.Sprintf("%v", v)
			}

			result = strings.ReplaceAll(result, match, replacement)
		}
	}

	return result, nil
}

func (wp *WildcardProcessor) SetVariable(name string, value interface{}) {
	wp.variableHandler.SetVariable(name, value)
}

func (wp *WildcardProcessor) GetVariable(name string) (interface{}, bool) {
	return wp.variableHandler.GetVariable(name)
}

func (wp *WildcardProcessor) ClearVariables() {
	wp.variableHandler.ClearVariables()
}

func (wp *WildcardProcessor) FindHandlerForWildcard(wildcard string) WildcardHandler {
	for _, handler := range wp.registry.handlers {
		if handler.CanHandle(wildcard) {
			return handler
		}
	}
	return nil
}

func (wp *WildcardProcessor) GetVariablesDebugInfo() string {
	return wp.variableHandler.getVariablesDebugInfo()
}
