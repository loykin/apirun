# auth_embedded_lazy

This example demonstrates "lazy" authentication acquisition when embedding auth configurations
into the Migrator. Three different auth entries are registered, and each is fetched only when the
first task that uses it is executed (via template reference `{{.auth.NAME}}`).

How it works:

- The program starts a local HTTP server with three endpoints: `/a1`, `/a2`, `/a3`.
- Each endpoint requires a different Basic token (derived from the corresponding auth provider).
- The migrator is configured with three Basic providers named `a1`, `a2`, and `a3`.
- Each migration YAML references the token through a header: `Authorization: {{.auth.aN}}`.
- Tokens are not acquired upfront; the first time the header is rendered for a task, the provider
  is invoked, and the token is cached for later template evaluations.

Run from repository root:

    go run ./examples/auth_embedded_lazy

You should see output indicating that each auth was acquired exactly once and that all migrations
completed successfully.
