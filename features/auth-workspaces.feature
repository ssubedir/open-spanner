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
  Scenario: An owner invites a teammate with viewer access
    Given a workspace owner and another registered account exist
    When the owner creates a viewer invitation for the teammate's email
    Then a secure invitation link is shown once
    And the invitation expires after seven days
    When the teammate signs in with the invited email and accepts the invitation
    Then the teammate becomes a viewer in the shared workspace
    And the teammate can read workspace resources
    But the teammate cannot change workspace resources

  @api_covered
  Scenario: Invitation links enforce their complete security lifecycle
    Given a pending workspace invitation exists
    Then another active invitation cannot be created for the same email
    And an account with a different email cannot accept the invitation
    When the owner revokes the invitation
    Then the revoked invitation cannot be accepted
    When a replacement invitation is accepted
    Then it cannot be accepted a second time
    When an invitation expires
    Then the expired invitation cannot be accepted
    And the API reports pending, accepted, revoked, and expired invitation states
    But invitation history never exposes invitation tokens

  @ui_covered @api_covered
  Scenario: A member switches between personal and shared workspaces
    Given a user belongs to a personal workspace and a shared workspace
    When the user selects the personal workspace in the dashboard
    Then the session is scoped to the personal workspace
    When the user creates a resource and switches to the shared workspace
    Then the personal resource is not visible in the shared workspace

  @api_covered
  Scenario: Membership role changes take effect immediately
    Given an admin has a write-capable API key in a shared workspace
    Then the admin can create and revoke teammate invitations
    When an owner changes the admin to a viewer
    Then the viewer's existing session becomes read-only
    And the viewer's existing API key becomes read-only
    And the viewer cannot manage invitations
    When the owner promotes the viewer to owner
    Then the promoted member can manage workspace membership

  @api_covered
  Scenario: A workspace always retains an owner
    Given a workspace has one owner
    Then the final owner cannot be demoted
    And the final owner cannot be removed
    When the workspace has another owner
    Then the first owner can be demoted
    But the remaining final owner still cannot be demoted or removed

  @ui_covered @api_covered
  Scenario: Removing a member revokes workspace access
    Given a user belongs to a shared workspace
    When an owner removes the user from that workspace
    Then the user's existing session can no longer access that workspace
    And the user's API keys for that workspace can no longer authenticate

  @ui_covered @api_covered
  Scenario: Scoped API keys can write only the allowed meter
    Given a dashboard user has two meters
    And the user creates an API key scoped to write one meter
    When a backend service writes usage for the allowed meter
    Then the usage is accepted
    When the same service writes usage for another meter
    Then the request is denied

  @ui_covered @api_covered
  Scenario: A user manages the complete API key lifecycle
    When the user creates an expiring API key
    Then the key can authenticate and its expiration is recorded
    When the user rotates the key with a grace period
    Then both keys can authenticate during the grace period
    And revoking the old key leaves only the replacement able to authenticate
    And the rotation appears in lifecycle history
    When another API key reaches its expiration
    Then the expired key cannot authenticate and is marked expired
    When the user revokes the replacement key
    Then the replacement key cannot authenticate
    And the revocation appears in lifecycle history

  @ui_covered
  Scenario: Dashboard users see clean auth failures
    Given a dashboard session has expired
    When the user opens a protected dashboard page
    Then the user is redirected to sign in
    And raw API errors are not shown
