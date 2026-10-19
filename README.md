# 📘 bididi — BDD Steps Documentation for Developers

Welcome to the guide for bididi, a reusable set of Godog steps for Go applications. It covers HTTP requests, database operations, mock services, Redis-style cache values, variables, and wildcards. The library accepts application interfaces; Gin, GORM, and Fx appear only in the example modules.

## 📚 Table of Contents

1. [📦 Install](#-install)
2. [🚀 Quick Start](#-quick-start)
3. [📖 Cucumber Basics for Beginners](#-cucumber-basics-for-beginners)
4. [🌐 API Request Steps](#-api-request-steps)
5. [🗄️ Database Steps](#️-database-steps)
6. [🎭 Mock Service Steps](#-mock-service-steps)
7. [🧮 Variable Handling Steps](#-variable-handling-steps)
8. [💾 Redis Steps](#-redis-steps)
9. [🃏 Wildcard System](#-wildcard-system)
10. [🎯 Complete Example](#-complete-example)
11. [🧪 Examples and Tests](#-examples-and-tests)

## 📦 Install

```sh
go get github.com/kruczekrobert/bididi
```

The root module does not require Gin, GORM, Fx, or a Redis client. To use bididi, register an `http.Handler` and, when needed, implementations of these small interfaces:

```go
type Database interface {
    Exec(query string, args ...interface{}) error
    Query(query string, args ...interface{}) ([]map[string]interface{}, error)
    Count(table string) (int64, error)
}

type RedisClient interface {
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
      """
      {}
      """
    Then have json response with status 200
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
      """
      {}
      """
    Then have response with status 200
```

### Background

Use `Background` for steps shared by every scenario in one feature:

```gherkin
Feature: Item lookup
  Background:
    Given app is running

  Scenario: Missing item
    When make a "GET" request to "/items/missing"
      """
      {}
      """
    Then have response with status 404
```

### Scenario Outlines with Examples

An outline runs once per `Examples` row. Gherkin expands `<name>` first; bididi then resolves variables and wildcards in the resulting step, table, or doc string.

```gherkin
Scenario Outline: Existing items
  Given app is running
  When make a "GET" request to "/items/<id>"
    """
    {}
    """
  Then have json response with status 200
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

### Make JSON Request

```gherkin
When make a "METHOD" request to "PATH"
  """
  {"payload": {"name":"Ada"}, "headers": {"X-Trace":"trace-1"}}
  """
```

`payload` and `headers` are optional fields of the request doc string. `payload` is a JSON object. For an empty request, use `{}`. Paths, payload values, and header names and values can contain variables or generating wildcards.

Payload only:

```gherkin
When make a "POST" request to "/echo"
  """
  {"payload":{"name":"Ada"}}
  """
```

Headers only:

```gherkin
When make a "GET" request to "/headers"
  """
  {"headers":{"X-Trace":"$VAR(trace_id)$"}}
  """
```

Payload and headers:

```gherkin
When make a "POST" request to "/echo"
  """
  {"payload":{"count":"$VAR(count)$"},"headers":{"X-Trace":"trace-$UUID$"}}
  """
```

### Assert Response Status

```gherkin
Then have response with status 200
```

Checks the HTTP status. Use an unquoted number.

### Assert JSON Response

```gherkin
Then have json response with status 200
  """
  {"id":"42","name":"$ANY_STR_FULL$"}
  """
```

Checks both status and the entire JSON body. Object keys must match exactly; arrays are compared without relying on order. Expected JSON supports variables, typed wildcards, UUIDs, times, and custom matchers. See [HTTP scenarios](tests/test-app/features/http_steps.feature) for these cases.

## 🗄️ Database Steps

Register a `Database` implementation with `features.RegisterDB(db)` before using these steps. The [test-app adapter](tests/test-app/app_test.go) demonstrates GORM with SQLite.

### Add Records

```gherkin
Given add "step_items" records
  | id | name            |
  | 1  | $VAR(row_name)$ |
```

Inserts rows from a table. Empty cells omit the column so the database default can apply. `$NULL$` inserts SQL `NULL`.

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

See [database scenarios](tests/test-app/features/db_steps.feature) for values, types, tables, SQL, extraction, and outlines.

## 🎭 Mock Service Steps

Call `features.MockURL()` in the Go test and pass the URL to the application under test. The mock server lives in the feature context and is closed by `features.Close()`.

### Define JSON Mock

```gherkin
Given json mock "notification"
  """
  {
    "method": "POST",
    "path": "events",
    "status": 200,
    "response": {"accepted": true},
    "headers": {"X-Mock": "from-mock"}
  }
  """
```

`method`, `path`, and a valid HTTP `status` are required. `headers` sets response headers. Variables and generating wildcards work in the path, response, and response headers.

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
  """
  {
    "payload": {"id":"42","name":"Ada"},
    "headers": {"Content-Type":"application/json"}
  }
  """
```

The assertion succeeds if any recorded call matches the supplied payload and headers. Both fields are optional; JSON comparison supports variables and wildcards. See [mock scenarios](tests/test-app/features/mock_steps.feature) for multiple mocks and response headers.

## 🧮 Variable Handling Steps

### Set Variable

```gherkin
Given set variable "name" to "str:Ada"
```

The value can use a prefix: `str:` keeps text, `int:` makes an integer, `float:` makes a number, `bool:` makes a boolean, and `auto:` infers a type. An unprefixed value is also inferred. Values may contain `$VAR(name)$`, `$UUID$`, `$NOW$`, or a custom generating wildcard.

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

`$VAR(name)$` can appear in later step text, tables, doc strings, mock definitions, and outline examples. `$NULL$` can be compared with `equals` and `not equals`. See [variable scenarios](tests/test-app/features/variable_steps.feature).

## 💾 Redis Steps

Register a `RedisClient` with `features.RegisterRedis(cache)`. The [test-app cache mock](tests/test-app/app_test.go) uses an in-memory map and clears it before each scenario; bididi does not require a Redis server or client package.

### Set Redis Key to String Value

```gherkin
Given set redis key "greeting" to
  """
  hello
  """
```

Keys and values can contain variables and generating wildcards.

Without a TTL, bididi passes `0` to `RedisClient.Set`, meaning no expiration.

### Set Redis Key to String Value with TTL

```gherkin
Given set redis key "temporary" with TTL "30s" to
  """
  hello
  """
```

TTL uses Go duration syntax and must be positive. bididi passes it to the registered client; the client owns expiration behavior.

### Set Redis Key to JSON Value

```gherkin
Given set redis key "profile" to JSON
  """
  {"name":"Ada","count":7}
  """
```

Wildcards are resolved in the JSON structure before it is stored, preserving number and boolean types.

### Set Redis Key to JSON Value with TTL

```gherkin
Given set redis key "temporary-json" with TTL "2m" to JSON
  """
  {"name":"Ada"}
  """
```

### Check Redis Key Exists

```gherkin
Then redis key "profile" exists
```

### Check Redis Key Does Not Exist

```gherkin
Then redis key "missing" does not exist
```

### Check Redis Key Contains String

```gherkin
Then have redis key "greeting" with string
  """
  $ANY_STR_FULL$
  """
```

Literal text, variables, built-in comparison wildcards, and custom matchers are supported.

### Check Redis Key Contains JSON

```gherkin
Then have redis key "profile" with JSON
  """
  {"name":"$ANY_STR_FULL$","count":"$ANY_INT$"}
  """
```

See [Redis scenarios](tests/test-app/features/redis_steps.feature) for all eight steps, TTL propagation, scenario isolation, outlines, and custom wildcards.

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
    And set redis key "item:$VAR(item_id)$" to JSON
      """
      {"id":"$VAR(item_id)$","name":"Ada"}
      """
    And json mock "notification"
      """
      {"method":"POST","path":"events","status":200,"response":{"accepted":true}}
      """
    When make a "POST" request to "/items/$VAR(item_id)$/notify"
      """
      {}
      """
    Then have json response with status 200
      """
      {"item":{"id":"42","name":"Ada"},"notification":{"accepted":true}}
      """
    And json mock "notification" was called with
      """
      {"payload":{"id":"42","name":"Ada"}}
      """
    And redis key "item:42" exists
    And have "step_items" records
      | id | name |
      | 1  | Ada  |
```

## 🧪 Examples and Tests

The applications are separate Go modules with a local `replace` to this repository:

| Module | Purpose |
| --- | --- |
| [examples/example-app](examples/example-app) | Minimal Gin handler assembled with Fx. |
| [tests/test-app](tests/test-app) | BDD contract scenarios for every step group, with GORM/SQLite and an in-memory cache mock. |

From the repository root, run each module's suite separately:

```sh
go test ./...
(cd examples/example-app && go test ./...)
(cd tests/test-app && go test ./...)
```
