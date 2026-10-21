package bdd

import (
	"net/http"
	"net/http/httptest"
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
	f.wildcardProcessor.SetVariable("status", 200)
	table := httpTestTable(
		[]string{"type", "name", "value"},
		[]string{"query", "source", "$VAR(source)$"},
		[]string{"response-header", "X-Mock", "$VAR(reply)$"},
	)
	require.NoError(t, f.jsonMockFromTable("notification", "POST", "events", "$VAR(status)$", table, &godog.DocString{Content: `{"accepted":true}`}))
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
	for _, test := range []struct {
		name   string
		status string
		table  *godog.Table
		body   *godog.DocString
	}{
		{"missing body", "200", nil, nil},
		{"invalid JSON", "200", nil, &godog.DocString{Content: `{"accepted":`}},
		{"missing status", "", nil, &godog.DocString{Content: `{}`}},
		{"method in table", "200", httpTestTable([]string{"type", "name", "value"}, []string{"method", "", "POST"}), &godog.DocString{Content: `{}`}},
		{"status in table", "200", httpTestTable([]string{"type", "name", "value"}, []string{"status", "", "200"}), &godog.DocString{Content: `{}`}},
		{"invalid status", "0", nil, &godog.DocString{Content: `{}`}},
		{"status too large", "1000", nil, &godog.DocString{Content: `{}`}},
		{"non-numeric status", "invalid", nil, &godog.DocString{Content: `{}`}},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, f.jsonMockFromTable("notification", "POST", "events", test.status, test.table, test.body))
		})
	}
}

func TestMockCallAssertionChecksRequestAndResponse(t *testing.T) {
	f := New()
	f.wildcardProcessor.SetVariable("source", "checkout")
	f.mockRegistry["notification"] = &MockConfig{
		Called: true,
		CallHistory: []MockCall{{
			Status:          200,
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
	require.NoError(t, f.assertMockCallFromTable("notification", "200", table, &godog.DocString{Content: `{"id":"$ANY_STR_FULL$"}`}))
	require.NoError(t, f.assertMockCallFromTable("notification", "", table, nil))
	require.NoError(t, f.assertMockCallFromTable("notification", "200", nil, &godog.DocString{Content: `{"id":"42"}`}))
	require.Error(t, f.assertMockCallFromTable("notification", "200", nil, &godog.DocString{Content: `{"id":"other"}`}))

	wrongResponseHeader := httpTestTable(
		[]string{"type", "name", "value"},
		[]string{"response-header", "X-Mock", "other"},
	)
	require.Error(t, f.assertMockCallFromTable("notification", "", wrongResponseHeader, nil))
	wrongQuery := httpTestTable(
		[]string{"type", "name", "value"},
		[]string{"query", "source", "other"},
	)
	require.Error(t, f.assertMockCallFromTable("notification", "", wrongQuery, nil))
}

func TestMockCallStatusMatchesTheSameRecordedCall(t *testing.T) {
	f := New()
	f.wildcardProcessor.SetVariable("status", 503)
	f.mockRegistry["notification"] = &MockConfig{
		Called: true,
		CallHistory: []MockCall{
			{Status: 200, Payload: map[string]interface{}{"id": "42"}},
			{Status: 503, Payload: map[string]interface{}{"id": "43"}},
		},
	}
	require.NoError(t, f.assertMockCallFromTable("notification", "200", nil, nil))
	require.NoError(t, f.assertMockCallFromTable("notification", "$VAR(status)$", nil, nil))
	require.NoError(t, f.assertMockCallFromTable("notification", "$ANY_INT$", nil, nil))
	require.NoError(t, f.assertMockCallFromTable("notification", "503", nil, &godog.DocString{Content: `{"id":"43"}`}))
	require.Error(t, f.assertMockCallFromTable("notification", "503", nil, &godog.DocString{Content: `{"id":"42"}`}))
	require.ErrorContains(t, f.assertMockCallFromTable("notification", "404", nil, nil), "expected response status 404")
	require.Error(t, f.assertMockCallFromTable("missing", "200", nil, nil))
	f.mockRegistry["unused"] = &MockConfig{}
	require.Error(t, f.assertMockCallFromTable("unused", "200", nil, nil))
	statusTable := httpTestTable([]string{"type", "name", "value"}, []string{"status", "", "200"})
	require.Error(t, f.assertMockCallFromTable("notification", "200", statusTable, nil))
}

func TestMockRecordsReturnedStatusWithoutMetadata(t *testing.T) {
	f := New()
	require.NoError(t, f.jsonMockFromTable("unavailable", "GET", "events", "503", nil, &godog.DocString{Content: `{"error":"unavailable"}`}))
	response := httptest.NewRecorder()
	f.handleMockRequest(response, httptest.NewRequest(http.MethodGet, "/mocks/events", nil))
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Len(t, f.mockRegistry["unavailable"].CallHistory, 1)
	assert.Equal(t, response.Code, f.mockRegistry["unavailable"].CallHistory[0].Status)
	require.NoError(t, f.assertMockCallFromTable("unavailable", "503", nil, nil))
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
