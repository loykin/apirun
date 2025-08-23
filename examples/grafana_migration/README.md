# Grafana Migration Example

This example shows how to:
- Create a Grafana user using the admin API
- Import a dashboard by loading the HTTP request body from a file

Requirements:
- Grafana running at http://localhost:3000
- Admin credentials: admin / admin
- Run commands from the repository root so relative file paths resolve correctly

How to run:
1. Start Grafana locally on port 3000.
2. From the repository root, run:
   apimigrate --config examples/grafana-migration/config.yaml up

Notes:
- The dashboard JSON body is loaded from a file using the new `body_file` field.
- The dashboard sample file is located at:
  examples/grafana-migration/artifact/001_dashboard.json
