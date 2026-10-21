Feature: Variable handling steps

  Background:
    Given app is running

  Scenario Outline: Set variable preserves the requested JSON type
    Given set variable "value" to "<input>"
    When make a "POST" request to "/echo"
      """
      {"value":"$VAR(value)$"}
      """
    Then have JSON response with status 200
      """
      {"payload":{"value":<expected>},"trace":""}
      """

    Examples:
      | input     | expected |
      | str:42    | "42"     |
      | int:42    | 42       |
      | float:3.5 | 3.5      |
      | bool:true | true     |
      | auto:42   | 42       |
      | auto:true | true     |
      | Ada       | "Ada"    |

  Scenario Outline: Compare a variable using every operator
    Given set variable "text" to "str:alpha-beta"
    Then variable "text" <operation> "<expected>"

    Examples:
      | operation    | expected   |
      | equals       | alpha-beta |
      | not equals   | other      |
      | contains     | beta       |
      | not contains | gamma      |

  Scenario: Set a variable from another variable
    Given set variable "first" to "str:Ada"
    And set variable "copy" to "$VAR(first)$"
    Then variable "copy" equals "Ada"

  Scenario: Set variables from a table in row order
    Given set variables
      | name     | value           |
      | order_id | str:42          |
      | count    | int:2           |
      | copy     | $VAR(order_id)$ |
    Then variable "order_id" equals "42"
    And variable "count" equals "2"
    And variable "copy" equals "42"

  Scenario: Variable values are escaped in JSON assertions
    When make a "POST" request to "/echo"
      """
      {"name":"Ada \"Lovelace\""}
      """
    And extract "payload.name" from response as "quoted"
    Then have JSON response with status 200
      """
      {"payload":{"name":"$VAR(quoted)$"},"trace":""}
      """

  Scenario: A custom wildcard can set a variable
    Given set variable "name" to "$TEST_VALUE$"
    Then variable "name" equals "Ada"

  Scenario: Resolve a variable in the variable name and expected value
    Given set variable "target" to "str:code"
    And set variable "code" to "str:Ada"
    And set variable "expected" to "str:Ada"
    Then variable "$VAR(target)$" equals "$VAR(expected)$"

  Scenario: Store and compare a null variable
    Given set variable "empty" to "$NULL$"
    Then variable "empty" equals "$NULL$"
    And variable "empty" not equals "Ada"

  Scenario: A concrete variable differs from NULL
    Given set variable "name" to "str:Ada"
    Then variable "name" not equals "$NULL$"

  Scenario Outline: Extract a response value with a type prefix
    When make a "POST" request to "/echo"
      """
      {"value":<source>}
      """
    And extract "payload.value" from response as "<prefix>result"
    And make a "POST" request to "/echo"
      """
      {"value":"$VAR(result)$"}
      """
    Then have JSON response with status 200
      """
      {"payload":{"value":<expected>},"trace":""}
      """

    Examples:
      | source | prefix | expected |
      | 7      | int:   | 7        |
      | 7      | str:   | "7"      |
      | 3.5    | float: | 3.5      |
      | "true" | bool:  | true     |
      | "42"   | auto:  | 42       |

  Scenario: Extract a value from an array in the response
    When make a "POST" request to "/echo"
      """
      {"items":[{"name":"Ada"}]}
      """
    And extract "payload.items.0.name" from response as "name"
    Then variable "name" equals "Ada"
