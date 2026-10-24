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

  Scenario: Reuse observed messages for absence, extraction, and exact count
    Given observe messages from "orders.events"
    When publish JSON message to "orders.commands"
      """
      {"order_id":"44"}
      """
    Then within "1s" messages from "orders.events" contain JSON
      """
      [
        {"type":"InvoiceRequested","order_id":"44"},
        {"type":"OrderCreated","order_id":"44"}
      ]
      """
    And extract "type" from message 1 from "orders.events" as "first_event_type"
    And variable "first_event_type" equals "OrderCreated"
    And extract "order_id" from message 2 from "orders.events" as "str:extracted_order_id"
    And variable "extracted_order_id" equals "44"
    And within "30ms" messages from "orders.events" have count 2 matching JSON
      """
      {"order_id":"44"}
      """
    And within "30ms" messages from "orders.events" do not contain JSON
      """
      [
        {"type":"PaymentFailed","order_id":"44"}
      ]
      """

  Scenario: Match published events in arrival order
    Given observe messages from "orders.events"
    When publish JSON message to "orders.events"
      """
      {"type":"Unrelated","order_id":"another"}
      """
    And publish JSON message to "orders.commands"
      """
      {"order_id":"45"}
      """
    Then within "1s" messages from "orders.events" contain JSON in order
      """
      [
        {"type":"OrderCreated","order_id":"45"},
        {"type":"InvoiceRequested","order_id":"45"}
      ]
      """

  Scenario: Wait for a matching message and extract from it
    Given observe messages from "orders.events"
    When publish JSON message to "orders.events"
      """
      {"type":"Unrelated","order_id":"wrong"}
      """
    And publish JSON message to "orders.commands"
      """
      {"order_id":"46"}
      """
    Then within "1s" extract "order_id" from first message from "orders.events" matching JSON as "str:extracted_order_id"
      """
      {"type":"OrderCreated","order_id":"46"}
      """
    And variable "extracted_order_id" equals "46"
