# Relay

A desktop HTTP client built with Go and Fyne.

Run with the Go version specified in `go.mod` and the [Fyne desktop prerequisites](https://docs.fyne.io/started/):

```sh
go run ./cmd/relay
```

Choose an HTTP method, enter a complete `http://` or `https://` URL, and click **Send** (or press Enter in the URL field). Supported methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS, CONNECT, and TRACE. Requests have a 30-second timeout and follow Go's default redirect behavior.

In **Query params**, enter one `key=value` pair per line. Enter values as plain text; Relay handles URL encoding. Repeated names are supported, and parameters are added to any already in the URL:

```text
search=hello world
tag=go
tag=fyne
```

In **JSON body**, optionally enter valid JSON. Relay sends it with `Content-Type: application/json`. An empty editor sends no body.

The response panel shows the HTTP status, total transfer time, and received body size in bytes (after automatic decompression, excluding headers). View response headers in the **Headers** tab. Switch between **Formatted JSON** and **Raw text**; non-JSON responses display as text in either mode. Copy buttons copy the displayed body or headers. HTTP error responses remain inspectable; validation and network errors appear above the editors.

Run checks:

```sh
go test ./...
go vet ./...
```
