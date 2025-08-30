# Auth Embedded Example

This example shows how to embed apimigrate into your Go program and configure auth programmatically.

It acquires a Basic auth token at startup using AcquireAuthAndSetEnv, which stores the token under the internal env key _auth_token for backward compatibility and also under the logical name "basic" in .auth. Important: the library does not add any Authorization prefix automatically. Since this example uses Basic auth, the migration sets the header explicitly as: Authorization: "Basic {{.auth.basic}}". The URL also uses the namespaced env: {{.env.api_base}}.

How to run:

1. From the project root, run:

   go run ./examples/auth_embedded

2. You should see the migration complete successfully.

Notes:
- You can swap the auth provider to `oauth2` if needed; use the matching WithName helper (e.g., AcquireOAuth2ClientCredentialsWithName) and pass your config while keeping the explicit logical name.
