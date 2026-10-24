Feature: Basic HTTP steps

  Background:
    Given app is running

  Scenario: GET returns a JSON response
    When make a "GET" request to "/items/42"
    Then have JSON response with status 200
      | type   | name         | value                           |
      | header | Content-Type | application/json; charset=utf-8 |
      """
      {"id":"42","name":"Ada"}
      """

  Scenario: GET returns a missing item status
    When make a "GET" request to "/items/missing"
    Then have response with status 404

  Scenario: GET returns an empty response body
    When make a "GET" request to "/empty"
    Then have empty response with status 204

  Scenario: POST sends a plain JSON payload
    When make a "POST" request to "/echo"
      """
      {"name":"Ada"}
      """
    Then have JSON response with status 200
      """
      {"payload":{"name":"Ada"},"trace":""}
      """

  Scenario: GET sends and checks headers
    When make a "GET" request to "/headers"
      | type   | name    | value         |
      | header | X-Trace | literal-trace |
    Then have JSON response with status 200
      | type   | name    | value         |
      | header | X-Trace | literal-trace |
      """
      {"trace":"literal-trace"}
      """

  Scenario: POST sends a payload and headers
    When make a "POST" request to "/echo"
      | type   | name    | value    |
      | header | X-Trace | combined |
      """
      {"count":2}
      """
    Then have JSON response with status 200
      """
      {"payload":{"count":2},"trace":"combined"}
      """

  Scenario: A request DocString can precede its metadata table
    When make a "POST" request to "/echo"
      """
      {"count":2}
      """
      | type   | name    | value    |
      | header | X-Trace | combined |
    Then have JSON response with status 200
      """
      {"payload":{"count":2},"trace":"combined"}
      """
      | type   | name         | value                           |
      | header | Content-Type | application/json; charset=utf-8 |

  Scenario: A variable resolves in the request path
    Given set variable "item_id" to "str:42"
    When make a "GET" request to "/items/$VAR(item_id)$"
    Then have JSON response with status 200
      """
      {"id":"42","name":"Ada"}
      """

  Scenario: Variables resolve in header names and values
    Given set variable "header_name" to "str:X-Trace"
    And set variable "trace_id" to "str:trace-42"
    When make a "GET" request to "/headers"
      | type   | name               | value           |
      | header | $VAR(header_name)$ | $VAR(trace_id)$ |
    Then have JSON response with status 200
      | type   | name               | value           |
      | header | $VAR(header_name)$ | $VAR(trace_id)$ |
      """
      {"trace":"trace-42"}
      """

  Scenario: Query rows support repeated values and variables
    Given set variable "tag" to "str:bdd"
    When make a "GET" request to "/query"
      | type   | name | value      |
      | query  | tag  | go         |
      | query  | tag  | $VAR(tag)$ |
    Then have JSON response with status 200
      """
      {"tags":["go","bdd"]}
      """

  Scenario: A variable resolves in the request payload and status
    Given set variable "count" to "int:7"
    And set variable "status" to "int:200"
    When make a "POST" request to "/echo"
      """
      {"count":"$VAR(count)$"}
      """
    Then have JSON response with status $VAR(status)$
      """
      {"payload":{"count":7},"trace":""}
      """

  Scenario: A variable resolves in the expected JSON
    Given set variable "expected_name" to "str:Ada"
    When make a "GET" request to "/items/42"
    Then have JSON response with status 200
      """
      {"id":"42","name":"$VAR(expected_name)$"}
      """

  Scenario: JSON assertions accept typed wildcards
    When make a "POST" request to "/echo"
      """
      {"count":7,"price":1.5,"enabled":true,"name":"Ada","empty":"","nothing":null}
      """
    Then have JSON response with status 200
      """
      {"payload":{"count":"$ANY_INT$","price":"$ANY_FLOAT$","enabled":"$ANY_BOOL$","name":"$ANY_STR_FULL$","empty":"$ANY_STR$","nothing":"$NULL$"},"trace":"$ANY$"}
      """

  Scenario: JSON arrays are compared without relying on order
    When make a "POST" request to "/echo"
      """
      {"items":["Ada","Bob"]}
      """
    Then have JSON response with status 200
      """
      {"payload":{"items":["Bob","Ada"]},"trace":""}
      """

  Scenario: ANY accepts values of different JSON types
    When make a "POST" request to "/echo"
      """
      {"number":7,"flag":true,"value":null}
      """
    Then have JSON response with status 200
      """
      {"payload":{"number":"$ANY$","flag":"$ANY$","value":"$ANY$"},"trace":"$ANY$"}
      """

  Scenario: Unordered arrays find a valid assignment for broad and specific values
    When make a "POST" request to "/echo"
      """
      {"items":["Ada","Bob"]}
      """
    Then have JSON response with status 200
      """
      {"payload":{"items":["$ANY_STR$","Ada"]},"trace":""}
      """

  Scenario: Custom wildcard generates input and matches output
    When make a "POST" request to "/echo"
      """
      {"name":"$TEST_VALUE$"}
      """
    Then have JSON response with status 200
      """
      {"payload":{"name":"$TEST_VALUE$"},"trace":""}
      """

  Scenario Outline: An outline replaces values in paths and DocStrings
    When make a "GET" request to "/items/<id>"
    Then have JSON response with status 200
      """
      {"id":"<id>","name":"<name>"}
      """

    Examples:
      | id | name |
      | 42 | Ada  |
      | 43 | Bob  |

  Scenario Outline: Variables from Examples resolve in paths and DocStrings
    Given set variable "item_id" to "str:<raw_id>"
    And set variable "item_name" to "str:<raw_name>"
    When make a "GET" request to "/items/<request_id>"
    Then have JSON response with status 200
      """
      {"id":"<request_id>","name":"<expected_name>"}
      """

    Examples:
      | raw_id | raw_name | request_id     | expected_name   |
      | 42     | Ada      | $VAR(item_id)$ | $VAR(item_name)$ |
      | 43     | Bob      | $VAR(item_id)$ | $VAR(item_name)$ |
