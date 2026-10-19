package bdd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
)

func (f *FeaturesContext) makeRequest(handler http.Handler, method string, path string, payload map[string]interface{}, headers map[string]string) (*httptest.ResponseRecorder, error) {
	buf := bytes.NewBuffer(nil)

	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("[14623623] marshal error: %w", err)
		}
		buf.Write(b)
	}

	req, err := http.NewRequest(method, path, buf)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if headers != nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)

	return response, nil
}

func (f *FeaturesContext) tableToMaps(table *godog.Table) []map[string]interface{} {
	var results []map[string]interface{}

	if len(table.Rows) == 0 {
		return results
	}

	keys := table.Rows[0].Cells

	for _, values := range table.Rows[1:] {
		result := map[string]interface{}{}
		for idx, value := range values.Cells {
			if idx < len(keys) {
				result[keys[idx].Value] = value.Value
			}
		}
		results = append(results, result)
	}
	return results
}

func convertToStringInterfaceMap(data interface{}) (map[string]interface{}, error) {
	switch v := data.(type) {
	case map[string]interface{}:
		return v, nil
	case map[string]string:
		result := make(map[string]interface{})
		for key, value := range v {
			result[key] = value
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unexpected type: expected map[string]interface{} or map[string]string, got %T", data)
	}
}

func isValidUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
