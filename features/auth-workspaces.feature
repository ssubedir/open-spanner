Feature: Authenticated workspace access
  Dashboard users should only see and manage resources that belong to their
  workspace. Backend services should use API keys with explicit scopes.

  Background:
    Given the Open Spanner API, dashboard, and workers are running

  @ui_covered @api_covered
  Scenario: A new dashboard user can sign in and open the dashboard
    Given a dashboard account exists
    When the user signs in with email and password
    Then the overview page is available
    And the signed-in user is shown in the sidebar

  @ui_covered @api_covered
  Scenario: Workspace resources are hidden from another dashboard user
    Given user A has created meters, usage, alerts, exports, API keys, plans, and subjects
    When user B signs in
    Then user B cannot see user A's meters
    And user B cannot see user A's usage
    And user B cannot see user A's alerts
    And user B cannot see user A's exports
    And user B cannot see user A's API keys
    And user B cannot see user A's plans or subjects

  @ui_covered @api_covered
  Scenario: Scoped API keys can write only the allowed meter
    Given a dashboard user has two meters
    And the user creates an API key scoped to write one meter
    When a backend service writes usage for the allowed meter
    Then the usage is accepted
    When the same service writes usage for another meter
    Then the request is denied

  @ui_covered
  Scenario: Dashboard users see clean auth failures
    Given a dashboard session has expired
    When the user opens a protected dashboard page
    Then the user is redirected to sign in
    And raw API errors are not shown
