# Auth Registry Example

This example demonstrates how a library user can register a custom authentication provider and use it via the public apimigrate API.

What it shows:
- Implementing a simple custom auth provider that returns a token value.
- Registering the provider using apimigrate.RegisterAuthProvider.
- Acquiring auth data using apimigrate.AcquireAuthByProviderSpecWithName and storing it under a logical name.

## Run

```bash
go run ./examples/auth_registry
```

You should see output similar to:

```
registered provider: demo
acquired: value=hello, name=my-demo
```
