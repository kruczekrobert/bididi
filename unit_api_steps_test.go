package bdd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v34"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func httpTestTable(rows ...[]string) *godog.Table {
	table := &godog.Table{}
	for _, row := range rows {
		cells := make([]*messages.PickleTableCell, 0, len(row))
		for _, value := range row {
			cells = append(cells, &messages.PickleTableCell{Value: value})
		}
		table.Rows = append(table.Rows, &messages.PickleTableRow{Cells: cells})
	}
	return table
}

func TestHTTPRequestAndResponseTables(t *testing.T) {
	f := New()
	f.wildcardProcessor.SetVariable("trace", "trace-42")
	f.apiApp = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/echo", r.URL.Path)
		assert.Equal(t, []string{"go", "bdd"}, r.URL.Query()["tag"])
		assert.Equal(t, "trace-42", r.Header.Get("X-Trace"))
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Ada", body["name"])
		w.Header().Set("X-Trace", "trace-42")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"42"}`))
	})

	request := httpTestTable(
		[]string{"type", "name", "value"},
		[]string{"query", "tag", "go"},
		[]string{"query", "tag", "bdd"},
		[]string{"header", "X-Trace", "$VAR(trace)$"},
	)
	require.NoError(t, f.makeHTTPRequestWithBodyAndMetadata("POST", "/echo", request, &godog.DocString{Content: `{"name":"Ada"}`}))

	response := httpTestTable(
		[]string{"type", "name", "value"},
		[]string{"header", "X-Trace", "$VAR(trace)$"},
	)
	require.NoError(t, f.assertHTTPResponseWithHeaders("201", response, &godog.DocString{Content: `{"id":"$ANY_STR_FULL$"}`}))
}

func TestHTTPRequestWithoutBody(t *testing.T) {
	f := New()
	f.apiApp = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/items/42", r.URL.Path)
		assert.Equal(t, int64(0), r.ContentLength)
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, f.makeHTTPRequestWithoutBody("GET", "/items/42"))
	require.NoError(t, f.assertResponseStatus("204"))
	require.NoError(t, f.assertEmptyHTTPResponse("204"))
	_, _ = f.apiResponse.Body.WriteString("unexpected")
	require.NoError(t, f.assertResponseStatus("204"))
	require.ErrorContains(t, f.assertEmptyHTTPResponse("204"), "expected empty body")
}

func TestHTTPTableValidation(t *testing.T) {
	f := New()
	f.apiApp = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, test := range []struct {
		name  string
		table *godog.Table
	}{
		{"wrong columns", httpTestTable([]string{"field", "name", "value"})},
		{"unknown type", httpTestTable([]string{"type", "name", "value"}, []string{"cookie", "id", "42"})},
		{"method in table", httpTestTable([]string{"type", "name", "value"}, []string{"method", "", "GET"})},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, f.makeHTTPRequestWithoutBodyAndMetadata("GET", "/echo", test.table))
		})
	}
	require.Error(t, f.makeHTTPRequestWithoutBody("GET", "/echo?tag=go"))
}
