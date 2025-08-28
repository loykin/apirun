# Auth Embedded Example

This example shows how to embed apimigrate into your Go program and also configure auth programmatically.

It acquires a Basic auth token at startup and stores it under a logical name. A simple migration then uses that auth by referencing `auth_name`.

How to run:

1. From the project root, run:

   go run ./examples/auth_embedded

2. You should see the migration complete successfully.

Notes:
- You can swap the auth provider to `oauth2` if needed; just change the provider type and config map accordingly.
