Feature: Operational reliability
  Operators should see terminal worker failures and safely retry recoverable work
  without losing the original job history.

  Background:
    Given a dashboard user is signed in

  @ui_covered @api_covered
  Scenario: An operator recovers a failed webhook delivery
    Given a webhook delivery has reached terminal failure
    Then the overview reports degraded alert worker health
    And the failed delivery remains visible with its attempt count and error
    When the webhook destination recovers
    And the operator retries the failed delivery
    Then the alert worker delivers the same job
    And duplicate retries are rejected
    And the overview reports healthy alert worker status

  @ui_covered @api_covered
  Scenario: Replicated workers remain observable and claim jobs once
    Given two replicas of the same worker are registered
    Then the overview shows healthy and stale replica counts
    And each replica identity is available for inspection
    When both replicas try to claim one queued job at the same time
    Then exactly one replica claims the job
    When the winning replica stops before completing the job
    Then another replica reclaims the job after its lease expires
