package bdd

import (
	"encoding/json"
	"io"
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
	require.NoError(t, f.makeHTTPRequest("POST", "/echo", request, &godog.DocString{Content: `{"name":"Ada"}`}))

	response := httpTestTable(
		[]string{"type", "name", "value"},
		[]string{"header", "X-Trace", "$VAR(trace)$"},
	)
	require.NoError(t, f.assertHTTPResponse("201", response, &godog.DocString{Content: `{"id":"$ANY_STR_FULL$"}`}))
	require.ErrorContains(t, f.assertHTTPResponse("201", response, nil), "requires a body DocString")
}

func TestHTTPRequestWithoutBody(t *testing.T) {
	f := New()
	f.apiApp = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/items/42", r.URL.Path)
		assert.Equal(t, int64(0), r.ContentLength)
		w.WriteHeader(http.StatusNoContent)
	})
	require.NoError(t, f.makeHTTPRequest("GET", "/items/42", nil, nil))
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
			require.Error(t, f.makeHTTPRequest("GET", "/echo", test.table, nil))
		})
	}
	require.Error(t, f.makeHTTPRequest("GET", "/echo?tag=go", nil, nil))
}

func TestHTTPTableIsOptional(t *testing.T) {
	for _, table := range []*godog.Table{nil, {}, httpTestTable([]string{"type", "name", "value"})} {
		rows, err := parseHTTPTable(table, map[string]bool{"header": true})
		require.NoError(t, err)
		assert.Empty(t, rows)
	}
}

func TestHTTPStepsWithOptionalArguments(t *testing.T) {
	f := New()
	f.RegisterApp(http.HandlerFunc(f.handleMockRequest))
	feature := `Feature: Optional HTTP step arguments

  Background:
    Given app is running

  Scenario: No request arguments and no mock assertion arguments
    Given json mock "reply" for "GET" request to "events" with status 200
      """
      {"accepted":true}
      """
    When make a "GET" request to "/mocks/events"
    Then have JSON response with status 200
      """
      {"accepted":true}
      """
    And json mock "reply" was called
    And json mock "reply" was called with status 200

  Scenario: JSON without a metadata table
    Given json mock "reply" for "POST" request to "events" with status 201
      """
      {"accepted":true}
      """
    When make a "POST" request to "/mocks/events"
      """
      {"id":"42"}
      """
    Then have JSON response with status 201
      """
      {"accepted":true}
      """
    And json mock "reply" was called with status 201
      """
      {"id":"42"}
      """

  Scenario: A metadata table without request or expected JSON
    Given json mock "reply" for "GET" request to "events" with status 200
      | type            | name   | value     |
      | query           | source | checkout  |
      | response-header | X-Mock | from-mock |
      """
      {"accepted":true}
      """
    When make a "GET" request to "/mocks/events"
      | type   | name    | value    |
      | query  | source  | checkout |
      | header | X-Trace | trace-42 |
    Then have response with status 200
    And json mock "reply" was called
      | type            | name    | value     |
      | query           | source  | checkout  |
      | header          | X-Trace | trace-42  |
      | response-header | X-Mock  | from-mock |

  Scenario: JSON and metadata share one step
    Given set variable "status" to "int:200"
    And json mock "reply" for "POST" request to "events" with status $VAR(status)$
      | type            | name   | value     |
      | response-header | X-Mock | from-mock |
      """
      {"accepted":true}
      """
    When make a "POST" request to "/mocks/events"
      | type   | name    | value    |
      | header | X-Trace | trace-42 |
      """
      {"id":"42"}
      """
    Then have JSON response with status 200
      | type   | name   | value     |
      | header | X-Mock | from-mock |
      """
      {"accepted":true}
      """
    And json mock "reply" was called with status $VAR(status)$
      | type   | name    | value    |
      | header | X-Trace | trace-42 |
      """
      {"id":"42"}
      """
    And json mock "reply" was called with status $ANY_INT$

  Scenario: JSON can precede the metadata table
    Given json mock "reply" for "POST" request to "events" with status 200
      """
      {"accepted":true}
      """
      | type            | name   | value     |
      | response-header | X-Mock | from-mock |
    When make a "POST" request to "/mocks/events"
      """
      {"id":"42"}
      """
      | type   | name    | value    |
      | header | X-Trace | trace-42 |
    Then have JSON response with status 200
      """
      {"accepted":true}
      """
      | type   | name   | value     |
      | header | X-Mock | from-mock |
    And json mock "reply" was called
      """
      {"id":"42"}
      """
      | type   | name    | value    |
      | header | X-Trace | trace-42 |
`
	suite := godog.TestSuite{
		Name:                "optional-http-arguments",
		ScenarioInitializer: f.InitializeScenario,
		Options: &godog.Options{
			Format:          "progress",
			Output:          io.Discard,
			Strict:          true,
			TestingT:        t,
			FeatureContents: []godog.Feature{{
				Name:     "optional-http-arguments.feature",
				Contents: []byte(feature),
			}},
		},
	}
	require.Zero(t, suite.Run())
}
