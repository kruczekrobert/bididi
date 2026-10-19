package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

type RedisClient interface {
	Exists(ctx context.Context, key string) (bool, error)
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

func parseRedisTTL(value string) (time.Duration, error) {
	ttl, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid Redis TTL %q: %w", value, err)
	}
	if ttl <= 0 {
		return 0, fmt.Errorf("Redis TTL must be positive: %q", value)
	}
	return ttl, nil
}

func (f *FeaturesContext) assertRedisKeyExists(key string) error {
	if f.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	processedKey, err := f.wildcardProcessor.ResolveWildcards(key)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in key: %v", err)
	}

	keyStr, ok := processedKey.(string)
	if !ok {
		return fmt.Errorf("processed key is not a string: %T", processedKey)
	}

	exists, err := f.redis.Exists(context.Background(), keyStr)
	if err != nil {
		return fmt.Errorf("failed to check if key exists: %v", err)
	}

	if !exists {
		return fmt.Errorf("key '%s' does not exist in Redis", keyStr)
	}

	return nil
}

func (f *FeaturesContext) assertRedisKeyDoesNotExists(key string) error {
	if f.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	processedKey, err := f.wildcardProcessor.ResolveWildcards(key)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in key: %v", err)
	}

	keyStr, ok := processedKey.(string)
	if !ok {
		return fmt.Errorf("processed key is not a string: %T", processedKey)
	}

	exists, err := f.redis.Exists(context.Background(), keyStr)
	if err != nil {
		return fmt.Errorf("failed to check if key exists: %v", err)
	}

	if exists {
		return fmt.Errorf("key '%s' exists in Redis but it shouldn't", keyStr)
	}

	return nil
}

func (f *FeaturesContext) assertRedisValueJson(key string, expectedJSON *godog.DocString) error {
	if f.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	if expectedJSON == nil {
		return fmt.Errorf("expected Redis JSON doc string is missing")
	}
	processedKey, err := f.wildcardProcessor.ResolveWildcards(key)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in key: %v", err)
	}

	keyStr, ok := processedKey.(string)
	if !ok {
		return fmt.Errorf("processed key is not a string: %T", processedKey)
	}

	actualValue, found, err := f.redis.Get(context.Background(), keyStr)
	if err != nil {
		return fmt.Errorf("failed to get key '%s': %v", keyStr, err)
	}
	if !found {
		return fmt.Errorf("key '%s' does not exist in Redis", keyStr)
	}

	if err := f.jsonComparator.CompareJSON(expectedJSON.Content, actualValue); err != nil {
		return fmt.Errorf("key '%s' JSON content does not match: %v", keyStr, err)
	}

	return nil
}

func (f *FeaturesContext) assertRedisValueString(key string, expectedValue *godog.DocString) error {
	if f.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	if expectedValue == nil {
		return fmt.Errorf("expected Redis string doc string is missing")
	}
	processedKey, err := f.wildcardProcessor.ResolveWildcards(key)
	if err != nil {
		return fmt.Errorf("failed to process wildcards in key: %v", err)
	}

	keyStr, ok := processedKey.(string)
	if !ok {
		return fmt.Errorf("processed key is not a string: %T", processedKey)
	}

	actualValue, found, err := f.redis.Get(context.Background(), keyStr)
	if err != nil {
		return fmt.Errorf("failed to get key '%s': %v", keyStr, err)
	}
	if !found {
		return fmt.Errorf("key '%s' does not exist in Redis", keyStr)
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

func (f *FeaturesContext) setRedisValue(key string, value *godog.DocString) error {
	return f.setRedisValueForTTL(key, value, 0)
}

func (f *FeaturesContext) setRedisValueWithTTL(key, ttlValue string, value *godog.DocString) error {
	ttl, err := parseRedisTTL(ttlValue)
	if err != nil {
		return err
	}
	return f.setRedisValueForTTL(key, value, ttl)
}

func (f *FeaturesContext) setRedisValueForTTL(key string, value *godog.DocString, ttl time.Duration) error {
	if f.redis == nil {
		return fmt.Errorf("redis not configured")
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

	err = f.redis.Set(context.Background(), keyStr, valStr, ttl)
	if err != nil {
		return fmt.Errorf("failed to set key '%s': %v", keyStr, err)
	}

	return nil
}

func (f *FeaturesContext) setRedisJSONValue(key string, value *godog.DocString) error {
	return f.setRedisJSONValueForTTL(key, value, 0)
}

func (f *FeaturesContext) setRedisJSONValueWithTTL(key, ttlValue string, value *godog.DocString) error {
	ttl, err := parseRedisTTL(ttlValue)
	if err != nil {
		return err
	}
	return f.setRedisJSONValueForTTL(key, value, ttl)
}

func (f *FeaturesContext) setRedisJSONValueForTTL(key string, value *godog.DocString, ttl time.Duration) error {
	if f.redis == nil {
		return fmt.Errorf("redis not configured")
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

	err = f.redis.Set(context.Background(), keyStr, valueToStore, ttl)
	if err != nil {
		return fmt.Errorf("failed to set key '%s': %v", keyStr, err)
	}

	return nil
}
