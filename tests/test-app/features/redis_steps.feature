Feature: Redis steps with an in-memory client

  Scenario: Store and compare a string value
    Given set redis key "greeting" to
      """
      hello
      """
    Then redis key "greeting" exists
    And have redis key "greeting" with string
      """
      hello
      """
    And cache mock recorded TTL "0s" for key "greeting"

  Scenario: Store and compare JSON with typed values
    Given set variable "name" to "str:Ada"
    And set variable "count" to "int:7"
    And set redis key "profile" to JSON
      """
      {"name":"$VAR(name)$","count":"$VAR(count)$"}
      """
    Then redis key "profile" exists
    And have redis key "profile" with JSON
      """
      {"name":"Ada","count":"$ANY_INT$"}
      """
    And cache mock recorded TTL "0s" for key "profile"

  Scenario: Store a string with TTL
    Given set redis key "temporary" with TTL "30s" to
      """
      hello
      """
    Then have redis key "temporary" with string
      """
      hello
      """
    And cache mock recorded TTL "30s" for key "temporary"

  Scenario: Store JSON with TTL
    Given set redis key "temporary-json" with TTL "2m" to JSON
      """
      {"name":"Ada"}
      """
    Then have redis key "temporary-json" with JSON
      """
      {"name":"Ada"}
      """
    And cache mock recorded TTL "2m" for key "temporary-json"

  Scenario: An unknown key does not exist
    Then redis key "never-created" does not exist

  Scenario: Variables and custom wildcards resolve in keys and values
    Given set variable "key" to "str:custom-key"
    And set redis key "$VAR(key)$" to
      """
      $TEST_VALUE$
      """
    Then redis key "$VAR(key)$" exists
    And have redis key "$VAR(key)$" with string
      """
      Ada
      """

  Scenario: A custom wildcard generates a cache key and matches a string value
    Given set redis key "profile:$TEST_VALUE$" to
      """
      Ada
      """
    Then redis key "profile:Ada" exists
    And have redis key "profile:Ada" with string
      """
      $TEST_VALUE$
      """
    And have redis key "profile:Ada" with string
      """
      $ANY_STR_FULL$
      """

  Scenario: A custom wildcard generates a key for JSON storage
    Given set redis key "json:$TEST_VALUE$" to JSON
      """
      {"name":"Ada"}
      """
    Then redis key "json:Ada" exists
    And have redis key "json:Ada" with JSON
      """
      {"name":"$ANY_STR_FULL$"}
      """

  Scenario Outline: The cache mock starts empty for every scenario
    Then redis key "shared-key" does not exist
    When set redis key "shared-key" to
      """
      <value>
      """
    Then have redis key "shared-key" with string
      """
      <value>
      """

    Examples:
      | value |
      | Ada   |
      | Bob   |

  Scenario Outline: An outline resolves Redis keys and string values
    Given set redis key "outline-<id>" to
      """
      <value>
      """
    Then have redis key "outline-<id>" with string
      """
      <value>
      """

    Examples:
      | id | value |
      | 1  | Ada   |
      | 2  | Bob   |
