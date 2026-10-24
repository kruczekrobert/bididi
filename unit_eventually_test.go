package bdd

import (
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type eventualDatabase struct {
	queryCalls int
}

func (d *eventualDatabase) Exec(string, ...interface{}) error { return nil }

func (d *eventualDatabase) Query(string, ...interface{}) ([]map[string]interface{}, error) {
	d.queryCalls++
	if d.queryCalls < 3 {
		return nil, nil
	}
	return []map[string]interface{}{{"id": "42", "status": "completed"}}, nil
}

func (d *eventualDatabase) Count(string) (int64, error) { return 0, nil }

type cacheRead struct {
	value string
	found bool
}

type scriptedCache struct {
	mu          sync.Mutex
	exists      []bool
	reads       []cacheRead
	existsCalls int
	getCalls    int
}

func (c *scriptedCache) Exists(context.Context, string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value := c.exists[min(c.existsCalls, len(c.exists)-1)]
	c.existsCalls++
	return value, nil
}

func (c *scriptedCache) Get(context.Context, string) (string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	read := c.reads[min(c.getCalls, len(c.reads)-1)]
	c.getCalls++
	return read.value, read.found, nil
}

func (*scriptedCache) Set(context.Context, string, string, time.Duration) error { return nil }

func TestHaveRecordsWithinRetriesUntilRowsMatch(t *testing.T) {
	f := New()
	db := &eventualDatabase{}
	f.RegisterDB(db)
	records := httpTestTable(
		[]string{"id", "status"},
		[]string{"42", "completed"},
	)

	require.NoError(t, f.haveRecordsWithin("100ms", "orders", records))
	assert.GreaterOrEqual(t, db.queryCalls, 3)
}

func TestCacheWithinStepsRetryExistsJSONAndDisappearance(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		f := New()
		cache := &scriptedCache{exists: []bool{false, false, true}}
		f.RegisterCache(cache)
		require.NoError(t, f.assertCacheKeyExistsWithin("100ms", "order:42"))
		assert.GreaterOrEqual(t, cache.existsCalls, 3)
	})

	t.Run("JSON", func(t *testing.T) {
		f := New()
		cache := &scriptedCache{reads: []cacheRead{{}, {value: `{"status":"completed"}`, found: true}}}
		f.RegisterCache(cache)
		require.NoError(t, f.assertCacheValueJSONWithin("100ms", "order:42", &godog.DocString{Content: `{"status":"completed"}`}))
		assert.GreaterOrEqual(t, cache.getCalls, 2)
	})

	t.Run("does not exist", func(t *testing.T) {
		f := New()
		cache := &scriptedCache{exists: []bool{true, true, false}}
		f.RegisterCache(cache)
		require.NoError(t, f.assertCacheKeyDoesNotExistWithin("100ms", "lock:42"))
		assert.GreaterOrEqual(t, cache.existsCalls, 3)
	})
}

func TestEventuallyHelpersReportTimeoutAndCheckTheWholeDuration(t *testing.T) {
	err := waitForAssertion("20ms", "condition", func() error { return assert.AnError })
	require.ErrorContains(t, err, "did not become true within 20ms")

	started := time.Now()
	require.NoError(t, assertForDuration("30ms", "condition", func() error { return nil }))
	assert.GreaterOrEqual(t, time.Since(started), 20*time.Millisecond)
}

type bindingMessageBus struct {
	messages []Message
}

func (*bindingMessageBus) Publish(context.Context, string, []byte) error { return nil }

func (b *bindingMessageBus) Observe(context.Context, string) (MessageObserver, error) {
	observer := &testMessageObserver{messages: make(chan Message, len(b.messages))}
	for _, message := range b.messages {
		observer.messages <- message
	}
	return observer, nil
}

func TestNewEventuallyAndOrderedMessageStepsAreBound(t *testing.T) {
	f := New()
	f.RegisterDB(&eventualDatabase{})
	f.RegisterCache(&scriptedCache{
		exists: []bool{true, true, false},
		reads:  []cacheRead{{value: `{"status":"completed"}`, found: true}},
	})
	f.RegisterMessageBus(&bindingMessageBus{messages: []Message{
		{ID: "1", Data: []byte(`{"type":"Unrelated"}`)},
		{ID: "2", Data: []byte(`{"type":"OrderCreated","payload":{"order_id":"42"}}`)},
		{ID: "3", Data: []byte(`{"type":"InvoiceRequested","payload":{"order_id":"42"}}`)},
	}})
	f.RegisterApp(http.HandlerFunc(f.handleMockRequest))

	feature := `Feature: Eventually and ordered message bindings
  Scenario: Use all new step forms
    Given app is running
    And json mock "billing" for "GET" request to "billing" with status 200
      """
      {}
      """
    And json mock "refund" for "GET" request to "refund" with status 200
      """
      {}
      """
    When make a "GET" request to "/mocks/billing"
    Then within "100ms" json mock "billing" was called
    And within "100ms" json mock "billing" was called 1 times
    And for "20ms" json mock "refund" was not called
    And within "100ms" have "orders" records
      | id | status    |
      | 42 | completed |
    And within "100ms" cache key "order:42" exists
    And within "100ms" have cache key "order:42" with JSON
      """
      {"status":"completed"}
      """
    And within "100ms" cache key "lock:42" does not exist
    And observe messages from "orders.events"
    And within "100ms" messages from "orders.events" contain JSON in order
      """
      [
        {"type":"OrderCreated"},
        {"type":"InvoiceRequested"}
      ]
      """
    And within "100ms" extract "payload.order_id" from first message from "orders.events" matching JSON as "str:order_id"
      """
      {"type":"OrderCreated"}
      """
    And variable "order_id" equals "42"
`

	suite := godog.TestSuite{
		Name:                "new-step-bindings",
		ScenarioInitializer: f.InitializeScenario,
		Options: &godog.Options{
			Format:   "progress",
			Output:   io.Discard,
			Strict:   true,
			TestingT: t,
			FeatureContents: []godog.Feature{{
				Name:     "new-step-bindings.feature",
				Contents: []byte(feature),
			}},
		},
	}
	require.Zero(t, suite.Run())
}
