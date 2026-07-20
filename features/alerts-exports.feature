Feature: Alerts and exports
  Dashboard users should configure operational signals and export usage data
  without blocking normal ingestion and querying flows.

  Background:
    Given a dashboard user is signed in
    And at least one meter exists

  @ui_covered @api_covered
  Scenario: A user creates a webhook alert
    Given an alert destination exists
    When the user creates a threshold rule for a meter
    And matching usage is reported
    Then the alert worker records a triggered event
    And the webhook receives the alert payload
    And the webhook request includes signing headers

  @api_covered
  Scenario: An alert with no matching usage remains healthy
    Given a threshold rule targets a subject without usage
    When the rule is evaluated
    Then the rule state is no_data with a zero value
    And no evaluation_failed event is recorded

  @ui_covered
  Scenario: A user inspects an alert event
    Given an alert event exists
    When the user opens the alert event
    Then the event modal shows the rule, trigger value, condition, and payload JSON

  @ui_covered @api_covered
  Scenario: A user exports the current usage query
    Given a usage query returns matching buckets
    When the user exports current buckets
    Then the downloaded CSV contains the current query results
    When the user exports current events
    Then the downloaded CSV contains the matching raw events

  @ui_covered @api_covered
  Scenario: A user queues a usage export job
    Given a usage query returns matching buckets
    When the user queues an export job
    Then the export worker completes the job
    And the export appears on the exports page
    And the generated CSV can be downloaded

  @ui_covered @api_covered
  Scenario: A user cancels and retries an export job
    Given a queued usage export exists
    When the export is canceled
    Then the canceled export cannot be downloaded
    And an API key without export write access cannot cancel or retry it
    When the user retries the canceled export
    Then the worker completes the same export job
    And the generated CSV can be downloaded

  @ui_covered @api_covered
  Scenario: Failed exports are visible without blocking other jobs
    Given one export job fails
    When the user opens the exports page
    Then the failed job shows its error state
    And other queued jobs can still complete
