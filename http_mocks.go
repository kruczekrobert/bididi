package bdd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/cucumber/godog"
)

func (f *FeaturesContext) jsonMockFromTable(name, method, path string, table *godog.Table, body *godog.DocString) error {
	if body == nil {
		return fmt.Errorf("mock %q requires a JSON response body", name)
	}
	rows, err := parseHTTPTable(table, map[string]bool{"query": true, "status": true, "response-header": true})
	if err != nil {
		return err
	}
	method, err = f.resolveHTTPText(method)
	if err != nil {
		return fmt.Errorf("mock %q method: %w", name, err)
	}
	path, err = f.resolveHTTPText(path)
	if err != nil {
		return fmt.Errorf("mock %q path: %w", name, err)
	}
	config := MockConfig{Method: method, Path: path, Headers: make(map[string]string)}
	query := make(url.Values)
	statusSet := false
	for _, row := range rows {
		switch row.typeName {
		case "query":
			var key, value string
			key, err = f.resolveHTTPText(row.name)
			if err == nil {
				value, err = f.resolveHTTPText(row.value)
			}
			if err == nil {
				query.Add(key, value)
			}
		case "status":
			if statusSet {
				return fmt.Errorf("mock %q has more than one status", name)
			}
			var status string
			status, err = f.resolveHTTPText(row.value)
			if err == nil {
				config.Status, err = strconv.Atoi(status)
			}
			statusSet = true
		case "response-header":
			var key, value string
			key, err = f.resolveHTTPText(row.name)
			if err == nil {
				value, err = f.resolveHTTPText(row.value)
			}
			if err == nil {
				config.Headers[key] = value
			}
		}
		if err != nil {
			return fmt.Errorf("mock %q %s: %w", name, row.typeName, err)
		}
	}
	if !statusSet || config.Method == "" || config.Path == "" {
		return fmt.Errorf("mock %q requires method, path and status", name)
	}
	parsedURL, err := url.Parse(config.Path)
	if err != nil {
		return fmt.Errorf("mock %q has invalid path: %w", name, err)
	}
	if parsedURL.RawQuery != "" {
		return fmt.Errorf("mock %q path must not contain query parameters; use query rows", name)
	}
	config.Path = f.buildNormalizedURL(config.Path, query)
	if err := json.Unmarshal([]byte(body.Content), &config.Response); err != nil {
		return fmt.Errorf("mock %q has invalid JSON response body: %w", name, err)
	}
	if config.Response == nil {
		return fmt.Errorf("mock %q requires a JSON object response body", name)
	}
	return f.registerMockConfig(name, config)
}

func (f *FeaturesContext) mockWasCalledWithTable(name string, table *godog.Table, body *godog.DocString) error {
	return f.assertMockCallFromTable(name, table, body)
}

func (f *FeaturesContext) mockWasCalledWithTableWithoutBody(name string, table *godog.Table) error {
	return f.assertMockCallFromTable(name, table, nil)
}

func (f *FeaturesContext) assertMockCallFromTable(name string, table *godog.Table, body *godog.DocString) error {
	rows, err := parseHTTPTable(table, map[string]bool{"query": true, "header": true, "response-header": true})
	if err != nil {
		return err
	}
	expected := MockExpectation{
		Headers:         make(map[string]string),
		Query:           make(url.Values),
		ResponseHeaders: make(map[string]string),
	}
	for _, row := range rows {
		key, err := f.resolveHTTPText(row.name)
		if err != nil {
			return fmt.Errorf("mock %q %s name: %w", name, row.typeName, err)
		}
		switch row.typeName {
		case "query":
			expected.Query.Add(key, row.value)
		case "header":
			expected.Headers[key] = row.value
		case "response-header":
			expected.ResponseHeaders[key] = row.value
		}
	}
	if body != nil {
		if err := json.Unmarshal([]byte(body.Content), &expected.Payload); err != nil {
			return fmt.Errorf("mock %q has invalid expected JSON body: %w", name, err)
		}
		if expected.Payload == nil {
			return fmt.Errorf("mock %q requires a JSON object expected body", name)
		}
	}
	encoded, err := json.Marshal(expected)
	if err != nil {
		return fmt.Errorf("mock %q: %w", name, err)
	}
	return f.assertMockCall(name, expected, string(encoded))
}
