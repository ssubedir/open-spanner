# Acceptance Test Specs

These files are living acceptance-test docs for the dashboard and service flows.
They use Cucumber/Gherkin language so product behavior is easy to review, but the
test runner remains Playwright.

## How To Use These Specs

- Treat `@ui_covered` scenarios as behavior backed by browser Playwright tests.
- Treat `@api_covered` scenarios as behavior backed by HTTP API, worker, or
  service integration tests.
- Treat `@planned` scenarios as accepted product behavior that still needs test
  automation.
- A scenario can have both `@ui_covered` and `@api_covered` when the dashboard
  flow and the service contract are both tested.
- SDK coverage is intentionally not tracked here yet. The current goal is app
  behavior and API confidence.
- Keep scenarios user-facing: describe what a dashboard user or backend service
  does, not implementation details.
- Add new scenarios before broad UI or API work so we know what "done" means.

## Recommended Approach

Use Gherkin as the test plan and Playwright as the execution layer. Full Cucumber
runtime can come later if we need non-engineers to write tests directly, but it
adds glue code and indirection that would slow us down right now.

The practical loop is:

1. Add or update a `.feature` scenario.
2. Implement the behavior.
3. Add or update the Playwright/API/SDK test that proves it.
4. Mark the scenario `@ui_covered`, `@api_covered`, or both.
