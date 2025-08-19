# Keycloak Migration Examples

This example demonstrates how to use apimigrate to authenticate with a local Keycloak (admin/root), then create a sample realm and a sample user using the stored access token.

Prerequisites:
- Keycloak running locally and reachable at http://localhost:8080
- Admin user credentials: admin / root
- apimigrate built (go build ./cmd/apimigrate)

Auth configuration:
- examples/keycloak-migration/auth.yaml logs in to Keycloak using Resource Owner Password Credentials against the master realm and the built-in client_id admin-cli. The acquired token is stored under the logical auth name "keycloak" and injected into requests with request.auth_name: keycloak.

How to run (single command):
- We placed two migrations, 001_create_realm.yaml and 002_create_user.yaml, together under examples/keycloak-migration/migration.
- Each migration file specifies its own HTTP method and full URL, so you can run them sequentially with one CLI invocation.

Command (using the combined config with migrate_dir and env):
SAMPLE_REALM=sample ./apimigrate \
  --config examples/keycloak-migration/config.yaml \
  -v

Rollback:
- Show status:
  ./apimigrate status \
    --config examples/keycloak-migration/config.yaml
- Roll back all applied migrations (down to version 0):
  ./apimigrate down \
    --config examples/keycloak-migration/config.yaml \
    --to 0
  This will delete the sample realm (and thus remove the sample user).

Notes:
- The realm name defaults to "sample" via config.yaml (env.realm). You can change it by editing config.yaml; if you prefer using an environment variable, set SAMPLE_REALM and remove the value under env.realm in config.yaml so valueFromEnv can be used.
- The response codes accept both 201 (Created) and 409 (Already exists) to allow re-running safely.
- The request Content-Type header is automatically set to application/json when the body is valid JSON.
- If your Keycloak requires a different client or realm for admin login, adjust auth.yaml accordingly.
