package bdd

import (
	"encoding/json"
	"fmt"
	"github.com/cucumber/godog"
)

type requestData struct {
	Payload map[string]interface{} `json:"payload"`
	Headers map[string]string      `json:"headers"`
}

func (f *FeaturesContext) appIsRunning() error {
	if f.app == nil {
		return fmt.Errorf("[14537537] app not registered")
	}
	f.apiApp = f.app
	return nil
}

func (f *FeaturesContext) processHeaderWildcards(headers map[string]string) (map[string]string, error) {
	if headers == nil {
		return nil, nil
	}

	processedHeaders := make(map[string]string)

	for key, value := range headers {
		processedKey, err := f.wildcardProcessor.SubstituteWildcards(key)
		if err != nil {
			return nil, fmt.Errorf("[147780] wildcard processing error for header key: %v", err)
		}

		processedValue, err := f.wildcardProcessor.SubstituteWildcards(value)
		if err != nil {
			return nil, fmt.Errorf("[147781] wildcard processing error for header value: %v", err)
		}

		stringKey, ok := processedKey.(string)
		if !ok {
			return nil, fmt.Errorf("[147782] expected string for header key, got %T", processedKey)
		}

		stringValue, ok := processedValue.(string)
		if !ok {
			return nil, fmt.Errorf("[147783] expected string for header value, got %T", processedValue)
		}

		processedHeaders[stringKey] = stringValue
	}

	return processedHeaders, nil
}

func (f *FeaturesContext) makeJsonRequest(method, path string, data *godog.DocString) error {
	if f.apiApp == nil {
		return fmt.Errorf("[14538538] app is not running")
	}
	var wrappedPayload requestData

	if data != nil {
		err := json.Unmarshal([]byte(data.Content), &wrappedPayload)
		if err != nil {
			return fmt.Errorf("[147777] unmarshall error: %v", err)
		}

		if wrappedPayload.Payload != nil {
			processedData, err := f.wildcardProcessor.SubstituteWildcards(wrappedPayload.Payload)
			if err != nil {
				return fmt.Errorf("[147778] wildcard processing error: %v", err)
			}

			var ok bool
			wrappedPayload.Payload, ok = processedData.(map[string]interface{})
			if !ok {
				return fmt.Errorf("[147779] expected object, got %T", processedData)
			}
		}

		wrappedPayload.Headers, err = f.processHeaderWildcards(wrappedPayload.Headers)
		if err != nil {
			return err
		}
	}

	pathWithWildcards, err := f.wildcardProcessor.SubstituteWildcards(path)
	if err != nil {
		return fmt.Errorf("[147778] wildcard processing error: %v", err)
	}

	resolvedPath, ok := pathWithWildcards.(string)
	if !ok {
		return fmt.Errorf("[147784] expected string for request path, got %T", pathWithWildcards)
	}
	f.apiResponse, err = f.makeRequest(f.apiApp, method, resolvedPath, wrappedPayload.Payload, wrappedPayload.Headers)

	return err
}

func (f *FeaturesContext) assertResponseStatus(status int) error {
	if f.apiResponse == nil {
		return fmt.Errorf("[1922432244] no API response available")
	}

	if f.apiResponse.Code != status {
		return fmt.Errorf("expected response status: %d, actual status: %d, response: %s",
			status, f.apiResponse.Code, f.apiResponse.Body.String())
	}
	return nil
}

func (f *FeaturesContext) assertJsonResponse(status int, data *godog.DocString) error {
	err := f.assertResponseStatus(status)
	if err != nil {
		return fmt.Errorf("[19028028] assert response status: %w", err)
	}

	if data != nil {
		err = f.jsonComparator.CompareJSON(data.Content, f.apiResponse.Body.String())
		if err != nil {
			return fmt.Errorf("[1525112511] assert json eq: %w", err)
		}
	}

	return nil
}
