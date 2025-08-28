# Store Env Example

This example demonstrates how values extracted from a response via env_from are automatically persisted into the local store and then reused for the down migration (and even later ups).

Key points:
- Use `response.env_from` to extract values from JSON responses using gjson paths.
- All values extracted via `response.env_from` are automatically persisted in the local SQLite store (`apimigrate.db`).
- During `down`, the stored values for that version are automatically merged into the `down.env` so you can template URLs or headers.
- Stored values are also available to subsequent `up` migrations applied in the same run and to future runs (until the version is rolled back).

How it works in this example:
- 001: `up` POSTs to `/create` and receives `{ "id": "123" }`. The migration extracts `rid` from `id` and it is automatically persisted.
- 001: `down` DELETEs `/resource/{{.rid}}`, which resolves to `/resource/123` thanks to the stored value.

This example uses illustrative endpoints. To run it, you should replace the base URL with your own API that mimics the behavior.

## Files
- `migration/001_store_and_delete.yaml`: demonstrates env extraction and persistence in `up`, and the usage in `down`.

## Snippet
```yaml
up:
  name: create resource
  request:
    method: POST
    url: "http://your.api/create"
    body: '{"name":"demo"}'
  response:
    result_code: ["200"]
    env_from:
      rid: id

down:
  name: delete resource
  method: DELETE
  url: "http://your.api/resource/{{.rid}}"
```
