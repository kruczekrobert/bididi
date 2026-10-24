Feature: Cache steps with an in-memory client

  Scenario: Store and compare a string value
    Given set cache key "greeting" to
      """
      hello
      """
    Then cache key "greeting" exists
    And have cache key "greeting" with string
      """
      hello
      """
    And cache mock recorded TTL "0s" for key "greeting"

  Scenario: Store and compare JSON with typed values
    Given set variable "name" to "str:Ada"
    And set variable "count" to "int:7"
    And set cache key "profile" to JSON
      """
      {"name":"$VAR(name)$","count":"$VAR(count)$"}
      """
    Then cache key "profile" exists
    And have cache key "profile" with JSON
      """
      {"name":"Ada","count":"$ANY_INT$"}
      """
    And cache mock recorded TTL "0s" for key "profile"

  Scenario: Store a string with TTL
    Given set cache key "temporary" with TTL "30s" to
      """
      hello
      """
    Then have cache key "temporary" with string
      """
      hello
      """
    And cache mock recorded TTL "30s" for key "temporary"

  Scenario: Store JSON with TTL
    Given set cache key "temporary-json" with TTL "2m" to JSON
      """
      {"name":"Ada"}
      """
    Then have cache key "temporary-json" with JSON
      """
      {"name":"Ada"}
      """
    And cache mock recorded TTL "2m" for key "temporary-json"

  Scenario: Wait for a cache key and its JSON value
    Given set cache key "order:42" to JSON
      """
      {"status":"completed"}
      """
    Then within "100ms" cache key "order:42" exists
    And within "100ms" have cache key "order:42" with JSON
      """
      {"status":"completed"}
      """

  Scenario: Wait for a cache key to expire
    Given set cache key "lock:42" with TTL "30ms" to
      """
      locked
      """
    Then within "500ms" cache key "lock:42" does not exist

  Scenario: An unknown key does not exist
    Then cache key "never-created" does not exist

  Scenario: Variables and custom wildcards resolve in keys and values
    Given set variable "key" to "str:custom-key"
    And set cache key "$VAR(key)$" to
      """
      $TEST_VALUE$
      """
    Then cache key "$VAR(key)$" exists
    And have cache key "$VAR(key)$" with string
      """
      Ada
      """

  Scenario: A custom wildcard generates a cache key and matches a string value
    Given set cache key "profile:$TEST_VALUE$" to
      """
      Ada
      """
    Then cache key "profile:Ada" exists
    And have cache key "profile:Ada" with string
      """
      $TEST_VALUE$
      """
    And have cache key "profile:Ada" with string
      """
      $ANY_STR_FULL$
      """

  Scenario: A custom wildcard generates a key for JSON storage
    Given set cache key "json:$TEST_VALUE$" to JSON
      """
      {"name":"Ada"}
      """
    Then cache key "json:Ada" exists
    And have cache key "json:Ada" with JSON
      """
      {"name":"$ANY_STR_FULL$"}
      """

  Scenario Outline: The cache mock starts empty for every scenario
    Then cache key "shared-key" does not exist
    When set cache key "shared-key" to
      """
      <value>
      """
    Then have cache key "shared-key" with string
      """
      <value>
      """

    Examples:
      | value |
      | Ada   |
      | Bob   |

  Scenario Outline: An outline resolves cache keys and string values
    Given set cache key "outline-<id>" to
      """
      <value>
      """
    Then have cache key "outline-<id>" with string
      """
      <value>
      """

    Examples:
      | id | value |
      | 1  | Ada   |
      | 2  | Bob   |
