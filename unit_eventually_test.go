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
	counts     []int64
	countCalls int
}

func (d *eventualDatabase) Exec(string, ...interface{}) error { return nil }

func (d *eventualDatabase) Query(string, ...interface{}) ([]map[string]interface{}, error) {
	d.queryCalls++
	if d.queryCalls < 3 {
		return nil, nil
	}
	return []map[string]interface{}{{"id": "42", "status": "completed"}}, nil
}

func (d *eventualDatabase) Count(string) (int64, error) {
	d.countCalls++
	if len(d.counts) == 0 {
		return 0, nil
	}
	return d.counts[min(d.countCalls-1, len(d.counts)-1)], nil
}

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

func TestHaveRecordsCountWithinRetriesAndResolvesVariables(t *testing.T) {
	f := New()
	db := &eventualDatabase{counts: []int64{0, 1, 2}}
	f.RegisterDB(db)
	require.NoError(t, f.setVariable("expected_count", "int:2"))

	require.NoError(t, f.haveRecordsCountWithin("100ms", "$VAR(expected_count)$", "orders"))
	assert.Equal(t, 3, db.countCalls)
}

func TestHaveEmptyTableWithinRetriesUntilEmpty(t *testing.T) {
	f := New()
	db := &eventualDatabase{counts: []int64{2, 1, 0}}
	f.RegisterDB(db)

	require.NoError(t, f.haveEmptyTableWithin("100ms", "orders"))
	assert.Equal(t, 3, db.countCalls)
}

func TestDatabaseCountWithinStepsReportTimeoutAndRejectInvalidDurations(t *testing.T) {
	tests := []struct {
		name      string
		assert    func(*FeaturesContext, string) error
		lastError string
	}{
		{
			name: "count",
			assert: func(f *FeaturesContext, timeout string) error {
				return f.haveRecordsCountWithin(timeout, "2", "orders")
			},
			lastError: "table orders has 1 records, expected 2",
		},
		{
			name: "empty",
			assert: func(f *FeaturesContext, timeout string) error {
				return f.haveEmptyTableWithin(timeout, "orders")
			},
			lastError: "table orders has 1 records, expected 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New()
			db := &eventualDatabase{counts: []int64{1}}
			f.RegisterDB(db)

			err := tt.assert(f, "20ms")
			require.ErrorContains(t, err, "did not become true within 20ms")
			assert.ErrorContains(t, err, tt.lastError)
			assert.GreaterOrEqual(t, db.countCalls, 2)

			for _, timeout := range []string{"invalid", "0s", "-1s"} {
				calls := db.countCalls
				require.ErrorContains(t, tt.assert(f, timeout), "expected a positive Go duration")
				assert.Equal(t, calls, db.countCalls)
			}
		})
	}
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
