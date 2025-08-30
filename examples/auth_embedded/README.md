# Auth Embedded Example

This example shows how to embed apimigrate into your Go program and configure auth programmatically.

It acquires a Basic auth token at startup using AcquireAuthAndSetEnv, which stores the token under the internal env key _auth_token. Important: the library does not add any Authorization prefix automatically. Since this example uses Basic auth, the migration sets the header explicitly as: Authorization: "Basic {{._auth_token}}". If you use an OAuth2 provider, set it as: Authorization: "Bearer {{._auth_token}}".

How to run:

1. From the project root, run:

   go run ./examples/auth_embedded

2. You should see the migration complete successfully.

Notes:
- You can swap the auth provider to `oauth2` if needed; use the matching WithName helper (e.g., AcquireOAuth2ClientCredentialsWithName) and pass your config while keeping the explicit logical name.
