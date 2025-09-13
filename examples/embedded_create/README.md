# Embedded Create Example

This example demonstrates how to programmatically create a new migration file using the library API.

It uses a timestamp-based filename (UTC) and writes a ready-to-edit YAML template.

## Run

From the repository root:

```
# Create a new migration with default directory (examples/embedded_create/migration)
go run ./examples/embedded_create "create user"

# Specify a custom directory
-go run ./examples/embedded_create --dir ./config/migration "init"
```

The program prints the full path of the newly created file, e.g.:

```
examples/embedded_create/migration/20250914005830_create_user.yaml
```

You can open the file and edit the request/response sections. The template contains:
- up section with a GET request to https://example.com
- headers list demonstrating proper format
- response with result_code: ["200"]
- a commented example down section

## Notes
- The filename format is `YYYYMMDDHHMMSS_slug.yaml` where `slug` is derived from your provided name.
- The function will not overwrite an existing file; if a file with the same name exists (same second), it returns an error.
