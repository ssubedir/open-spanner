Feature: Plans and entitlements
  Dashboard users should define quota packages, assign subjects to plans,
  and inspect usage progress without turning Open Spanner into a payment system.

  Background:
    Given a dashboard user is signed in
    And at least one meter exists

  @ui_covered @api_covered
  Scenario: A user creates a plan with multiple meter limits
    When the user creates a plan with more than one meter limit
    Then the plan catalog shows the plan
    And the plan detail page shows each configured limit

  @ui_covered @api_covered
  Scenario: A user assigns a subject to a plan
    Given a plan exists
    When the user assigns a subject to the plan
    Then the assignment appears on the plan detail page
    And the subject is evaluated against the plan's limits

  @ui_covered @api_covered
  Scenario: A user schedules a subject plan change
    Given a subject has an active plan
    When the user schedules another plan for a future time
    Then the future assignment is shown as scheduled
    And the current plan remains effective before that time
    And another workspace cannot see the scheduled assignment
    When the effective time arrives
    Then the future plan becomes active
    And the previous assignment appears as ended in history
    When the user removes the active assignment
    Then the subject is no longer entitled by either plan
    And the removed assignment appears as ended in history

  @ui_covered @api_covered
  Scenario: Usage updates entitlement state
    Given a subject is assigned to a plan
    When usage is reported for a limited meter
    Then entitlement state is updated by the worker
    And warning or exceeded transitions are recorded when thresholds are crossed

  @ui_covered @api_covered
  Scenario: A user checks usage progress from a plan
    Given a subject is assigned to a plan with usage
    When the user clicks Check progress
    Then progress opens in a modal
    And current usage, limit, remaining usage, and overage are shown

  @ui_covered @api_covered
  Scenario: A user sees plan overage after exceeding a limit
    Given a subject is assigned to a plan with usage over a meter limit
    When the user opens usage progress
    Then the entitlement state is exceeded
    And the amount over the limit is shown

  @ui_covered @api_covered
  Scenario: A user previews a plan version change
    Given a plan has assigned subjects
    When the user edits the plan limits and previews the change
    Then the impact modal shows total subjects
    And the modal shows projected ok, warning, and exceeded counts
    And the modal does not list every subject

  @api_covered
  Scenario: A backend service checks remaining quota
    Given a subject is assigned to a plan
    When a backend service requests entitlement progress with an API key
    Then the response includes remaining quota for each meter limit
