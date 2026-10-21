Feature: HTTP mock steps

  Background:
    Given app is running

  Scenario: Two mocks can be declared and checked independently
    Given json mock "notification" for "POST" request to "events" with status 200
      """
      {"accepted":true}
      """
    And json mock "unused" for "POST" request to "unused" with status 200
      """
      {"accepted":false}
      """
    When make a "POST" request to "/items/42/notify"
    Then have JSON response with status 200
      """
      {"item":{"id":"42","name":"Ada"},"notification":{"accepted":true}}
      """
    And json mock "notification" was called
    And json mock "unused" was not called
    And json mock "notification" was called with status 200
      | type   | name         | value            |
      | header | Content-Type | application/json |
      """
      {"id":"42","name":"Ada"}
      """

  Scenario: Call matching checks every recorded call
    Given json mock "notification" for "POST" request to "events" with status 200
      """
      {"accepted":true}
      """
    When make a "POST" request to "/items/42/notify"
    And make a "POST" request to "/items/43/notify"
    Then json mock "notification" was called with status 200
      | type   | name         | value            |
      | header | Content-Type | application/json |
      """
      {"id":"43","name":"Bob"}
      """

  Scenario: Variables resolve in mock path and response
    Given set variable "mock_path" to "str:events"
    And set variable "reply" to "str:ok"
    And set variable "mock_status" to "int:200"
    And json mock "configured" for "POST" request to "$VAR(mock_path)$" with status $VAR(mock_status)$
      """
      {"result":"$VAR(reply)$"}
      """
    When make a "POST" request to "/items/42/notify"
    Then have JSON response with status 200
      """
      {"item":{"id":"42","name":"Ada"},"notification":{"result":"ok"}}
      """
    And json mock "configured" was called with status $VAR(mock_status)$

  Scenario: A mock status is visible to the application
    Given json mock "unavailable" for "POST" request to "events" with status 503
      """
      {"error":"unavailable"}
      """
    When make a "POST" request to "/items/42/notify"
    Then have response with status 502
    And json mock "unavailable" was called with status 503

  Scenario: Mock call payload supports comparison wildcards
    Given json mock "notification" for "POST" request to "events" with status 200
      """
      {"accepted":true}
      """
    When make a "POST" request to "/items/42/notify"
    Then json mock "notification" was called with status $ANY_INT$
      | type   | name         | value          |
      | header | Content-Type | $ANY_STR_FULL$ |
      """
      {"id":"$ANY_STR_FULL$","name":"Ada"}
      """

  Scenario: A variable resolves in the expected mock call
    Given set variable "expected_name" to "str:Ada"
    And json mock "notification" for "POST" request to "events" with status 200
      """
      {"accepted":true}
      """
    When make a "POST" request to "/items/42/notify"
    Then json mock "notification" was called
      """
      {"id":"42","name":"$VAR(expected_name)$"}
      """

  Scenario: Mock query and response headers support variables and assertions
    Given set variable "source" to "str:checkout"
    And set variable "reply_header" to "str:from-mock"
    And json mock "notification" for "POST" request to "events" with status 200
      | type            | name   | value              |
      | query           | source | $VAR(source)$      |
      | response-header | X-Mock | $VAR(reply_header)$ |
      """
      {"accepted":true}
      """
    When make a "POST" request to "/items/42/notify"
      | type   | name   | value         |
      | query  | source | $VAR(source)$    |
    Then have JSON response with status 200
      """
      {"item":{"id":"42","name":"Ada"},"notification":{"accepted":true},"mock_header":"from-mock"}
      """
    And json mock "notification" was called with status 200
      | type            | name         | value              |
      | query           | source       | $VAR(source)$      |
      | header          | Content-Type | application/json   |
      | response-header | X-Mock       | $VAR(reply_header)$ |
      """
      {"id":"42","name":"Ada"}
      """

  Scenario: Mock metadata can be checked without a request body expectation
    Given json mock "notification" for "POST" request to "events" with status 200
      """
      {"accepted":true}
      """
    When make a "POST" request to "/items/42/notify"
    Then json mock "notification" was called
      | type   | name         | value            |
      | header | Content-Type | application/json |
