package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

type Cache interface {
	Exists(ctx context.Context, key string) (bool, error)
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

func parseCacheTTL(value string) (time.Duration, error) {
	ttl, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid cache TTL %q: %w", value, err)
	}
	if ttl <= 0 {
		return 0, fmt.Errorf("cache TTL must be positive: %q", value)
	}
	return ttl, nil
}

func (f *FeaturesContext) assertCacheKeyExists(key string) error {
	if f.cache == nil {
		return fmt.Errorf("cache not configured")
	}
	processedKey, err := f.wildcardProcessor.ResolveWildcards(key)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in key: %v", err)
	}

	keyStr, ok := processedKey.(string)
	if !ok {
		return fmt.Errorf("processed key is not a string: %T", processedKey)
	}

	exists, err := f.cache.Exists(context.Background(), keyStr)
	if err != nil {
		return fmt.Errorf("failed to check if key exists: %v", err)
	}

	if !exists {
		return fmt.Errorf("key '%s' does not exist in cache", keyStr)
	}

	return nil
}

func (f *FeaturesContext) assertCacheKeyDoesNotExists(key string) error {
	if f.cache == nil {
		return fmt.Errorf("cache not configured")
	}
	processedKey, err := f.wildcardProcessor.ResolveWildcards(key)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in key: %v", err)
	}

	keyStr, ok := processedKey.(string)
	if !ok {
		return fmt.Errorf("processed key is not a string: %T", processedKey)
	}

	exists, err := f.cache.Exists(context.Background(), keyStr)
	if err != nil {
		return fmt.Errorf("failed to check if key exists: %v", err)
	}

	if exists {
		return fmt.Errorf("key '%s' exists in cache but it shouldn't", keyStr)
	}

	return nil
}

func (f *FeaturesContext) assertCacheValueJson(key string, expectedJSON *godog.DocString) error {
	if f.cache == nil {
		return fmt.Errorf("cache not configured")
	}
	if expectedJSON == nil {
		return fmt.Errorf("expected cache JSON doc string is missing")
	}
	processedKey, err := f.wildcardProcessor.ResolveWildcards(key)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in key: %v", err)
	}

	keyStr, ok := processedKey.(string)
	if !ok {
		return fmt.Errorf("processed key is not a string: %T", processedKey)
	}

	actualValue, found, err := f.cache.Get(context.Background(), keyStr)
	if err != nil {
		return fmt.Errorf("failed to get key '%s': %v", keyStr, err)
	}
	if !found {
		return fmt.Errorf("key '%s' does not exist in cache", keyStr)
	}

	if err := f.jsonComparator.CompareJSON(expectedJSON.Content, actualValue); err != nil {
		return fmt.Errorf("key '%s' JSON content does not match: %v", keyStr, err)
	}

	return nil
}

func (f *FeaturesContext) assertCacheValueString(key string, expectedValue *godog.DocString) error {
	if f.cache == nil {
		return fmt.Errorf("cache not configured")
	}
	if expectedValue == nil {
		return fmt.Errorf("expected cache string doc string is missing")
	}
	processedKey, err := f.wildcardProcessor.ResolveWildcards(key)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in key: %v", err)
	}

	keyStr, ok := processedKey.(string)
	if !ok {
		return fmt.Errorf("processed key is not a string: %T", processedKey)
	}

	actualValue, found, err := f.cache.Get(context.Background(), keyStr)
	if err != nil {
		return fmt.Errorf("failed to get key '%s': %v", keyStr, err)
	}
	if !found {
		return fmt.Errorf("key '%s' does not exist in cache", keyStr)
	}

	processedExpected, err := f.wildcardProcessor.ResolveWildcards(expectedValue.Content)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in expected value: %v", err)
	}
	if token, ok := processedExpected.(string); ok && !strings.HasPrefix(expectedValue.Content, "$VAR(") && f.wildcardProcessor.FindHandlerForWildcard(token) != nil {
		if err := f.jsonComparator.compareValues(token, actualValue, keyStr); err != nil {
			return fmt.Errorf("key '%s' string content does not match: %w", keyStr, err)
		}
		return nil
	}

	var expectedStr string
	switch v := processedExpected.(type) {
	case string:
		expectedStr = v
	default:
		expectedStr = fmt.Sprintf("%v", v)
	}

	if actualValue != expectedStr {
		return fmt.Errorf("key '%s' contains '%s', expected '%s'", keyStr, actualValue, expectedStr)
	}

	return nil
}

func (f *FeaturesContext) setCacheValue(key string, value *godog.DocString) error {
	return f.setCacheValueForTTL(key, value, 0)
}

func (f *FeaturesContext) setCacheValueWithTTL(key, ttlValue string, value *godog.DocString) error {
	ttl, err := parseCacheTTL(ttlValue)
	if err != nil {
		return err
	}
	return f.setCacheValueForTTL(key, value, ttl)
}

func (f *FeaturesContext) setCacheValueForTTL(key string, value *godog.DocString, ttl time.Duration) error {
	if f.cache == nil {
		return fmt.Errorf("cache not configured")
	}
	processedKey, err := f.wildcardProcessor.SubstituteWildcards(key)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in key: %v", err)
	}

	keyStr, ok := processedKey.(string)
	if !ok {
		return fmt.Errorf("processed key is not a string: %T", processedKey)
	}

	var valStr string
	if value != nil {
		processedValue, err := f.wildcardProcessor.SubstituteWildcards(value.Content)
		if err != nil {
			return fmt.Errorf("failed to process wildcards in value: %v", err)
		}

		switch v := processedValue.(type) {
		case string:
			valStr = v
		default:
			valStr = fmt.Sprintf("%v", v)
		}
	}

	err = f.cache.Set(context.Background(), keyStr, valStr, ttl)
	if err != nil {
		return fmt.Errorf("failed to set key '%s': %v", keyStr, err)
	}

	return nil
}

func (f *FeaturesContext) setCacheJSONValue(key string, value *godog.DocString) error {
	return f.setCacheJSONValueForTTL(key, value, 0)
}

func (f *FeaturesContext) setCacheJSONValueWithTTL(key, ttlValue string, value *godog.DocString) error {
	ttl, err := parseCacheTTL(ttlValue)
	if err != nil {
		return err
	}
	return f.setCacheJSONValueForTTL(key, value, ttl)
}

func (f *FeaturesContext) setCacheJSONValueForTTL(key string, value *godog.DocString, ttl time.Duration) error {
	if f.cache == nil {
		return fmt.Errorf("cache not configured")
	}
	processedKey, err := f.wildcardProcessor.SubstituteWildcards(key)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in key: %v", err)
	}

	keyStr, ok := processedKey.(string)
	if !ok {
		return fmt.Errorf("processed key is not a string: %T", processedKey)
	}
	valueToStore := "null"
	if value != nil {
		var jsonValue interface{}
		if err := json.Unmarshal([]byte(value.Content), &jsonValue); err != nil {
			return fmt.Errorf("failed to unmarshal JSON: %v", err)
		}

		processedValue, err := f.wildcardProcessor.SubstituteWildcards(jsonValue)
		if err != nil {
			return fmt.Errorf("failed to process wildcards in JSON value: %v", err)
		}
		encodedValue, err := json.Marshal(processedValue)
		if err != nil {
			return fmt.Errorf("failed to encode JSON value: %w", err)
		}
		valueToStore = string(encodedValue)
	}

	err = f.cache.Set(context.Background(), keyStr, valueToStore, ttl)
	if err != nil {
		return fmt.Errorf("failed to set key '%s': %v", keyStr, err)
	}

	return nil
}
