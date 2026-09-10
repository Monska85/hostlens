## ADDED Requirements

### Requirement: Inspectable test coverage

Make and Just SHALL provide equivalent coverage commands that execute tests in disposable containers, print a readable summary, and save a local machine-readable profile and HTML report. Reports SHALL identify their measured code and test scope without treating a percentage as proof of security or completeness. CI SHALL execute the maintained regression and end-to-end checks and retain the coverage report. Test or report-generation failures SHALL fail the command; old output SHALL NOT be presented as a successful new result.

#### Scenario: Inspect local gaps

- **WHEN** a developer runs the coverage command with prepared prerequisites
- **THEN** the command reports measured coverage and identifies a local HTML report for inspecting uncovered code

#### Scenario: Failed coverage run

- **WHEN** a test fails or report generation fails
- **THEN** the command returns failure and does not claim that previous reports validate the current source

#### Scenario: End-to-end scope

- **WHEN** an acceptance test executes separate gateway and diagnostic processes
- **THEN** validation distinguishes that behavioral evidence from code included in coverage instrumentation
