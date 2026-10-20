package testapp_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/kruczekrobert/bididi"
	"github.com/kruczekrobert/bididi/tests/test-app"
	"go.uber.org/fx"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type gormDatabase struct{ db *gorm.DB }

func (g gormDatabase) Exec(query string, args ...interface{}) error {
	return g.db.Exec(query, args...).Error
}

func (g gormDatabase) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	var rows []map[string]interface{}
	err := g.db.Raw(query, args...).Scan(&rows).Error
	return rows, err
}

func (g gormDatabase) Count(table string) (int64, error) {
	var count int64
	err := g.db.Table(table).Count(&count).Error
	return count, err
}

type memoryRedis struct {
	values   map[string]string
	ttlByKey map[string]time.Duration
}

func (m *memoryRedis) Exists(_ context.Context, key string) (bool, error) {
	_, ok := m.values[key]
	return ok, nil
}

func (m *memoryRedis) Get(_ context.Context, key string) (string, bool, error) {
	value, ok := m.values[key]
	return value, ok, nil
}

func (m *memoryRedis) Set(_ context.Context, key, value string, ttl time.Duration) error {
	m.values[key] = value
	m.ttlByKey[key] = ttl
	return nil
}

type testWildcard struct{}

func (testWildcard) CanHandle(wildcard string) bool { return wildcard == "$TEST_VALUE$" }

func (testWildcard) Process(wildcard string, context bdd.Context) (interface{}, error) {
	if context == bdd.ContextGeneration {
		return "Ada", nil
	}
	return wildcard, nil
}

func (testWildcard) ShouldBeQuotedInJSON(string, interface{}) bool { return true }

func (testWildcard) Match(_ string, actual interface{}) error {
	if actual != "Ada" {
		return fmt.Errorf("expected Ada, got %v", actual)
	}
	return nil
}

type fakeService struct {
	items map[string]testapp.Item
}

func (f fakeService) Lookup(id string) (testapp.Item, bool) {
	item, ok := f.items[id]
	return item, ok
}

func TestFeature(t *testing.T) {
	features := bdd.New()
	defer features.Close()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	features.RegisterDB(gormDatabase{db: db})
	cache := &memoryRedis{
		values:   make(map[string]string),
		ttlByKey: make(map[string]time.Duration),
	}
	features.RegisterRedis(cache)
	bus := newMemoryMessageBus()
	bus.handlers["orders.commands"] = func(ctx context.Context, data []byte) error {
		return testapp.ProcessOrderCommand(ctx, data, bus.Publish)
	}
	features.RegisterMessageBus(bus)
	if err := features.RegisterWildcard(`\$TEST_VALUE\$`, testWildcard{}); err != nil {
		t.Fatal(err)
	}

	service := fakeService{items: map[string]testapp.Item{
		"42": {ID: "42", Name: "Ada"},
		"43": {ID: "43", Name: "Bob"},
	}}
	var handler http.Handler
	app := fx.New(
		testapp.Module,
		fx.Provide(func() testapp.Service { return service }),
		fx.Supply(features.MockURL()),
		fx.Populate(&handler),
	)
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	features.RegisterApp(handler)

	suite := godog.TestSuite{
		Name:                "test-app",
		ScenarioInitializer: func(scenario *godog.ScenarioContext) {
			scenario.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
				cache.values = make(map[string]string)
				cache.ttlByKey = make(map[string]time.Duration)
				return ctx, nil
			})
			features.InitializeScenario(scenario)
			scenario.Step(`^cache mock recorded TTL "([^"]*)" for key "([^"]*)"$`, func(ttlValue, key string) error {
				expected, err := time.ParseDuration(ttlValue)
				if err != nil {
					return err
				}
				actual, ok := cache.ttlByKey[key]
				if !ok {
					return fmt.Errorf("cache mock did not record key %q", key)
				}
				if actual != expected {
					return fmt.Errorf("cache mock recorded TTL %s for key %q, expected %s", actual, key, expected)
				}
				return nil
			})
		},
		Options: &godog.Options{
			Format:   "progress",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("BDD scenarios failed")
	}
}
