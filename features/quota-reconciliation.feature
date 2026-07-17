Feature: Quota reconciliation
  Dashboard operators should detect quota-counter drift and apply only audited,
  optimistic repairs without changing unrelated usage or decisions.

  Background:
    Given a dashboard user is signed in
    And a subject has an active quota counter

  @ui_covered @api_covered
  Scenario: A user detects and repairs quota counter drift
    Given source usage has drifted from the active quota counter
    When the user runs quota reconciliation
    Then the dashboard shows the counter mismatch
    When the user previews the counter repair
    Then the current and recalculated counter values are shown
    When the user applies the audited repair
    Then a new reconciliation scan is healthy
    And the applied repair appears in repair history
