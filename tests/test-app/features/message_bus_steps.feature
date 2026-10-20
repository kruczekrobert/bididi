Feature: Message bus steps

  Scenario: Match several published events without relying on their order
    Given observe messages from "orders.events"
    When publish JSON message to "orders.events"
      """
      {"type":"Unrelated","order_id":"another"}
      """
    And publish JSON message to "orders.commands"
      """
      {"order_id":"42"}
      """
    Then within "1s" messages from "orders.events" contain JSON
      """
      [
        {"type":"InvoiceRequested","order_id":"42","created_at":"$NOW$"},
        {"type":"OrderCreated","order_id":"42","created_at":"$NOW$"}
      ]
      """
    And within "1s" messages from "orders.events" contain JSON
      """
      [
        {"type":"$ANY_STR_FULL$","order_id":"42"},
        {"type":"OrderCreated","order_id":"42"}
      ]
      """

  Scenario: Resolve variables in published and expected messages
    Given set variable "order_id" to "str:43"
    And observe messages from "orders.events"
    When publish JSON message to "orders.commands"
      """
      {"order_id":"$VAR(order_id)$"}
      """
    Then within "1s" messages from "orders.events" contain JSON
      """
      [
        {"type":"OrderCreated","order_id":"$VAR(order_id)$","created_at":"$ANY_STR_FULL$"},
        {"type":"InvoiceRequested","order_id":"$VAR(order_id)$","created_at":"$ANY_STR_FULL$"}
      ]
      """
