# auth_embedded_mutli_registry_type2

This example demonstrates performing authentication separately from migrations (legacy/decoupled style),
while still using the new struct-based auth API.

It acquires two different Basic auth tokens under logical names `a1` and `a2`, stores them into the
base environment's `.auth` map, and then runs two migrations that target different endpoints requiring
different credentials.

Run:

go run ./examples/auth_embedded_multi_registry_type2

It uses a local httptest server and a temporary SQLite store file, so it does not require internet or
leave files in the repository.
