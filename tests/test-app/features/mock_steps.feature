Feature: HTTP mock steps

  Background:
    Given app is running

  Scenario: Two mocks can be declared and checked independently
    Given json mock "notification"
      """
      {"method":"POST","path":"events","status":200,"response":{"accepted":true}}
      """
    And json mock "unused"
      """
      {"method":"POST","path":"unused","status":200,"response":{"accepted":false}}
      """
    When make a "POST" request to "/items/42/notify"
      """
      {}
      """
    Then have json response with status 200
      """
      {"item":{"id":"42","name":"Ada"},"notification":{"accepted":true}}
      """
    And json mock "notification" was called
    And json mock "unused" was not called
    And json mock "notification" was called with
      """
      {"payload":{"id":"42","name":"Ada"},"headers":{"content-type":"application/json"}}
      """

  Scenario: Call matching checks every recorded call
    Given json mock "notification"
      """
      {"method":"POST","path":"events","status":200,"response":{"accepted":true}}
      """
    When make a "POST" request to "/items/42/notify"
      """
      {}
      """
    And make a "POST" request to "/items/43/notify"
      """
      {}
      """
    Then json mock "notification" was called with
      """
      {"payload":{"id":"43","name":"Bob"}}
      """

  Scenario: Variables resolve in mock path and response
    Given set variable "mock_path" to "str:events"
    And set variable "reply" to "str:ok"
    And json mock "configured"
      """
      {"method":"POST","path":"$VAR(mock_path)$","status":200,"response":{"result":"$VAR(reply)$"}}
      """
    When make a "POST" request to "/items/42/notify"
      """
      {}
      """
    Then have json response with status 200
      """
      {"item":{"id":"42","name":"Ada"},"notification":{"result":"ok"}}
      """
    And json mock "configured" was called

  Scenario: A mock status is visible to the application
    Given json mock "unavailable"
      """
      {"method":"POST","path":"events","status":503,"response":{"error":"unavailable"}}
      """
    When make a "POST" request to "/items/42/notify"
      """
      {}
      """
    Then have response with status 502
    And json mock "unavailable" was called

  Scenario: Mock call payload supports comparison wildcards
    Given json mock "notification"
      """
      {"method":"POST","path":"events","status":200,"response":{"accepted":true}}
      """
    When make a "POST" request to "/items/42/notify"
      """
      {}
      """
    Then json mock "notification" was called with
      """
      {"payload":{"id":"$ANY_STR_FULL$","name":"Ada"}}
      """

  Scenario: A variable resolves in the expected mock call
    Given set variable "expected_name" to "str:Ada"
    And json mock "notification"
      """
      {"method":"POST","path":"events","status":200,"response":{"accepted":true}}
      """
    When make a "POST" request to "/items/42/notify"
      """
      {}
      """
    Then json mock "notification" was called with
      """
      {"payload":{"id":"42","name":"$VAR(expected_name)$"}}
      """

  Scenario: Mock response headers reach the application
    Given set variable "header_value" to "str:from-mock"
    And json mock "notification"
      """
      {"method":"POST","path":"events","status":200,"response":{"accepted":true},"headers":{"X-Mock":"$VAR(header_value)$"}}
      """
    When make a "POST" request to "/items/42/notify"
      """
      {}
      """
    Then have json response with status 200
      """
      {"item":{"id":"42","name":"Ada"},"notification":{"accepted":true},"mock_header":"from-mock"}
      """
