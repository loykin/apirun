package migration

// ctxKey is an unexported custom type to avoid context key collisions (SA1029).
type ctxKey string

// SaveResponseBodyKey is a typed context key to toggle storing response bodies in the migration history.
var SaveResponseBodyKey ctxKey = "apimigrate.save_response_body"
