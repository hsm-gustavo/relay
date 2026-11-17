# Relay

A desktop HTTP client built with Go and Fyne.

Run with the Go version specified in `go.mod` and the [Fyne desktop prerequisites](https://docs.fyne.io/started/):

```sh
go run ./cmd/relay
```

Choose an HTTP method, enter a URL, and click **Send** (or press Enter in the URL field). URLs without a scheme default to HTTP: `127.0.0.1:8080` becomes `http://127.0.0.1:8080`. Use `https://` explicitly for HTTPS. Supported methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS, CONNECT, and TRACE. Requests have a 30-second timeout and follow Go's default redirect behavior.

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

## Collections and history

Use **New collection** in the sidebar to create a local collection. Give the current request a name and click **Save request** to add it to the selected collection. Select a saved request to reopen it, edit its name/method/URL/parameters/body, and click **Save request** to update it. Select a different collection before saving to move the request there. **New request** starts a separate request. Collections can be renamed or deleted; deleting a collection removes its saved requests but keeps history.

The **History** tab records sent requests, timestamps, responses, and network errors, newest first. Select an item to restore its request and response. You can rename or delete history items, edit and resend their requests, or save an edited copy into a collection. Editing a reopened history request preserves the original history entry.

Collections, history (including response bodies and headers), the current editor draft, and the JSON/raw view preference survive restarts. Draft edits save automatically; collection requests change when you click **Save request**. Requests still running when the app closes appear as interrupted in history on the next start.

Data is stored locally in `relay/workspace.json` inside the OS user configuration directory (`$XDG_CONFIG_HOME` or `~/.config` on Linux). This file contains the full request and response contents. Saves use a private temporary file and an atomic replacement. Storage errors appear in the window; malformed or unsupported workspace files are left untouched instead of being reset.
