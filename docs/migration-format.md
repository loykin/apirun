# Migration File Format

Complete guide to writing migration files for apirun workflows.

## Overview

Migration files define HTTP API operations in YAML format with:
- **Up migrations**: Operations to apply changes
- **Down migrations**: Operations to rollback changes (optional)
- **Templating**: Dynamic content using Go templates
- **Response processing**: Status validation and data extraction
- **Environment variables**: Variable extraction and reuse

## Basic Structure

```yaml
# 001_example_migration.yaml
up:
  name: "descriptive name"
  env:           # local environment variables
    key: value
  request:       # HTTP request definition
    # request configuration
  response:      # response validation and processing
    # response configuration

down:            # optional rollback operation
  name: "rollback operation"
  # similar structure to up, or simplified format
```

## Up Migration Format

### Complete Up Migration

```yaml
up:
  name: create user account
  env:
    username: demo_user
    email: demo@example.com
    role: standard
  request:
    auth_name: admin_basic     # authentication provider name
    method: POST
    url: "{{.api_base}}/api/v1/users"
    headers:
      - name: Content-Type
        value: application/json
      - name: X-Request-ID
        value: "{{.request_id}}"
    queries:
      - name: notify
        value: "true"
      - name: source
        value: migration
    body: |
      {
        "username": "{{.username}}",
        "email": "{{.email}}",
        "role": "{{.role}}",
        "enabled": true,
        "created_by": "apirun"
      }
    render_body: true          # enable template rendering (default: true)
  response:
    result_code: ["201", "409"] # allowed HTTP status codes
    env_missing: skip          # skip | fail - behavior for missing extractions
    env_from:
      user_id: "id"            # extract user ID from response
      profile_url: "profile.url" # extract nested values
      permissions: "permissions" # extract arrays/objects
```

### Simplified Up Migration

```yaml
up:
  name: simple health check
  request:
    method: GET
    url: "{{.api_base}}/health"
  response:
    result_code: ["200"]
```

## Down Migration Format

### Complete Down Migration

```yaml
down:
  name: delete user account
  auth: admin_basic            # simplified auth reference
  find:                        # optional: find resources before deletion
    request:
      method: GET
      url: "{{.api_base}}/api/v1/users?username={{.username}}&exact=true"
    response:
      result_code: ["200"]
      env_from:
        user_id: "0.id"        # extract from first array element
  method: DELETE               # simplified request format
  url: "{{.api_base}}/api/v1/users/{{.user_id}}"
  headers:
    - name: X-Reason
      value: "Migration rollback"
  response:
    result_code: ["200", "404"] # 404 acceptable if already deleted
```

### Simplified Down Migration

```yaml
down:
  name: remove configuration
  auth: admin_basic
  method: DELETE
  url: "{{.api_base}}/config/{{.config_id}}"
```

## Request Configuration

### HTTP Methods

```yaml
request:
  method: GET     # GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS
  url: "{{.api_base}}/endpoint"
```

### Headers

```yaml
request:
  headers:
    - name: Content-Type
      value: application/json
    - name: Authorization
      value: "Bearer {{.auth.oauth_provider}}"
    - name: X-Custom-Header
      value: "{{.custom_value}}"
    - name: Accept
      value: application/json
```

### Query Parameters

```yaml
request:
  queries:
    - name: limit
      value: "10"
    - name: filter
      value: "{{.filter_criteria}}"
    - name: include
      value: "metadata,permissions"
```

### Request Body

#### JSON Body

```yaml
request:
  body: |
    {
      "name": "{{.resource_name}}",
      "config": {
        "enabled": {{.enabled}},
        "settings": {{.settings | toJson}}
      },
      "metadata": {
        "created_by": "apirun",
        "environment": "{{.environment}}"
      }
    }
```

#### Form Data

```yaml
request:
  headers:
    - name: Content-Type
      value: application/x-www-form-urlencoded
  body: |
    username={{.username}}&password={{.password}}&grant_type=password
```

#### XML Body

```yaml
request:
  headers:
    - name: Content-Type
      value: application/xml
  body: |
    <?xml version="1.0" encoding="UTF-8"?>
    <user>
      <name>{{.username}}</name>
      <email>{{.email}}</email>
      <active>{{.enabled}}</active>
    </user>
```

#### Disable Body Rendering

```yaml
request:
  render_body: false  # send body as-is, no template processing
  body: |
    {"template": "{{not_a_template}}", "literal": "braces"}
```

## Response Processing

### Status Code Validation

```yaml
response:
  result_code: ["200"]           # only 200 accepted
  # result_code: ["200", "201"]  # 200 or 201 accepted
  # result_code: []              # any status code accepted
```

### Data Extraction

#### Simple Extraction

```yaml
response:
  result_code: ["200"]
  env_from:
    id: "id"                     # extract top-level field
    name: "name"
    email: "email"
```

#### Nested Extraction

```yaml
response:
  env_from:
    user_id: "data.user.id"              # nested object
    profile_image: "data.user.profile.avatar_url"
    first_permission: "data.permissions.0"  # first array element
    last_login: "data.metadata.last_login"
```

#### Complex JSON Paths (gjson)

```yaml
response:
  env_from:
    # Array operations
    user_count: "users.#"                    # count of users array
    admin_users: "users.#(role==admin).name" # filter and extract

    # Conditional extraction
    primary_email: "emails.#(primary==true).address"

    # Nested arrays
    all_permissions: "roles.#.permissions.#.name"

    # Object existence
    has_profile: "profile.@this"             # true if profile exists
```

### Extraction Error Handling

```yaml
response:
  result_code: ["200"]
  env_missing: fail              # fail migration if extraction fails
  env_from:
    required_id: "id"            # will fail if 'id' not in response
    optional_field: "optional"   # will fail if 'optional' not found

# Alternative: skip missing extractions
response:
  env_missing: skip              # ignore missing fields (default)
  env_from:
    id: "id"                     # extracted if present
    optional: "maybe_missing"    # ignored if missing
```

## Template System

### Variable Namespaces

```yaml
# Environment variables
url: "{{.env.api_base}}/users"

# Authentication tokens
headers:
  - name: Authorization
    value: "Bearer {{.auth.oauth_provider}}"

# Direct access (legacy, prefer namespaced)
url: "{{.api_base}}/users"
```

### Template Functions

#### Built-in Functions

```yaml
body: |
  {
    "timestamp": "{{.timestamp | default \"unknown\"}}",
    "config": {{.config_object | toJson}},
    "id": "{{.id | upper}}",
    "name": "{{.name | lower}}",
    "count": {{.count | add 1}}
  }
```

#### Common Patterns

```yaml
# Default values
"field": "{{.optional_field | default \"fallback_value\"}}"

# JSON serialization
"object_field": {{.complex_object | toJson}}

# String manipulation
"upper_name": "{{.name | upper}}"
"lower_email": "{{.email | lower}}"

# Arithmetic
"next_id": {{.current_id | add 1}}
"half_count": {{.total_count | div 2}}
```

### Conditional Templates

```yaml
body: |
  {
    "name": "{{.name}}",
    {{if .email}}"email": "{{.email}}",{{end}}
    {{if eq .environment "production"}}"secure": true{{else}}"secure": false{{end}}
  }
```

## Environment Variables

### Local Variables

```yaml
up:
  name: create resource
  env:
    resource_type: user          # local to this migration
    default_role: member
    notification_enabled: true
  request:
    body: |
      {
        "type": "{{.resource_type}}",
        "role": "{{.default_role}}",
        "notify": {{.notification_enabled}}
      }
```

### Variable Extraction and Reuse

```yaml
# First migration: extract variables
up:
  name: create parent resource
  request:
    method: POST
    url: "{{.api_base}}/parents"
    body: |
      {"name": "parent_resource"}
  response:
    result_code: ["201"]
    env_from:
      parent_id: "id"            # extracted and stored
      parent_name: "name"

# Later migration: use extracted variables
---
up:
  name: create child resource
  request:
    method: POST
    url: "{{.api_base}}/parents/{{.parent_id}}/children"
    body: |
      {
        "name": "child_of_{{.parent_name}}",
        "parent_id": "{{.parent_id}}"
      }
```

## Advanced Patterns

### Multi-Step Operations

```yaml
up:
  name: complex user setup
  # Step 1: Create user
  request:
    method: POST
    url: "{{.api_base}}/users"
    body: |
      {"username": "{{.username}}", "email": "{{.email}}"}
  response:
    result_code: ["201"]
    env_from:
      user_id: "id"

# Step 2: Set permissions (separate request in same migration)
- name: assign permissions
  request:
    method: PUT
    url: "{{.api_base}}/users/{{.user_id}}/permissions"
    body: |
      {"permissions": ["read", "write"]}
  response:
    result_code: ["200"]

# Step 3: Send notification
- name: send welcome notification
  request:
    method: POST
    url: "{{.notification_api}}/send"
    body: |
      {
        "user_id": "{{.user_id}}",
        "template": "welcome",
        "email": "{{.email}}"
      }
  response:
    result_code: ["200", "202"]
```

### Conditional Operations

```yaml
up:
  name: environment-specific setup
  env:
    is_production: "{{eq .environment \"production\"}}"
  request:
    method: POST
    url: "{{.api_base}}/config"
    body: |
      {
        "environment": "{{.environment}}",
        {{if .is_production}}
        "security_level": "high",
        "monitoring": true,
        "backup_enabled": true
        {{else}}
        "security_level": "standard",
        "monitoring": false,
        "backup_enabled": false
        {{end}}
      }
```

### Error Recovery Patterns

```yaml
up:
  name: robust resource creation
  request:
    method: POST
    url: "{{.api_base}}/resources"
    body: |
      {"name": "{{.resource_name}}"}
  response:
    result_code: ["201", "409"]    # accept creation or conflict
    env_from:
      resource_id: "id"
      resource_status: "status"

# Handle existing resource
- name: update existing if needed
  request:
    method: PUT
    url: "{{.api_base}}/resources/{{.resource_id}}"
    body: |
      {"status": "updated", "modified_by": "migration"}
  response:
    result_code: ["200", "404"]    # OK if updated or not found
```

## Best Practices

### 1. Descriptive Naming

```yaml
# Good: descriptive names
up:
  name: create user account with default permissions

down:
  name: delete user account and cleanup permissions
```

### 2. Idempotent Operations

```yaml
# Good: handle existing resources
response:
  result_code: ["201", "409"]    # accept creation or conflict

# Good: check before delete
down:
  name: delete user if exists
  auth: admin_basic
  method: DELETE
  url: "{{.api_base}}/users/{{.user_id}}"
  response:
    result_code: ["200", "404"]  # OK if deleted or already gone
```

### 3. Comprehensive Variable Extraction

```yaml
response:
  result_code: ["201"]
  env_from:
    user_id: "id"                # primary identifier
    username: "username"         # for reference
    email: "email"              # for notifications
    profile_url: "profile.url"   # for linking
    created_at: "created_at"     # for auditing
```

### 4. Clear Error Handling

```yaml
response:
  result_code: ["200", "201"]
  env_missing: fail              # explicit about required extractions
  env_from:
    required_id: "id"            # clearly required
    optional_field: "metadata.optional"
```

### 5. Template Safety

```yaml
# Good: use defaults for optional values
body: |
  {
    "name": "{{.name}}",
    "description": "{{.description | default \"No description provided\"}}",
    "tags": {{.tags | default "[]" | toJson}}
  }
```

## File Organization

### Naming Convention

```
001_initial_setup.yaml           # sequential numbering
002_create_admin_user.yaml       # descriptive names
003_configure_permissions.yaml   # clear purpose
004_setup_notifications.yaml     # logical grouping
```

### File Structure

```yaml
# File header comment (optional)
# Purpose: Create initial user accounts
# Dependencies: requires admin API access
# Environment: development, staging, production

up:
  # main operation

down:
  # rollback operation (optional but recommended)
```

## Validation and Testing

### Dry Run Testing

```bash
# Test migration without execution
apirun up --dry-run

# Test specific migration
apirun up --to 3 --dry-run
```

### Status Code Validation

```yaml
# Restrictive: only exact success
response:
  result_code: ["200"]

# Permissive: multiple acceptable outcomes
response:
  result_code: ["200", "201", "409"]

# Wide open: any status (use carefully)
response:
  result_code: []
```

### Template Validation

```yaml
# Test templates with actual values
env:
  test_mode: true
  api_base: "http://localhost:8080"
  username: "test_user"

request:
  url: "{{.api_base}}/users"    # validates to: http://localhost:8080/users
  body: |
    {"username": "{{.username}}"}  # validates to: {"username": "test_user"}
```