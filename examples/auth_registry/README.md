# Auth Registry Example

This example demonstrates how a library user can register a custom authentication provider and use it via the public apimigrate API.

What it shows:
- Implementing a simple custom auth provider that returns a header/value pair.
- Registering the provider using apimigrate.RegisterAuthProvider.
- Acquiring auth data using apimigrate.AcquireAuthByProviderSpec.

## Run

```bash
go run ./examples/auth_registry
```

You should see output similar to:

```
registered provider: demo
acquired: header=X-Demo, value=hello, name=my-demo
```
