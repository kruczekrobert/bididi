package bdd

import (
	"encoding/json"
	"net/http"
	"github.com/cucumber/godog"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMakeJsonRequestWithHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	f := &FeaturesContext{
		mockRegistry:      make(map[string]*MockConfig),
		wildcardProcessor: NewWildcardProcessor(),
		jsonComparator:    NewJSONComparator(NewWildcardProcessor()),
		apiApp:            handler,
	}

	t.Run("PayloadWithHeaders", func(t *testing.T) {
		docString := &godog.DocString{
			Content: `{
				"payload": {"name": "John", "age": 30},
				"headers": {"Authorization": "Bearer token123", "X-Custom": "value"}
			}`,
		}

		err := f.makeJsonRequest("POST", "/test", docString)

		assert.NoError(t, err)
	})

	t.Run("HeadersOnly", func(t *testing.T) {
		docString := &godog.DocString{
			Content: `{
				"headers": {"Authorization": "Bearer token123", "X-Custom": "value"}
			}`,
		}

		err := f.makeJsonRequest("GET", "/test", docString)

		assert.NoError(t, err)
	})

	t.Run("PayloadOnly", func(t *testing.T) {
		docString := &godog.DocString{
			Content: `{
				"payload": {"name": "John", "age": 30}
			}`,
		}

		err := f.makeJsonRequest("POST", "/test", docString)

		assert.NoError(t, err)
	})

	t.Run("EmptyRequest", func(t *testing.T) {
		docString := &godog.DocString{
			Content: `{}`,
		}

		err := f.makeJsonRequest("GET", "/test", docString)

		assert.NoError(t, err)
	})

	t.Run("HeadersWithWildcards", func(t *testing.T) {
		f.wildcardProcessor.SetVariable("token", "test-token-123")

		docString := &godog.DocString{
			Content: `{
				"headers": {"Authorization": "Bearer $VAR(token)$", "X-Custom": "value"}
			}`,
		}

		err := f.makeJsonRequest("GET", "/test", docString)

		assert.NoError(t, err)
	})
}

func TestRequestDataUnmarshaling(t *testing.T) {
	t.Run("UnmarshalRequestData", func(t *testing.T) {
		jsonData := `{
			"payload": {"name": "John", "age": 30},
			"headers": {"Authorization": "Bearer token123"}
		}`

		var wrappedPayload requestData
		err := json.Unmarshal([]byte(jsonData), &wrappedPayload)

		assert.NoError(t, err)
		assert.NotNil(t, wrappedPayload.Payload)
		assert.NotNil(t, wrappedPayload.Headers)
		assert.Equal(t, "John", wrappedPayload.Payload["name"])
		assert.Equal(t, "Bearer token123", wrappedPayload.Headers["Authorization"])
	})

	t.Run("UnmarshalPayloadOnly", func(t *testing.T) {
		jsonData := `{
			"payload": {"name": "John", "age": 30}
		}`

		var wrappedPayload requestData
		err := json.Unmarshal([]byte(jsonData), &wrappedPayload)

		assert.NoError(t, err)
		assert.NotNil(t, wrappedPayload.Payload)
		assert.Nil(t, wrappedPayload.Headers)
		assert.Equal(t, "John", wrappedPayload.Payload["name"])
	})

	t.Run("UnmarshalHeadersOnly", func(t *testing.T) {
		jsonData := `{
			"headers": {"Authorization": "Bearer token123"}
		}`

		var wrappedPayload requestData
		err := json.Unmarshal([]byte(jsonData), &wrappedPayload)

		assert.NoError(t, err)
		assert.Nil(t, wrappedPayload.Payload)
		assert.NotNil(t, wrappedPayload.Headers)
		assert.Equal(t, "Bearer token123", wrappedPayload.Headers["Authorization"])
	})

	t.Run("UnmarshalEmpty", func(t *testing.T) {
		jsonData := `{}`

		var wrappedPayload requestData
		err := json.Unmarshal([]byte(jsonData), &wrappedPayload)

		assert.NoError(t, err)
		assert.Nil(t, wrappedPayload.Payload)
		assert.Nil(t, wrappedPayload.Headers)
	})
}

func TestProcessHeaderWildcards(t *testing.T) {
	processor := NewWildcardProcessor()
	processor.SetVariable("user_id", "user-123")
	processor.SetVariable("api_key", "secret-key")

	f := &FeaturesContext{
		wildcardProcessor: processor,
	}

	t.Run("ProcessWildcardsInHeaders", func(t *testing.T) {
		headers := map[string]string{
			"Authorization": "Bearer $VAR(api_key)$",
			"X-User-ID":     "$VAR(user_id)$",
			"X-Static":      "static-value",
		}

		processedHeaders, err := f.processHeaderWildcards(headers)
		assert.NoError(t, err)

		assert.Equal(t, "Bearer secret-key", processedHeaders["Authorization"])
		assert.Equal(t, "user-123", processedHeaders["X-User-ID"])
		assert.Equal(t, "static-value", processedHeaders["X-Static"])
	})

	t.Run("ProcessNilHeaders", func(t *testing.T) {
		processedHeaders, err := f.processHeaderWildcards(nil)
		assert.NoError(t, err)
		assert.Nil(t, processedHeaders)
	})

	t.Run("ProcessEmptyHeaders", func(t *testing.T) {
		headers := map[string]string{}
		processedHeaders, err := f.processHeaderWildcards(headers)
		assert.NoError(t, err)
		assert.Empty(t, processedHeaders)
	})
}
