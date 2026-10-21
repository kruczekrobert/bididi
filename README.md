# 📘 bididi — BDD Steps Documentation for Developers

Welcome to the guide for bididi, a reusable set of Godog steps for Go applications. It covers HTTP requests, database operations, mock services, cache values, message buses, variables, and wildcards. The library accepts application interfaces; Gin, GORM, and Fx appear only in the example modules.

## 📚 Table of Contents

1. [📦 Install](#-install)
2. [🚀 Quick Start](#-quick-start)
3. [📖 Cucumber Basics for Beginners](#-cucumber-basics-for-beginners)
4. [🌐 API Request Steps](#-api-request-steps)
5. [🗄️ Database Steps](#️-database-steps)
6. [🎭 Mock Service Steps](#-mock-service-steps)
7. [🧮 Variable Handling Steps](#-variable-handling-steps)
8. [💾 Cache Steps](#-cache-steps)
9. [📨 Message Bus Steps](#-message-bus-steps)
10. [🃏 Wildcard System](#-wildcard-system)
11. [🎯 Complete Example](#-complete-example)
12. [🧪 Examples and Tests](#-examples-and-tests)

## 📦 Install

```sh
go get github.com/kruczekrobert/bididi
```

The root module does not require Gin, GORM, Fx, or a cache client package. To use bididi, register an `http.Handler` and, when needed, implementations of these small interfaces:

```go
type Database interface {
    Exec(query string, args ...interface{}) error
    Query(query string, args ...interface{}) ([]map[string]interface{}, error)
    Count(table string) (int64, error)
}

type Cache interface {
    Exists(ctx context.Context, key string) (bool, error)
    Get(ctx context.Context, key string) (value string, found bool, err error)
    Set(ctx context.Context, key, value string, ttl time.Duration) error
}
```

The complete setup is in [example-app/app_test.go](examples/example-app/app_test.go) and [test-app/app_test.go](tests/test-app/app_test.go). Both applications assemble their handlers with Fx; bididi does not depend on Fx.

## 🚀 Quick Start

Register the application in the Go test:

```go
features := bdd.New()
defer features.Close()
features.RegisterApp(handler) // Any http.Handler

suite := godog.TestSuite{
    Name:                "my-app",
    ScenarioInitializer: features.InitializeScenario,
    Options: &godog.Options{
        Paths:    []string{"features"},
        TestingT: t,
    },
}
if suite.Run() != 0 {
    t.Fatal("BDD scenarios failed")
}
```

Then write a feature. This one matches the route in [example-app](examples/example-app):

```gherkin
Feature: Greeting API
  Scenario: Greet a person
    Given app is running
    When make a "GET" request to "/hello/Ada"
    Then have JSON response with status 200
      """
      {"message":"hello Ada"}
      """
```

`Given app is running` checks the registered handler. Requests are served in process; the step does not start a network server.

## 📖 Cucumber Basics for Beginners

### Feature Files Structure

A `.feature` file contains a `Feature`, one or more `Scenario` blocks, and steps. A scenario describes one behavior. Tables and doc strings supply structured data to steps.

### Basic Keywords

| Keyword | Purpose |
| --- | --- |
| `Feature` | Groups related behaviors. |
| `Scenario` | Defines one example. |
| `Given` | Sets up the state. |
| `When` | Performs an action. |
| `Then` | Asserts a result. |
| `And` / `But` | Continues the preceding step type. |

### Example Structure

```gherkin
Feature: Item lookup
  Scenario: An existing item is returned
    Given app is running
    When make a "GET" request to "/items/42"
    Then have JSON response with status 200
      """
      {"id":"42","name":"Ada"}
      """
```

### Background

Use `Background` for steps shared by every scenario in one feature:

```gherkin
Feature: Item lookup
  Background:
    Given app is running

  Scenario: Missing item
    When make a "GET" request to "/items/missing"
    Then have response with status 404
```

### Scenario Outlines with Examples

An outline runs once per `Examples` row. Gherkin expands `<name>` first; bididi then resolves variables and wildcards in the resulting step, table, or doc string.

```gherkin
Scenario Outline: Existing items
  Given app is running
  When make a "GET" request to "/items/<id>"
  Then have JSON response with status 200
    """
    {"id":"<id>","name":"<name>"}
    """

  Examples:
    | id | name |
    | 42 | Ada  |
    | 43 | Bob  |
```

An examples cell may itself contain `$VAR(name)$`; see [http_steps.feature](tests/test-app/features/http_steps.feature) and [db_steps.feature](tests/test-app/features/db_steps.feature).

### Tags

Gherkin tags such as `@http` or `@db` can be used by the Godog runner to select scenarios.

## 🌐 API Request Steps

### Initialize Application

```gherkin
Given app is running
```

Register an `http.Handler` with `features.RegisterApp(handler)` in Go before running the suite. The handler can come from Gin, `http.ServeMux`, or another router.

### Make HTTP Request

```gherkin
When make a "GET" request to "/items/42"
```

This sends a request without a body or metadata table. It also works for other methods, such as a POST without a body.

Use `with metadata` for query parameters or request headers:

```gherkin
When make a "GET" request to "/query" with metadata
  | type  | name | value |
  | query | tag  | go    |
```

Use `with JSON` when sending a JSON body:

```gherkin
When make a "POST" request to "/echo" with JSON
  """
  {"name":"Ada"}
  """
```

To send both a JSON body and metadata:

```gherkin
When make a "POST" request to "/echo" with JSON and metadata
  | type   | name    | value           |
  | query  | source  | checkout        |
  | header | X-Trace | $VAR(trace_id)$ |
  """
  {"name":"Ada"}
  """
```

The metadata table uses columns `type`, `name`, and `value`. Repeat `query` rows for repeated parameters. The DocString is the request JSON object itself. Method, path, table names and values support variables and generating wildcards.

### Assert HTTP Response

```gherkin
Then have response with status 404
```

This checks only the status. To compare a JSON body as well:

```gherkin
Then have JSON response with status 200
  """
  {"id":"42","name":"Ada"}
  """
```

The status supports variables and comparison wildcards. The DocString compares the entire JSON body with the existing variable, wildcard, and unordered-array rules.

To check response headers as well, use the table variant:

```gherkin
Then have JSON response with status 200 with metadata
  | type   | name         | value                           |
  | header | Content-Type | application/json; charset=utf-8 |
  """
  {"id":"42","name":"$ANY_STR_FULL$"}
  """
```

The table checks only the listed response headers.

To require an empty response body:

```gherkin
Then have empty response with status 204
```

See [HTTP scenarios](tests/test-app/features/http_steps.feature) for query parameters, headers, variables, wildcards, and outlines.

## 🗄️ Database Steps

Register a `Database` implementation with `features.RegisterDB(db)` before using these steps. The [test-app adapter](tests/test-app/app_test.go) demonstrates GORM with SQLite.

### Add Records

```gherkin
Given add "step_items" records
  | id | name            |
  | 1  | $VAR(row_name)$ |
```

Inserts rows from a table. Empty cells omit the column so the database default can apply. `$NULL$` inserts SQL `NULL`, and `$EMPTY$` inserts an empty string.

### Verify Records Exist

```gherkin
Then have "step_items" records
  | id | name               |
  | 1  | $ANY_STR_FULL$      |
```

Checks the listed fields. Each expected row must match a distinct database row; additional database fields are allowed.

### Verify Table is Empty

```gherkin
Then have empty "step_items" table
```

### Verify Record Count

```gherkin
Then have "2" records in "step_items" table
```

The quoted count can also be `$VAR(expected_count)$`.

### Execute Raw SQL

```gherkin
When execute SQL "DELETE FROM step_items WHERE id = 1"
```

Variables and generating wildcards in the statement are resolved before execution.

### Extract Variable from Database

```gherkin
And extract from database "SELECT name FROM step_items WHERE id = 1" as "name"
```

The query must return one row. A single column stores its value; multiple columns store a map. Prefix the variable name with `str:`, `int:`, `float:`, `bool:`, or `auto:` to convert it.

See [database scenarios](tests/test-app/features/db_steps.feature) for tables, SQL, extraction, value types, and outlines.

## 🎭 Mock Service Steps

Call `features.MockURL()` in the Go test and pass the URL to the application under test. The mock server lives in the feature context and is closed by `features.Close()`.

### Define JSON Mock

```gherkin
Given json mock "notification" for "POST" request to "events"
  | type            | name   | value     |
  | query           | source | checkout  |
  | status          |        | 200       |
  | response-header | X-Mock | from-mock |
  """
  {"accepted":true}
  """
```

Method and path identify the mock in the step text. The required `status` row configures the HTTP status **returned by the mock**; it is not an assertion. `query` selects which incoming request the mock handles. `response-header` sets a header returned by the mock. The DocString is the JSON response body. Variables and generating wildcards work in the step text, table, and body.

### Verify Mock Was Called

```gherkin
Then json mock "notification" was called
```

### Verify Mock Was Not Called

```gherkin
Then json mock "unused" was not called
```

Declare `unused` first; assertions refer to named mocks already in the registry.

### Verify Mock Was Called With Specific Data

```gherkin
Then json mock "notification" was called with
  | type            | name         | value            |
  | query           | source       | checkout         |
  | header          | Content-Type | application/json |
  | response-header | X-Mock       | from-mock        |
  """
  {"id":"42","name":"Ada"}
  """
```

`header` checks a header received by the mock. `response-header` checks a header sent by the mock. This assertion table accepts `query`, `header`, and `response-header`; it does not accept `status`. The assertion succeeds if any recorded call matches the listed query parameters, headers, and optional JSON request body. Variables and comparison wildcards work in assertions. For a table-only assertion, use `Then json mock "notification" was called with metadata`. The application's HTTP response status can be checked with `Then have response with status 200`. See [mock scenarios](tests/test-app/features/mock_steps.feature).

## 🧮 Variable Handling Steps

### Set Variable

```gherkin
Given set variable "name" to "str:Ada"
```

The value can use a prefix: `str:` keeps text, `int:` makes an integer, `float:` makes a number, `bool:` makes a boolean, and `auto:` infers a type. An unprefixed value is also inferred. Values may contain `$VAR(name)$`, `$UUID$`, `$NOW$`, or a custom generating wildcard.

### Set Multiple Variables

```gherkin
Given set variables
  | name     | value          |
  | order_id | str:42         |
  | count    | int:2          |
  | copy     | $VAR(order_id)$ |
```

The table requires `name` and `value` columns and at least one data row. Rows use the same type prefixes and wildcards as `set variable`. They run in order, so later rows can refer to variables set earlier in the table.

### Extract Variable from Response

```gherkin
And extract "payload.items.0.name" from response as "name"
```

The path uses dots for object fields and numeric array indexes. Variable-name prefixes work here too.

### Check Variable Value

```gherkin
Then variable "name" equals "Ada"
Then variable "name" not equals "Bob"
Then variable "name" contains "Ad"
Then variable "name" not contains "Bo"
```

For a type-sensitive comparison, including a value extracted from the database, use JSON:

```gherkin
Then variable "result" equals JSON
  """
  42
  """
```

This distinguishes the number `42` from the string `"42"` and also compares objects and arrays.

`$VAR(name)$` can appear in later step text, tables, doc strings, mock definitions, and outline examples. `$NULL$` can be compared with `equals` and `not equals`. See [variable scenarios](tests/test-app/features/variable_steps.feature).

## 💾 Cache Steps

Register a `Cache` with `features.RegisterCache(cache)`. The [test-app cache mock](tests/test-app/app_test.go) uses an in-memory map and clears it before each scenario; bididi does not require a particular cache backend.

### Set Cache Key to String Value

```gherkin
Given set cache key "greeting" to
  """
  hello
  """
```

Keys and values can contain variables and generating wildcards.

Without a TTL, bididi passes `0` to `Cache.Set`, meaning no expiration.

### Set Cache Key to String Value with TTL

```gherkin
Given set cache key "temporary" with TTL "30s" to
  """
  hello
  """
```

TTL uses Go duration syntax and must be positive. bididi passes it to the registered client; the client owns expiration behavior.

### Set Cache Key to JSON Value

```gherkin
Given set cache key "profile" to JSON
  """
  {"name":"Ada","count":7}
  """
```

Wildcards are resolved in the JSON structure before it is stored, preserving number and boolean types.

### Set Cache Key to JSON Value with TTL

```gherkin
Given set cache key "temporary-json" with TTL "2m" to JSON
  """
  {"name":"Ada"}
  """
```

### Check Cache Key Exists

```gherkin
Then cache key "profile" exists
```

### Check Cache Key Does Not Exist

```gherkin
Then cache key "missing" does not exist
```

### Check Cache Key Contains String

```gherkin
Then have cache key "greeting" with string
  """
  $ANY_STR_FULL$
  """
```

Literal text, variables, built-in comparison wildcards, and custom matchers are supported.

### Check Cache Key Contains JSON

```gherkin
Then have cache key "profile" with JSON
  """
  {"name":"$ANY_STR_FULL$","count":"$ANY_INT$"}
  """
```

See [cache scenarios](tests/test-app/features/cache_steps.feature) for all eight steps, TTL propagation, scenario isolation, outlines, and custom wildcards.

## 📨 Message Bus Steps

Register an implementation of `MessageBus` with `features.RegisterMessageBus(bus)` before running the suite. bididi supplies the steps and matching logic; the adapter decides how to publish and observe messages. The [test-app adapter](tests/test-app/message_bus_test.go) uses an in-memory bus. A GCP Pub/Sub or Redis Streams adapter can implement the same interface.

```go
type MessageBus interface {
    Publish(ctx context.Context, destination string, data []byte) error
    Observe(ctx context.Context, destination string) (MessageObserver, error)
}

type MessageObserver interface {
    Next(ctx context.Context) (Message, error)
    Close() error
}

type Message struct {
    ID   string
    Data []byte
}
```

`Observe` must establish the observer's starting position before it returns, so messages published after the `Given` step can be read later by `Next`. `Next` waits for a message or its context to end. `Close` releases the observer after the scenario. The adapter can use a separate test subscription for Pub/Sub or a stream cursor for Redis Streams. The application still owns its own consumption and ACK/NACK behavior; the test observer reads independently. Supply stable message IDs when available so redeliveries are not counted twice.

### Observe and publish

```gherkin
Given observe messages from "orders.events"
When publish JSON message to "orders.commands"
  """
  {"order_id":"42"}
  """
```

Start observing before the action that emits events. The publish step sends the JSON DocString as UTF-8 bytes. Destinations and JSON values can use variables and generating wildcards.

### Assert published messages

```gherkin
Then within "5s" messages from "orders.events" contain JSON
  """
  [
    {"type":"InvoiceRequested","order_id":"42","created_at":"$NOW$"},
    {"type":"OrderCreated","order_id":"42","created_at":"$ANY_STR_FULL$"}
  ]
  """
```

The DocString must be a non-empty JSON array of objects. The step waits up to the given positive Go duration. Expected objects match distinct observed messages in any order; unrelated messages and extra object fields are allowed. Comparison wildcards and `$VAR(name)$` work in expected JSON. Observed messages stay in the scenario buffer, so later assertions can inspect them again without another read. An unmet assertion reports the messages observed before the timeout. See [message bus scenarios](tests/test-app/features/message_bus_steps.feature) for unordered events, variables, and wildcards.

## 🃏 Wildcard System

Wildcards generate input data or validate output data, depending on the step:

### Variable Wildcards

- `$VAR(name)$` — value saved earlier in the scenario.

### UUID Wildcards

- `$UUID$` — generate a UUID in input; validate UUID format in assertions.

### NULL Wildcard

- `$NULL$` — insert or compare a null value.

### Time Wildcards

- `$NOW$` — current timestamp.
- `$NOW+1h$` — current timestamp plus a Go duration.
- `$NOW-30m$` — current timestamp minus a Go duration.

### Generic Wildcards (assertions only)

- `$ANY$` — any JSON value, including `null`.
- `$ANY_STR$` — any string, including empty.
- `$ANY_STR_FULL$` — non-empty string.
- `$ANY_INT$` — integer.
- `$ANY_FLOAT$` — numeric value.
- `$ANY_BOOL$` — boolean.

### Custom Wildcards

Use `features.RegisterWildcard(pattern, handler)` to add a token pattern. `WildcardHandler` generates values; implementing `WildcardMatcher` also enables assertion matching. The [$TEST_VALUE$ example](tests/test-app/app_test.go) is exercised in HTTP, database, variable, and cache scenarios.

## 🎯 Complete Example

This scenario uses the handlers and adapters in [test-app](tests/test-app). Its HTTP route, database table, mock endpoint, and cache are deliberately small so each step stays visible:

```gherkin
Feature: Item notification
  Background:
    Given app is running
    And execute SQL "DROP TABLE IF EXISTS step_items"
    And execute SQL "CREATE TABLE step_items (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"

  Scenario: Notify about a stored item
    Given set variable "item_id" to "str:42"
    And add "step_items" records
      | id | name |
      | 1  | Ada  |
    And set cache key "item:$VAR(item_id)$" to JSON
      """
      {"id":"$VAR(item_id)$","name":"Ada"}
      """
    And json mock "notification" for "POST" request to "events"
      | type   | name | value |
      | status |      | 200    |
      """
      {"accepted":true}
      """
    When make a "POST" request to "/items/$VAR(item_id)$/notify"
    Then have JSON response with status 200
      """
      {"item":{"id":"42","name":"Ada"},"notification":{"accepted":true}}
      """
    And json mock "notification" was called with
      | type   | name         | value            |
      | header | Content-Type | application/json |
      """
      {"id":"42","name":"Ada"}
      """
    And cache key "item:42" exists
    And have "step_items" records
      | id | name |
      | 1  | Ada  |
```

## 🧪 Examples and Tests

The applications are separate Go modules with a local `replace` to this repository:

| Module | Purpose |
| --- | --- |
| [examples/example-app](examples/example-app) | Minimal Gin handler assembled with Fx. |
| [tests/test-app](tests/test-app) | BDD contract scenarios for every step group, with GORM/SQLite, an in-memory cache mock, and an in-memory message bus. |

From the repository root, run each module's suite separately:

```sh
go test ./...
(cd examples/example-app && go test ./...)
(cd tests/test-app && go test ./...)
```
