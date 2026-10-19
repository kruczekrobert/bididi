Feature: Basic database steps

  Background:
    Given execute SQL "DROP TABLE IF EXISTS step_items"
    And execute SQL "CREATE TABLE step_items (id INTEGER PRIMARY KEY, name TEXT NOT NULL, amount REAL, note TEXT DEFAULT 'default-note', token TEXT, created_at TEXT)"

  Scenario: Empty table
    Then have empty "step_items" table
    And have "0" records in "step_items" table

  Scenario: Add and read one record
    Given add "step_items" records
      | id | name |
      | 1  | Ada  |
    Then have "step_items" records
      | id | name |
      | 1  | Ada  |
    And have "1" records in "step_items" table

  Scenario: Execute SQL changes the table
    Given add "step_items" records
      | id | name |
      | 1  | Ada  |
    When execute SQL "DELETE FROM step_items WHERE id = 1"
    Then have empty "step_items" table

  Scenario: Extract one value from the database
    Given add "step_items" records
      | id | name |
      | 1  | Ada  |
    When extract from database "SELECT name FROM step_items WHERE id = 1" as "name"
    Then variable "name" equals "Ada"

  Scenario: Read multiple records without depending on their order
    Given add "step_items" records
      | id | name |
      | 1  | Ada  |
      | 2  | Bob  |
    Then have "step_items" records
      | name |
      | Bob  |
      | Ada  |
    And have "2" records in "step_items" table

  Scenario: Empty table cells use the database default
    Given add "step_items" records
      | id | name | note |
      | 1  | Ada  |      |
    Then have "step_items" records
      | id | note         |
      | 1  | default-note |

  Scenario: Null values survive a data table insert
    Given add "step_items" records
      | id | name | note   |
      | 1  | Ada  | $NULL$ |
    Then have "step_items" records
      | id | note   |
      | 1  | $NULL$ |

  Scenario: A decimal value survives a data table insert
    Given add "step_items" records
      | id | name | amount |
      | 1  | Ada  | 3.5    |
    Then have "step_items" records
      | id | amount |
      | 1  | 3.5    |

  Scenario: Compare database values with typed wildcards
    Given add "step_items" records
      | id | name | amount | note |
      | 1  | Ada  | 3.5    |      |
    Then have "step_items" records
      | id        | name           | amount      | note      |
      | $ANY_INT$ | $ANY_STR_FULL$ | $ANY_FLOAT$ | $ANY_STR$ |
    And have "step_items" records
      | name  |
      | $ANY$ |

  Scenario: Variables resolve in data tables and the record count
    Given set variable "row_name" to "str:Ada"
    And set variable "expected_count" to "str:1"
    And add "step_items" records
      | id | name            |
      | 1  | $VAR(row_name)$ |
    Then have "step_items" records
      | id | name            |
      | 1  | $VAR(row_name)$ |
    And have "$VAR(expected_count)$" records in "step_items" table

  Scenario: Execute SQL updates an existing record
    Given add "step_items" records
      | id | name |
      | 1  | Ada  |
    When execute SQL "UPDATE step_items SET name = 'Bob' WHERE id = 1"
    Then have "step_items" records
      | id | name |
      | 1  | Bob  |

  Scenario: A variable resolves in the extraction query
    Given set variable "wanted_id" to "str:1"
    And add "step_items" records
      | id | name |
      | 1  | Ada  |
    When extract from database "SELECT name FROM step_items WHERE id = $VAR(wanted_id)$" as "found_name"
    Then variable "found_name" equals "Ada"

  Scenario: Extract a row with multiple columns
    Given add "step_items" records
      | id | name |
      | 1  | Ada  |
    And app is running
    When extract from database "SELECT id, name FROM step_items WHERE id = 1" as "item"
    And make a "POST" request to "/echo"
      """
      {"payload":{"item":"$VAR(item)$"}}
      """
    Then have json response with status 200
      """
      {"payload":{"item":{"id":1,"name":"Ada"}},"trace":""}
      """

  Scenario Outline: Extract a database value with a type prefix
    Given add "step_items" records
      | id | name | amount |
      | 1  | Ada  | 3.5    |
    And app is running
    When extract from database "<query>" as "<prefix>result"
    And make a "POST" request to "/echo"
      """
      {"payload":{"value":"$VAR(result)$"}}
      """
    Then have json response with status 200
      """
      {"payload":{"value":<expected>},"trace":""}
      """

    Examples:
      | query                                        | prefix | expected |
      | SELECT id FROM step_items WHERE id = 1       | int:   | 1        |
      | SELECT amount FROM step_items WHERE id = 1   | float: | 3.5      |
      | SELECT amount FROM step_items WHERE id = 1   | str:   | "3.5"    |
      | SELECT 'true'                                | bool:  | true     |
      | SELECT '42'                                  | auto:  | 42       |

  Scenario Outline: An outline replaces values in a data table
    Given add "step_items" records
      | id   | name   |
      | <id> | <name> |
    Then have "step_items" records
      | id   | name   |
      | <id> | <name> |

    Examples:
      | id | name |
      | 3  | Ada  |
      | 4  | Bob  |

  Scenario Outline: Variables from Examples resolve in data tables
    Given set variable "record_name" to "str:<raw_name>"
    And add "step_items" records
      | id   | name            |
      | <id> | <name_from_var> |
    Then have "step_items" records
      | id   | name            |
      | <id> | <name_from_var> |

    Examples:
      | id | raw_name | name_from_var      |
      | 5  | Ada      | $VAR(record_name)$ |
      | 6  | Bob      | $VAR(record_name)$ |

  Scenario: Generate and compare a UUID in a data table and DocString
    Given add "step_items" records
      | id | name | token  |
      | 1  | Ada  | $UUID$ |
    Then have "step_items" records
      | id | token  |
      | 1  | $UUID$ |
    And extract from database "SELECT token FROM step_items WHERE id = 1" as "token"
    And app is running
    When make a "POST" request to "/echo"
      """
      {"payload":{"token":"$VAR(token)$"}}
      """
    Then have json response with status 200
      """
      {"payload":{"token":"$UUID$"},"trace":""}
      """

  Scenario Outline: Generate and compare time wildcards in a data table
    Given add "step_items" records
      | id | name | created_at |
      | 1  | Ada  | <time>     |
    Then have "step_items" records
      | id | created_at |
      | 1  | <time>     |

    Examples:
      | time      |
      | $NOW$     |
      | $NOW+1h$  |
      | $NOW-30m$ |

  Scenario: Variables resolve in execute SQL and integer record counts
    Given set variable "name" to "str:Ada"
    And set variable "count" to "int:1"
    When execute SQL "INSERT INTO step_items (id, name) VALUES (1, '$VAR(name)$')"
    Then have "$VAR(count)$" records in "step_items" table
    And have "step_items" records
      | id | name |
      | 1  | Ada  |

  Scenario: Custom wildcard works in database tables
    Given add "step_items" records
      | id | name         |
      | 1  | $TEST_VALUE$ |
    Then have "step_items" records
      | id | name         |
      | 1  | $TEST_VALUE$ |
