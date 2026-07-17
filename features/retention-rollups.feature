Feature: Usage retention and rollups
  Raw usage should age out according to each meter's retention policy without
  changing the usage totals and dimensions available to operators.

  Background:
    Given a dashboard user is signed in
    And a retention-enabled meter exists

  @ui_covered @api_covered
  Scenario: The retention worker preserves usage analytics
    Given usage exists before and inside the meter retention window
    When the retention worker finalizes complete historical hours
    Then expired raw events are removed
    And recent raw events remain available
    And grouped historical usage is unchanged
    And historical dimension values remain available
    And the overview reports healthy retention worker and rollup coverage
