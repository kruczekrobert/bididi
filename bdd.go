package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

func newFeaturesContext() *FeaturesContext {
	wildcardProcessor := NewWildcardProcessor()
	return &FeaturesContext{
		mockRegistry:      make(map[string]*MockConfig),
		wildcardProcessor: wildcardProcessor,
		jsonComparator:    NewJSONComparator(wildcardProcessor),
	}
}

func New() *FeaturesContext {
	return newFeaturesContext()
}

func (f *FeaturesContext) RegisterApp(handler http.Handler) {
	f.app = handler
}

func (f *FeaturesContext) RegisterDB(db Database) {
	f.db = db
}

func (f *FeaturesContext) RegisterRedis(redis RedisClient) {
	f.redis = redis
}

func (f *FeaturesContext) RegisterWildcard(pattern string, handler WildcardHandler) error {
	return f.wildcardProcessor.RegisterWildcard(pattern, handler)
}

func (f *FeaturesContext) MockURL() string {
	if f.mockHTTPServer == nil {
		f.mockServerStart()
	}
	return f.mockHTTPServer.URL + "/mocks"
}

func (f *FeaturesContext) Close() {
	f.mockServerStop()
}

type FeaturesContext struct {
	app               http.Handler
	apiApp            http.Handler
	mockHTTPServer    *httptest.Server
	apiResponse       *httptest.ResponseRecorder
	db                Database
	redis             RedisClient
	mockRegistry      map[string]*MockConfig
	wildcardProcessor *WildcardProcessor
	jsonComparator    *JSONComparator
}

func (f *FeaturesContext) clear(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
	f.apiApp = nil
	f.apiResponse = nil
	f.mockRegistry = make(map[string]*MockConfig)
	f.wildcardProcessor.ClearVariables()
	return ctx, nil
}

func (f *FeaturesContext) InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Before(f.clear)

	// sql
	ctx.Step(`^add "([^"]*)" records$`, f.addRecords)
	ctx.Step(`^have "([^"]*)" records$`, f.haveRecords)
	ctx.Step(`^have empty "([^"]*)" table$`, f.haveEmptyTable)
	ctx.Step(`^have "([^"]*)" records in "([^"]*)" table$`, f.haveRecordsCount)
	ctx.Step(`^execute SQL "([^"]*)"$`, f.executeRawSQL)

	// vars
	ctx.Step(`^set variable "([^"]*)" to "([^"]*)"$`, f.setVariable)
	ctx.Step(`^extract "([^"]*)" from response as "([^"]*)"$`, f.extractVariableFromResponse)
	ctx.Step(`^extract from database "([^"]*)" as "([^"]*)"$`, f.extractVariableFromDatabase)
	ctx.Step(`^variable "([^"]*)" (equals|not equals|contains|not contains) "([^"]*)"$`, f.checkVariable)

	// HTTP request steps
	ctx.Step(`^app is running$`, f.appIsRunning)
	ctx.Step(`^make a "([^"]*)" request to "([^"]*)"$`, f.makeJsonRequest)
	ctx.Step(`^have response with status (\d+)$`, f.assertResponseStatus)
	ctx.Step(`^have json response with status (\d+)$`, f.assertJsonResponse)

	// Mock steps
	ctx.Step(`^json mock "([^"]*)"$`, f.jsonMock)
	ctx.Step(`^json mock "([^"]*)" was called$`, f.jsonMockWasCalled)
	ctx.Step(`^json mock "([^"]*)" was not called$`, f.jsonMockWasNotCalled)
	ctx.Step(`^json mock "([^"]*)" was called with$`, f.mockWasCalledWith)

	// Redis steps
	ctx.Step(`^redis key "([^"]*)" exists$`, f.assertRedisKeyExists)
	ctx.Step(`^redis key "([^"]*)" does not exist$`, f.assertRedisKeyDoesNotExists)
	ctx.Step(`^have redis key "([^"]*)" with JSON$`, f.assertRedisValueJson)
	ctx.Step(`^have redis key "([^"]*)" with string$`, f.assertRedisValueString)
	ctx.Step(`^set redis key "([^"]*)" to$`, f.setRedisValue)
	ctx.Step(`^set redis key "([^"]*)" to JSON$`, f.setRedisJSONValue)
	ctx.Step(`^set redis key "([^"]*)" with TTL "([^"]*)" to$`, f.setRedisValueWithTTL)
	ctx.Step(`^set redis key "([^"]*)" with TTL "([^"]*)" to JSON$`, f.setRedisJSONValueWithTTL)
}

func (f *FeaturesContext) mockServerStart() {
	f.mockHTTPServer = httptest.NewServer(http.HandlerFunc(f.handleMockRequest))
}

func (f *FeaturesContext) handleMockRequest(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/mocks/") {
		writeMockJSON(w, http.StatusNotFound, map[string]interface{}{"error": "not found"})
		return
	}

	method := r.Method
	rawPath := strings.TrimPrefix(r.URL.Path, "/mocks/")
	fullURL := f.buildNormalizedURL(rawPath, r.URL.Query())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeMockJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request body"})
		return
	}
		var payload map[string]interface{}
		if len(body) > 0 {
			err = json.Unmarshal(body, &payload)
			if err != nil {
				log.Println("Error unmarshalling body:", err)
				writeMockJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid json payload"})
				return
			}
		}

		headers := make(map[string]string)
		for key, values := range r.Header {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}

		for _, mock := range f.mockRegistry {
			if method == mock.Method && fullURL == mock.Path {
				call := MockCall{
					Method:   method,
					Path:     fullURL,
					Headers:  headers,
					Payload:  payload,
					CalledAt: time.Now().Format(time.RFC3339),
				}
				mock.CallHistory = append(mock.CallHistory, call)

				mock.Called = true
				for key, value := range mock.Headers {
					w.Header().Set(key, value)
				}
				writeMockJSON(w, mock.Status, mock.Response)
				return
			}
		}
		writeMockJSON(w, http.StatusNotFound, map[string]interface{}{"error": "not found"})
}

func writeMockJSON(w http.ResponseWriter, status int, value interface{}) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Println("Error encoding mock response:", err)
	}
}

func (f *FeaturesContext) buildNormalizedURL(path string, query url.Values) string {
	queryParts := make([]string, 0, len(query))
	for key, values := range query {
		for _, value := range values {
			queryParts = append(queryParts, fmt.Sprintf("%s=%s", key, value))
		}
	}
	sort.Strings(queryParts)
	if len(queryParts) == 0 {
		return path
	}
	return fmt.Sprintf("%s?%s", path, strings.Join(queryParts, "&"))
}

func (f *FeaturesContext) mockServerStop() {
	if f.mockHTTPServer != nil {
		f.mockHTTPServer.Close()
		f.mockHTTPServer = nil
	}
}
