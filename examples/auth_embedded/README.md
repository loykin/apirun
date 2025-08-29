# Auth Embedded Example

This example shows how to embed apimigrate into your Go program and configure auth programmatically using the new WithName helpers.

It acquires a Basic auth token at startup with AcquireBasicAuthWithName and stores it under an explicit logical name. A simple migration then uses that auth by referencing `auth_name`.

How to run:

1. From the project root, run:

   go run ./examples/auth_embedded

2. You should see the migration complete successfully.

Notes:
- You can swap the auth provider to `oauth2` if needed; use the matching WithName helper (e.g., AcquireOAuth2ClientCredentialsWithName) and pass your config while keeping the explicit logical name.
