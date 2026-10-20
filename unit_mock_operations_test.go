package bdd

import (
	"net/url"
	"testing"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONMockFromTable(t *testing.T) {
	f := New()
	f.wildcardProcessor.SetVariable("source", "checkout")
	f.wildcardProcessor.SetVariable("reply", "ok")
	table := httpTestTable(
		[]string{"type", "name", "value"},
		[]string{"query", "source", "$VAR(source)$"},
		[]string{"status", "", "200"},
		[]string{"response-header", "X-Mock", "$VAR(reply)$"},
	)
	require.NoError(t, f.jsonMockFromTable("notification", "POST", "events", table, &godog.DocString{Content: `{"accepted":true}`}))
	mock := f.mockRegistry["notification"]
	require.NotNil(t, mock)
	assert.Equal(t, "POST", mock.Method)
	assert.Equal(t, "events?source=checkout", mock.Path)
	assert.Equal(t, 200, mock.Status)
	assert.Equal(t, "ok", mock.Headers["X-Mock"])
	assert.Equal(t, true, mock.Response["accepted"])
	assert.False(t, mock.Called)
}

func TestJSONMockFromTableRejectsInvalidInput(t *testing.T) {
	f := New()
	validTable := httpTestTable(
		[]string{"type", "name", "value"},
		[]string{"status", "", "200"},
	)
	for _, test := range []struct {
		name  string
		table *godog.Table
		body  *godog.DocString
	}{
		{"missing body", validTable, nil},
		{"invalid JSON", validTable, &godog.DocString{Content: `{"accepted":`}},
		{"missing status", httpTestTable([]string{"type", "name", "value"}), &godog.DocString{Content: `{}`}},
		{"method in table", httpTestTable([]string{"type", "name", "value"}, []string{"method", "", "POST"}, []string{"status", "", "200"}), &godog.DocString{Content: `{}`}},
		{"invalid status", httpTestTable([]string{"type", "name", "value"}, []string{"status", "", "0"}), &godog.DocString{Content: `{}`}},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, f.jsonMockFromTable("notification", "POST", "events", test.table, test.body))
		})
	}
}

func TestMockCallAssertionChecksRequestAndResponse(t *testing.T) {
	f := New()
	f.wildcardProcessor.SetVariable("source", "checkout")
	f.mockRegistry["notification"] = &MockConfig{
		Called: true,
		CallHistory: []MockCall{{
			Query:           url.Values{"source": {"checkout"}},
			Headers:         map[string]string{"Content-Type": "application/json"},
			ResponseHeaders: map[string]string{"X-Mock": "from-mock"},
			Payload:         map[string]interface{}{"id": "42"},
		}},
	}
	table := httpTestTable(
		[]string{"type", "name", "value"},
		[]string{"query", "source", "$VAR(source)$"},
		[]string{"header", "Content-Type", "$ANY_STR_FULL$"},
		[]string{"response-header", "X-Mock", "from-mock"},
	)
	require.NoError(t, f.mockWasCalledWithTable("notification", table, &godog.DocString{Content: `{"id":"$ANY_STR_FULL$"}`}))
	require.NoError(t, f.mockWasCalledWithTableWithoutBody("notification", table))

	wrongResponseHeader := httpTestTable(
		[]string{"type", "name", "value"},
		[]string{"response-header", "X-Mock", "other"},
	)
	require.Error(t, f.mockWasCalledWithTableWithoutBody("notification", wrongResponseHeader))
	wrongQuery := httpTestTable(
		[]string{"type", "name", "value"},
		[]string{"query", "source", "other"},
	)
	require.Error(t, f.mockWasCalledWithTableWithoutBody("notification", wrongQuery))
}

func TestMockCalledAndNotCalled(t *testing.T) {
	f := New()
	f.mockRegistry["called"] = &MockConfig{Called: true}
	f.mockRegistry["unused"] = &MockConfig{}
	require.NoError(t, f.jsonMockWasCalled("called"))
	require.NoError(t, f.jsonMockWasNotCalled("unused"))
	require.Error(t, f.jsonMockWasCalled("unused"))
	require.Error(t, f.jsonMockWasNotCalled("called"))
	require.Error(t, f.jsonMockWasCalled("missing"))
}
