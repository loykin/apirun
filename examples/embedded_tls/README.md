# Embedded TLS Example

This example shows how to configure TLS client options when running migrations in library (embedded) mode.

Key points
- Set `Migrator.TLSConfig` with a `crypto/tls.Config` to apply to all HTTP requests
- `InsecureSkipVerify` (allow self-signed certificates)
- `MinVersion` / `MaxVersion` (limit allowed TLS versions)

## Run

From the repository root:

```
go run ./examples/embedded_tls
```

The sample sends a GET request to https://httpbin.org/status/200 and expects a 200 response.
To bypass verification for a self-signed HTTPS server (e.g., a local test server), set `InsecureSkipVerify` to true at the top of main.go and change the `URL` environment value to your server address.

Example:
- URL: https://localhost:8443/status
- TLS: InsecureSkipVerify = true
- Adjust MinVersion/MaxVersion as needed

## Files
- `main.go` – minimal program configuring and running apimigrate.Migrator with TLS options
- `migration/001_check.yaml` – simple up migration that calls an HTTPS endpoint
