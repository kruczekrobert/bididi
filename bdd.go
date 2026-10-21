package bdd

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

func newFeaturesContext() *FeaturesContext {
	wildcardProcessor := NewWildcardProcessor()
	return &FeaturesContext{
		mockRegistry:      make(map[string]*MockConfig),
		messageObservers:  make(map[string]MessageObserver),
		observedMessages:  make(map[string][]observedMessage),
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

func (f *FeaturesContext) RegisterCache(cache Cache) {
	f.cache = cache
}

func (f *FeaturesContext) RegisterMessageBus(bus MessageBus) {
	f.messageBus = bus
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
	_ = f.closeMessageObservers()
	f.mockServerStop()
}

type FeaturesContext struct {
	app               http.Handler
	apiApp            http.Handler
	mockHTTPServer    *httptest.Server
	apiResponse       *httptest.ResponseRecorder
	db                Database
	cache             Cache
	messageBus        MessageBus
	messageObservers  map[string]MessageObserver
	observedMessages  map[string][]observedMessage
	mockRegistry      map[string]*MockConfig
	wildcardProcessor *WildcardProcessor
	jsonComparator    *JSONComparator
}

func (f *FeaturesContext) clear(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
	if err := f.closeMessageObservers(); err != nil {
		return ctx, err
	}
	f.apiApp = nil
	f.apiResponse = nil
	f.mockRegistry = make(map[string]*MockConfig)
	f.wildcardProcessor.ClearVariables()
	return ctx, nil
}

func (f *FeaturesContext) InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Before(f.clear)
	ctx.After(f.afterScenario)

	// sql
	ctx.Step(`^add "([^"]*)" records$`, f.addRecords)
	ctx.Step(`^have "([^"]*)" records$`, f.haveRecords)
	ctx.Step(`^have empty "([^"]*)" table$`, f.haveEmptyTable)
	ctx.Step(`^have "([^"]*)" records in "([^"]*)" table$`, f.haveRecordsCount)
	ctx.Step(`^execute SQL "([^"]*)"$`, f.executeRawSQL)

	// vars
	ctx.Step(`^set variable "([^"]*)" to "([^"]*)"$`, f.setVariable)
	ctx.Step(`^set variables$`, f.setVariables)
	ctx.Step(`^extract "([^"]*)" from response as "([^"]*)"$`, f.extractVariableFromResponse)
	ctx.Step(`^extract from database "([^"]*)" as "([^"]*)"$`, f.extractVariableFromDatabase)
	ctx.Step(`^variable "([^"]*)" (equals|not equals|contains|not contains) "([^"]*)"$`, f.checkVariable)
	ctx.Step(`^variable "([^"]*)" equals JSON$`, f.checkVariableJSON)

	// HTTP request steps
	ctx.Step(`^app is running$`, f.appIsRunning)
	ctx.Step(`^make a "([^"]*)" request to "([^"]*)"$`, f.makeHTTPRequestWithoutBody)
	ctx.Step(`^make a "([^"]*)" request to "([^"]*)" with metadata$`, f.makeHTTPRequestWithoutBodyAndMetadata)
	ctx.Step(`^make a "([^"]*)" request to "([^"]*)" with JSON$`, f.makeHTTPRequestWithBody)
	ctx.Step(`^make a "([^"]*)" request to "([^"]*)" with JSON and metadata$`, f.makeHTTPRequestWithBodyAndMetadata)
	ctx.Step(`^have response with status ([^ ]+)$`, f.assertResponseStatus)
	ctx.Step(`^have JSON response with status ([^ ]+)$`, f.assertHTTPResponseWithBody)
	ctx.Step(`^have JSON response with status ([^ ]+) with metadata$`, f.assertHTTPResponseWithHeaders)
	ctx.Step(`^have empty response with status ([^ ]+)$`, f.assertEmptyHTTPResponse)

	// Mock steps
	ctx.Step(`^json mock "([^"]*)" for "([^"]*)" request to "([^"]*)"$`, f.jsonMockFromTable)
	ctx.Step(`^json mock "([^"]*)" was called$`, f.jsonMockWasCalled)
	ctx.Step(`^json mock "([^"]*)" was not called$`, f.jsonMockWasNotCalled)
	ctx.Step(`^json mock "([^"]*)" was called with$`, f.mockWasCalledWithTable)
	ctx.Step(`^json mock "([^"]*)" was called with metadata$`, f.mockWasCalledWithTableWithoutBody)

	// Cache steps
	ctx.Step(`^cache key "([^"]*)" exists$`, f.assertCacheKeyExists)
	ctx.Step(`^cache key "([^"]*)" does not exist$`, f.assertCacheKeyDoesNotExists)
	ctx.Step(`^have cache key "([^"]*)" with JSON$`, f.assertCacheValueJson)
	ctx.Step(`^have cache key "([^"]*)" with string$`, f.assertCacheValueString)
	ctx.Step(`^set cache key "([^"]*)" to$`, f.setCacheValue)
	ctx.Step(`^set cache key "([^"]*)" to JSON$`, f.setCacheJSONValue)
	ctx.Step(`^set cache key "([^"]*)" with TTL "([^"]*)" to$`, f.setCacheValueWithTTL)
	ctx.Step(`^set cache key "([^"]*)" with TTL "([^"]*)" to JSON$`, f.setCacheJSONValueWithTTL)

	// Message bus steps
	ctx.Step(`^observe messages from "([^"]*)"$`, f.observeMessages)
	ctx.Step(`^publish JSON message to "([^"]*)"$`, f.publishJSONMessage)
	ctx.Step(`^within "([^"]*)" messages from "([^"]*)" contain JSON$`, f.assertMessages)
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
					Query:    r.URL.Query(),
					Headers:  headers,
					Payload:  payload,
					CalledAt: time.Now().Format(time.RFC3339),
				}
				mock.Called = true
				for key, value := range mock.Headers {
					w.Header().Set(key, value)
				}
				writeMockJSON(w, mock.Status, mock.Response)
				call.ResponseHeaders = make(map[string]string)
				for key, values := range w.Header() {
				if len(values) > 0 {
					call.ResponseHeaders[key] = values[0]
				}
				}
				mock.CallHistory = append(mock.CallHistory, call)
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
	if len(query) == 0 {
		return path
	}
	return path + "?" + query.Encode()
}

func (f *FeaturesContext) mockServerStop() {
	if f.mockHTTPServer != nil {
		f.mockHTTPServer.Close()
		f.mockHTTPServer = nil
	}
}
