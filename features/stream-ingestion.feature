Feature: Streaming ingestion
  Backend services should be able to report usage through the gRPC streaming
  surface while preserving the same meter, usage, and entitlement behavior.

  Background:
    Given the Open Spanner API and gRPC server are running

  @api_covered
  Scenario: gRPC streaming accepts usage events
    Given a meter exists
    And an API key can write usage for that meter
    When a backend service streams usage events
    Then the events are accepted by the service
    And the events can be queried through the REST API

  @api_covered
  Scenario: Streamed usage updates entitlement state
    Given a subject is assigned to a plan
    When a backend service streams usage over a plan limit
    Then entitlement state changes to exceeded
    And the exceeded transition is visible through entitlement events
