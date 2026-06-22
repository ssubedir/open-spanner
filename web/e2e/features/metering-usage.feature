Feature: Metering and usage exploration
  Dashboard users should define usage signals, ingest usage from trusted
  services, and explore usage with filters, dimensions, charts, and exports.

  Background:
    Given a dashboard user is signed in

  @ui_covered @api_covered
  Scenario: A user creates a meter with dimensions
    When the user creates an API request meter
    Then the meter appears in the meter list
    And the meter detail page shows its dimensions
    And a missing meter detail route shows a centered not-found state

  @ui_covered @api_covered
  Scenario: A user queries usage by nested and hyphenated dimensions
    Given usage exists with nested metadata and a hyphenated metadata key
    When the user groups usage by those dimensions
    Then bucketed usage is shown without dimension errors
    And the expected dimension values appear in the results

  @ui_covered @api_covered
  Scenario: A user saves and opens an advanced usage query
    Given usage exists for multiple subjects and metadata values
    When the user builds an advanced usage query
    And the user saves the query
    Then the query can be reopened from the usage page
    And the query can be shown on the overview page
    And only matching usage appears in buckets and events

  @ui_covered @api_covered
  Scenario: A user opens usage from subject activity
    Given a subject has usage for a meter
    When the user opens usage from the subject page
    Then the usage query is prefilled with the subject and meter

  @ui_covered
  Scenario: A user visualizes usage over time
    Given bucketed usage exists across multiple time windows
    When the user changes chart bucket and chart type
    Then the chart updates without changing the query results
    And cumulative and stacked controls are applied consistently

  @ui_covered
  Scenario: Usage filters remain readable as they grow
    Given an advanced query has nested filter groups
    When the user opens the usage page on desktop and mobile widths
    Then filter controls remain readable
    And no control overlaps another control
