Feature: Example application

  Scenario: Greeting by name
    Given app is running
    When make a "GET" request to "/hello/Ada"
      """
      {}
      """
    Then have json response with status 200
      """
      {"message":"hello Ada"}
      """
