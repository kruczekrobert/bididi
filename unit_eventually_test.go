package bdd

import (
	"context"
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
