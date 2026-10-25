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
	"sync"
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
	mockMu            sync.RWMutex
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
	f.mockMu.Lock()
	f.mockRegistry = make(map[string]*MockConfig)
	f.mockMu.Unlock()
	f.wildcardProcessor.ClearVariables()
	return ctx, nil
}

func (f *FeaturesContext) InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Before(f.clear)
	ctx.After(f.afterScenario)
	ctx.StepContext().Before(func(ctx context.Context, step *godog.Step) (context.Context, error) {
		return context.WithValue(ctx, httpStepContextKey{}, step), nil
	})

	// sql
	ctx.Step(`^add "([^"]*)" records$`, f.addRecords)
	ctx.Step(`^have "([^"]*)" records$`, f.haveRecords)
	ctx.Step(`^within "([^"]*)" have "([^"]*)" records$`, f.haveRecordsWithin)
	ctx.Step(`^have empty "([^"]*)" table$`, f.haveEmptyTable)
	ctx.Step(`^within "([^"]*)" have empty "([^"]*)" table$`, f.haveEmptyTableWithin)
	ctx.Step(`^have "([^"]*)" records in "([^"]*)" table$`, f.haveRecordsCount)
	ctx.Step(`^within "([^"]*)" have "([^"]*)" records in "([^"]*)" table$`, f.haveRecordsCountWithin)
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
	ctx.Step(`^make a "([^"]*)" request to "([^"]*)"$`, func(ctx context.Context, method, path string) error {
		table, body := httpStepArguments(ctx)
		return f.makeHTTPRequest(method, path, table, body)
	})
	ctx.Step(`^have response with status ([^ ]+)$`, f.assertResponseStatus)
	ctx.Step(`^have JSON response with status ([^ ]+)$`, func(ctx context.Context, status string) error {
		table, body := httpStepArguments(ctx)
		return f.assertHTTPResponse(status, table, body)
	})
	ctx.Step(`^have empty response with status ([^ ]+)$`, f.assertEmptyHTTPResponse)

	// Mock steps
	ctx.Step(`^json mock "([^"]*)" for "([^"]*)" request to "([^"]*)" with status ([^ ]+)$`, func(ctx context.Context, name, method, path, status string) error {
		table, body := httpStepArguments(ctx)
		return f.jsonMockFromTable(name, method, path, status, table, body)
	})
	ctx.Step(`^json mock "([^"]*)" was called(?: with status ([^ ]+))?$`, func(ctx context.Context, name, status string) error {
		table, body := httpStepArguments(ctx)
		if status == "" && table == nil && body == nil {
			return f.jsonMockWasCalled(name)
		}
		return f.assertMockCallFromTable(name, status, table, body)
	})
	ctx.Step(`^json mock "([^"]*)" was not called$`, f.jsonMockWasNotCalled)
	ctx.Step(`^within "([^"]*)" json mock "([^"]*)" was called$`, f.jsonMockWasCalledWithin)
	ctx.Step(`^within "([^"]*)" json mock "([^"]*)" was called ([0-9]+) times$`, f.jsonMockWasCalledCountWithin)
	ctx.Step(`^for "([^"]*)" json mock "([^"]*)" was not called$`, f.jsonMockWasNotCalledFor)

	// Cache steps
	ctx.Step(`^cache key "([^"]*)" exists$`, f.assertCacheKeyExists)
	ctx.Step(`^cache key "([^"]*)" does not exist$`, f.assertCacheKeyDoesNotExists)
	ctx.Step(`^have cache key "([^"]*)" with JSON$`, f.assertCacheValueJson)
	ctx.Step(`^within "([^"]*)" cache key "([^"]*)" exists$`, f.assertCacheKeyExistsWithin)
	ctx.Step(`^within "([^"]*)" cache key "([^"]*)" does not exist$`, f.assertCacheKeyDoesNotExistWithin)
	ctx.Step(`^within "([^"]*)" have cache key "([^"]*)" with JSON$`, f.assertCacheValueJSONWithin)
	ctx.Step(`^have cache key "([^"]*)" with string$`, f.assertCacheValueString)
	ctx.Step(`^set cache key "([^"]*)" to$`, f.setCacheValue)
	ctx.Step(`^set cache key "([^"]*)" to JSON$`, f.setCacheJSONValue)
	ctx.Step(`^set cache key "([^"]*)" with TTL "([^"]*)" to$`, f.setCacheValueWithTTL)
	ctx.Step(`^set cache key "([^"]*)" with TTL "([^"]*)" to JSON$`, f.setCacheJSONValueWithTTL)

	// Message bus steps
	ctx.Step(`^observe messages from "([^"]*)"$`, f.observeMessages)
	ctx.Step(`^publish JSON message to "([^"]*)"$`, f.publishJSONMessage)
	ctx.Step(`^within "([^"]*)" messages from "([^"]*)" contain JSON$`, f.assertMessages)
	ctx.Step(`^within "([^"]*)" messages from "([^"]*)" contain JSON in order$`, f.assertMessagesInOrder)
	ctx.Step(`^within "([^"]*)" messages from "([^"]*)" do not contain JSON$`, f.assertMessagesAbsent)
	ctx.Step(`^extract "([^"]*)" from message ([1-9][0-9]*) from "([^"]*)" as "([^"]*)"$`, f.extractVariableFromMessage)
	ctx.Step(`^within "([^"]*)" extract "([^"]*)" from first message from "([^"]*)" matching JSON as "([^"]*)"$`, f.extractVariableFromMatchingMessage)
	ctx.Step(`^within "([^"]*)" messages from "([^"]*)" have count ([0-9]+) matching JSON$`, f.assertMessageCount)
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

	f.mockMu.Lock()
	for _, mock := range f.mockRegistry {
		if method == mock.Method && fullURL == mock.Path {
			call := MockCall{
				Method:   method,
				Path:     fullURL,
				Status:   mock.Status,
				Query:    r.URL.Query(),
				Headers:  headers,
				Payload:  payload,
				CalledAt: time.Now().Format(time.RFC3339),
			}
			for key, value := range mock.Headers {
				w.Header().Set(key, value)
			}
			if w.Header().Get("Content-Type") == "" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
			}
			call.ResponseHeaders = make(map[string]string)
			for key, values := range w.Header() {
				if len(values) > 0 {
					call.ResponseHeaders[key] = values[0]
				}
			}
			mock.Called = true
			mock.CallHistory = append(mock.CallHistory, call)
			status := mock.Status
			response := mock.Response
			f.mockMu.Unlock()
			writeMockJSON(w, status, response)
			return
		}
	}
	f.mockMu.Unlock()
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
