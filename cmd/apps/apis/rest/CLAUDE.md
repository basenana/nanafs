# REST API

This package provides a complete set of RESTFul-style HTTP APIs for NanaFS to offer external services.

## API Style

- APIs primarily adopt the RESTFul style.
- Due to historical reasons, APIs for Entries / Group / File do not fully follow the RESTFul style and are considered
  exceptions.
- Newly added APIs must strictly adhere to the RESTFul style.
- API JSON fields should use the `aaa_bbb` naming convention.

## Code Style

- Newly added code must maintain overall consistency with the existing codebase.
- Code should be self-explanatory; avoid unnecessary comments.
- If comments are absolutely necessary, keep them concise and write them in English.

## Code Quality

- Changes to APIs must be accompanied by corresponding updates or additions to unit tests.
- After modifications, ensure all unit tests pass, even if there were pre-existing issues with the unit tests.

## Keep Documentation Updated

- When API changes are completed, systematically review whether the Request and Response structures have changed, and
  compare them with the API documentation.
- If inconsistencies are found between the code and the API documentation, actively update the API documentation.