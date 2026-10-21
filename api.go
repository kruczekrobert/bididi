package bdd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/cucumber/godog"
)

func (f *FeaturesContext) appIsRunning() error {
	if f.app == nil {
		return fmt.Errorf("[14537537] app not registered")
	}
	f.apiApp = f.app
	return nil
}

func (f *FeaturesContext) makeHTTPRequest(method, path string, table *godog.Table, body *godog.DocString) error {
	if f.apiApp == nil {
		return fmt.Errorf("[14538538] app is not running")
	}
	var rows []httpTableRow
	var err error
	if table != nil {
		rows, err = parseHTTPTable(table, map[string]bool{"query": true, "header": true})
		if err != nil {
			return err
		}
	}
	headers := make(map[string]string)
	query := make(url.Values)
	for _, row := range rows {
		switch row.typeName {
		case "query":
			var name, value string
			name, err = f.resolveHTTPText(row.name)
			if err == nil {
				value, err = f.resolveHTTPText(row.value)
			}
			if err == nil {
				query.Add(name, value)
			}
		case "header":
			var name, value string
			name, err = f.resolveHTTPText(row.name)
			if err == nil {
				value, err = f.resolveHTTPText(row.value)
			}
			if err == nil {
				headers[name] = value
			}
		}
		if err != nil {
			return fmt.Errorf("HTTP request %s: %w", row.typeName, err)
		}
	}
	method, err = f.resolveHTTPText(method)
	if err != nil {
		return fmt.Errorf("HTTP request method: %w", err)
	}
	path, err = f.resolveHTTPText(path)
	if err != nil {
		return fmt.Errorf("HTTP request path: %w", err)
	}
	if method == "" || path == "" {
		return fmt.Errorf("HTTP request requires method and path")
	}
	requestURL, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("invalid HTTP request path: %w", err)
	}
	if requestURL.RawQuery != "" {
		return fmt.Errorf("HTTP request path must not contain query parameters; use query rows")
	}
	requestURL.RawQuery = query.Encode()

	var payload map[string]interface{}
	if body != nil {
		if err := json.Unmarshal([]byte(body.Content), &payload); err != nil {
			return fmt.Errorf("invalid HTTP request JSON body: %w", err)
		}
		if payload == nil {
			return fmt.Errorf("HTTP request requires a JSON object body")
		}
		processed, err := f.wildcardProcessor.SubstituteWildcards(payload)
		if err != nil {
			return fmt.Errorf("HTTP request body: %w", err)
		}
		payload = processed.(map[string]interface{})
	}
	f.apiResponse, err = f.makeRequest(f.apiApp, method, requestURL.String(), payload, headers)
	return err
}

func (f *FeaturesContext) assertResponseStatus(status string) error {
	if f.apiResponse == nil {
		return fmt.Errorf("[1922432244] no API response available")
	}
	return f.compareHTTPStatus(status, f.apiResponse.Code)
}

func (f *FeaturesContext) assertEmptyHTTPResponse(status string) error {
	if err := f.assertResponseStatus(status); err != nil {
		return err
	}
	if actual := f.apiResponse.Body.String(); actual != "" {
		return fmt.Errorf("HTTP response body: expected empty body, got %q", actual)
	}
	return nil
}

func (f *FeaturesContext) assertHTTPResponse(status string, table *godog.Table, body *godog.DocString) error {
	if err := f.assertResponseStatus(status); err != nil {
		return err
	}
	if table != nil {
		rows, err := parseHTTPTable(table, map[string]bool{"header": true})
		if err != nil {
			return err
		}
		for _, row := range rows {
			name, err := f.resolveHTTPText(row.name)
			if err != nil {
				return fmt.Errorf("HTTP response header name: %w", err)
			}
			values := f.apiResponse.Header().Values(name)
			if len(values) == 0 {
				return fmt.Errorf("HTTP response is missing header %q", name)
			}
			if err := f.compareHTTPText(row.value, values[0]); err != nil {
				return fmt.Errorf("HTTP response header %q: %w", name, err)
			}
		}
	}
	if body == nil {
		return fmt.Errorf("HTTP response requires a body DocString")
	}
	if err := f.jsonComparator.CompareJSON(body.Content, f.apiResponse.Body.String()); err != nil {
		return fmt.Errorf("HTTP response body: %w", err)
	}
	return nil
}
