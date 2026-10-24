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

The root module does not require Gin, GORM, Fx, a cache client, or a message broker client package. To use bididi, register an `http.Handler` and, when needed, implementations of these small interfaces for database, cache, and message bus / Pub/Sub integration:

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

Use `features.RegisterDB(db)`, `features.RegisterCache(cache)`, and `features.RegisterMessageBus(bus)` to enable the corresponding steps. A message bus adapter can connect to GCP Pub/Sub, Redis Streams, or another broker. It handles publishing and observing messages; bididi provides JSON assertions, negative assertions, exact matching counts, and value extraction. See [Message Bus Steps](#-message-bus-steps) for the adapter contract and step syntax.

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

This sends a request without a body or metadata table. It also works for other methods, such as a POST without a body. The same step accepts an optional metadata table and an optional JSON DocString.

Add a table for query parameters or request headers:

```gherkin
When make a "GET" request to "/query"
  | type  | name | value |
  | query | tag  | go    |
```

Add a DocString when sending a JSON body:

```gherkin
When make a "POST" request to "/echo"
  """
  {"name":"Ada"}
  """
```

To send both a JSON body and metadata:

```gherkin
When make a "POST" request to "/echo"
  | type   | name    | value           |
  | query  | source  | checkout        |
  | header | X-Trace | $VAR(trace_id)$ |
  """
  {"name":"Ada"}
  """
```

The optional metadata table uses columns `type`, `name`, and `value`. Repeat `query` rows for repeated parameters. The optional DocString is the request JSON object itself. The table and DocString can appear in either order. Method, path, table names and values support variables and generating wildcards.

The previous request suffixes `with metadata`, `with JSON`, and `with JSON and metadata` are no longer registered; remove the suffix and keep the existing table and/or DocString.

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

To check response headers as well, add an optional table to the same step:

```gherkin
Then have JSON response with status 200
  | type   | name         | value                           |
  | header | Content-Type | application/json; charset=utf-8 |
  """
  {"id":"42","name":"$ANY_STR_FULL$"}
  """
```

The optional table checks only the listed response headers. The JSON DocString remains required. Remove `with metadata` from existing JSON response assertions.

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

For effects produced asynchronously, add a timeout. The assertion is retried until it succeeds or the positive Go duration expires:

```gherkin
Then within "5s" have "orders" records
  | id | status    |
  | 42 | completed |
```

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
Given json mock "notification" for "POST" request to "events" with status 200
  | type            | name   | value     |
  | query           | source | checkout  |
  | response-header | X-Mock | from-mock |
  """
  {"accepted":true}
  """
```

Method, path, and the required status are supplied in the step text. The status configures the HTTP response returned by the mock. The metadata table is optional and accepts `query` and `response-header` rows. `query` selects which incoming request the mock handles. `response-header` sets a header returned by the mock. The required DocString is the JSON response body. Variables and generating wildcards work in the step text, table, and body. Move existing `status` rows into `with status ...` in the step text; `status` rows are no longer accepted.

Without additional metadata:

```gherkin
Given json mock "unavailable" for "POST" request to "events" with status 503
  """
  {"error":"unavailable"}
  """
```

### Verify Mock Was Called

```gherkin
Then json mock "notification" was called
```

### Verify Mock Was Not Called

```gherkin
Then json mock "unused" was not called
```

Declare `unused` first; assertions refer to named mocks already in the registry.

To prove that an asynchronous action does not call a mock during an entire window, use `for`:

```gherkin
Then for "500ms" json mock "refund" was not called
```

### Wait for Mock Calls

```gherkin
Then within "3s" json mock "billing" was called
And within "3s" json mock "billing" was called 2 times
```

Both steps retry until the condition becomes true or the positive Go duration expires. The count step succeeds as soon as the current call count equals the expected count. Mock call history is safe to read while the application is making concurrent requests.

### Verify Mock Was Called With Specific Data

```gherkin
Then json mock "notification" was called with status 200
  | type            | name         | value            |
  | query           | source       | checkout         |
  | header          | Content-Type | application/json |
  | response-header | X-Mock       | from-mock        |
  """
  {"id":"42","name":"Ada"}
  """
```

The `was called` step accepts an optional `with status ...` suffix, an optional metadata table, and an optional JSON request DocString. Use the same step for a body-only assertion, a table-only assertion, or both. Replace the previous `was called with` and `was called with metadata` spellings with `was called`, optionally followed by `with status ...`.

`header` checks a header received by the mock. `response-header` checks a header sent by the mock. The table accepts `query`, `header`, and `response-header`; status belongs in the step text. The assertion succeeds if one recorded call matches the expected returned status, listed query parameters, headers, and JSON request body. Variables and comparison wildcards work in assertions, including the status.

To check only that the mock returned a particular status:

```gherkin
Then json mock "unavailable" was called with status 503
```

The application's HTTP response status is checked separately with `Then have response with status 200`. See [mock scenarios](tests/test-app/features/mock_steps.feature).

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

### Wait for Cache State

Use `within` when a worker updates Redis or another cache asynchronously. The assertion is retried until it succeeds or the positive Go duration expires:

```gherkin
Then within "2s" cache key "order:42" exists
And within "2s" have cache key "order:42" with JSON
  """
  {"status":"completed"}
  """
And within "5s" cache key "lock:42" does not exist
```

`does not exist` means that the key must become absent before the timeout; it may succeed immediately if the key is already absent.

See [cache scenarios](tests/test-app/features/cache_steps.feature) for cache setup, waiting, TTL propagation, scenario isolation, outlines, and custom wildcards.

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

### Assert published messages in order

```gherkin
Then within "5s" messages from "orders.events" contain JSON in order
  """
  [
    {"type":"OrderCreated","order_id":"42"},
    {"type":"InvoiceRequested","order_id":"42"}
  ]
  """
```

The expected objects must form an ordered subsequence of unique observed messages. Unrelated messages may occur before, between, or after them. Matching remains partial, so extra object fields are allowed.

### Assert that messages are absent

```gherkin
Then within "500ms" messages from "orders.events" do not contain JSON
  """
  [
    {"type":"PaymentFailed","order_id":"42"}
  ]
  """
```

The DocString has the same non-empty array format and partial-object matching rules as `contain JSON`. A forbidden match fails the step. A successful negative assertion always waits for the complete time window, so an event arriving near the end is not missed.

### Extract from an observed message

```gherkin
And extract "payload.order_id" from message 1 from "orders.events" as "order_id"
```

Extraction reads the existing scenario buffer; it does not wait for another message. Message numbers are one-based and follow the order in which `MessageObserver.Next` returned unique messages. The path uses dots for object fields and numeric array indexes. Variable-name prefixes `str:`, `int:`, `float:`, `bool:`, and `auto:` work as they do for response extraction.

To wait for a specific message and avoid relying on its arrival number, match it by partial JSON and extract from the first matching unique message:

```gherkin
Then within "5s" extract "payload.order_id" from first message from "orders.events" matching JSON as "str:order_id"
  """
  {
    "type":"OrderCreated",
    "correlation_id":"$VAR(correlation_id)$"
  }
  """
```

The step checks the existing scenario buffer first and then reads until a match is found or the timeout expires.

### Assert an exact matching count

```gherkin
Then within "500ms" messages from "orders.events" have count 2 matching JSON
  """
  {"type":"OrderCreated"}
  """
```

The DocString is one JSON object and uses partial-object matching, variables, and comparison wildcards. The step waits for the complete time window before comparing the exact count, so later duplicates or extra events are detected. Messages with the same non-empty `Message.ID` are counted once; when an adapter cannot provide an ID, each delivery is treated as a separate message.

All message steps share one per-destination scenario buffer. Matching assertions are intentionally order-independent. Only the numbered extraction step depends on observer arrival order, which may differ from publish order when a broker or concurrent consumers do not guarantee ordering.

## 🃏 Wildcard System

Wildcards generate input data or validate output data, depending on the step:

### Variable Wildcards

- `$VAR(name)$` — value saved earlier in the scenario.

### UUID Wildcards

- `$UUID$` — generate a UUID in input; validate UUID format in assertions.

### NULL Wildcard

- `$NULL$` — insert or compare a null value.

### Time Wildcards

- `$NOW$` — current timestamp; comparisons allow a default tolerance of ±1 second.
- `$NOW+1h$` — current timestamp plus a Go duration; comparisons allow a default tolerance of ±1 second around the shifted time.
- `$NOW-30m$` — current timestamp minus a Go duration; comparisons allow a default tolerance of ±1 second around the shifted time.
- `$NOW_WITHIN(5s)$` — comparison-only wildcard requiring the timestamp to be within the given positive Go duration of now.
- `$NOW+1h_WITHIN(10s)$` — comparison-only wildcard requiring the timestamp to be within ±10 seconds of now plus one hour. Negative offsets use the same form, for example `$NOW-30m_WITHIN(2s)$`.

### Generic Wildcards (assertions only)

- `$ANY$` — any JSON value, including `null`.
- `$ANY_STR$` — any string, including empty.
- `$ANY_STR_FULL$` — non-empty string.
- `$ANY_INT$` — integer.
- `$ANY_FLOAT$` — numeric value.
- `$ANY_BOOL$` — boolean.
- `$ANY_ARRAY$` — any JSON array, including an empty array. `$ANY$` already accepts arrays as well as every other JSON type.
- `$NOT_NULL$` — any non-null JSON value.

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
    And json mock "notification" for "POST" request to "events" with status 200
      """
      {"accepted":true}
      """
    When make a "POST" request to "/items/$VAR(item_id)$/notify"
    Then have JSON response with status 200
      """
      {"item":{"id":"42","name":"Ada"},"notification":{"accepted":true}}
      """
    And json mock "notification" was called with status 200
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
