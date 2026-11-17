# Relay

A desktop HTTP client built with Go and Fyne.

Run with the Go version specified in `go.mod` and the [Fyne desktop prerequisites](https://docs.fyne.io/started/):

```sh
go run ./cmd/relay
```

Choose an HTTP method, enter a URL, and click **Send** (or press Enter in the URL field). URLs without a scheme default to HTTP: `127.0.0.1:8080` becomes `http://127.0.0.1:8080`. Use `https://` explicitly for HTTPS. Supported methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS, CONNECT, and TRACE. Requests default to a 30-second timeout and follow Go's default redirect behavior.

In **Query params**, enter one `key=value` pair per line. Enter values as plain text; Relay handles URL encoding. Repeated names are supported, and parameters are added to any already in the URL:

```text
search=hello world
tag=go
tag=fyne
```

In **Body**, choose JSON, URL-encoded, or multipart Content-Type. JSON remains the default and preserves existing requests; an empty JSON editor sends no body.

The response panel shows the HTTP status, total transfer time, and received body size in bytes (after automatic decompression, excluding headers). View response headers in the **Headers** tab. Switch between **Formatted JSON** and **Raw text**; non-JSON responses display as text in either mode. Copy buttons copy the displayed body or headers. HTTP error responses remain inspectable; validation and network errors appear above the editors.

Run checks:

```sh
go test ./...
go vet ./...
```

## Collections and history

Use **New collection** in the sidebar to create a local collection. Give the current request a name and click **Save request** to add it to the selected collection. Select a saved request to reopen it, edit its name/method/URL/parameters/body, and click **Save request** to update it. Select a different collection before saving to move the request there. **New tab** (or the + button) starts a separate request. Collections can be renamed or deleted; deleting a collection removes its saved requests but keeps history.

The **History** tab records sent requests, timestamps, responses, and network errors, newest first. Select an item to restore its request and response. You can rename or delete history items, edit and resend their requests, or save an edited copy into a collection. Editing a reopened history request preserves the original history entry.

Collections, history (including response bodies and headers), all open request tabs, the selected tab, and each tab’s JSON/raw view preference survive restarts. Draft edits save automatically; collection requests change when you click **Save request**. Requests still running when the app closes appear as interrupted in history on the next start.

Data is stored locally in `relay/workspace.json` inside the OS user configuration directory (`$XDG_CONFIG_HOME` or `~/.config` on Linux). This file contains the full request and response contents. Saves use a private temporary file and an atomic replacement. Storage errors appear in the window; malformed or unsupported workspace files are left untouched instead of being reset.

## Authentication, tabs, and request controls

In **Authentication**, choose **None**, **Basic**, or **Bearer**. Basic Auth sends the username and password in the Authorization header; Bearer sends the entered token. Passwords and tokens are masked in the editor. Authentication settings are preserved in saved requests, history, and tab drafts in the same private local workspace file; this file is not encrypted.

Each request tab has its own editor, response, and running request. You can send requests in different tabs concurrently. Opening a collection request selects its existing tab if it is already open. Opening history creates a new tab. Use the tab’s close button to close it; closing a running tab cancels that request. Closing the last tab creates an empty one.

In **Settings**, enter a timeout such as `30s`, `500ms`, or `2m`. A blank value uses 30 seconds; `0` disables the timeout. The timeout covers connecting, redirects, and reading the response. Click **Cancel** beside Send to stop the current tab’s request. Cancellation and timeout results are recorded in history, with messages explaining common network, hostname, and TLS errors. HTTP error responses remain visible with their status and body.

**Copy response body** copies the active response in the selected JSON/raw format and is enabled after a response arrives.

## Text colors and JSON highlighting

HTTP methods use distinct colors in the editor and sidebar: GET is green, POST amber, PUT blue, PATCH purple, DELETE red, and HEAD/OPTIONS cyan. Response statuses use green for success, cyan for redirects, amber for client errors, and red for server/network errors. Status codes and method names remain visible alongside the colors.

Formatted JSON responses highlight object keys, strings, numbers, booleans, and null values. The JSON editor has **Edit** and **Highlighted preview** tabs. Raw responses, non-JSON bodies, and incomplete JSON previews remain plain text. Header names are highlighted separately from values. The colors adapt to light and dark themes without changing response text or copied contents.

## Large responses and failed transfers

The response view uses a bounded preview (up to 4,096 characters, 120 columns per line, and 60 lines) so minified HTML and large JSON do not freeze the window. The preview explains when it is limited. Large or deeply nested JSON remains raw to avoid expensive formatting. **Copy response body** copies the full retained body, not just the preview. Header previews are bounded too, while **Copy headers** copies all headers.

Responses retain up to 16 MiB of decompressed body data. Transfers exceeding that limit stop with an explicit incomplete-response message. If a read fails, times out, or is cancelled after headers arrive, the received status, headers, and partial body remain available alongside the error. Body size then describes the retained bytes, not the complete server response.

Response bodies are stored once in private, deduplicated files under `relay/bodies/`; the workspace JSON contains references. Draft edits rewrite metadata rather than entire response bodies. Unreferenced body files are removed after a successful save. Existing workspaces migrate automatically to version 2, and missing or damaged body files produce an error without overwriting the workspace. When backing up or moving data, keep `workspace.json` and its `bodies` directory together.

## Environments and variables

Use **New environment** in the top bar to create **Local** with:

```text
base_url=http://localhost:8080
```

Create **Production** with `base_url=https://api.example.com`, then save a request with the URL `{{base_url}}/users`. Choose an environment in the top bar and click **Send** to use that target. **Edit environment** changes its name or variables; **Delete environment** removes it and selects **No environment** if necessary. The active environment applies to all tabs, and environments and selection survive restarts.

Enter one `name=value` per line. Names are case-sensitive, start with an ASCII letter or underscore, and may contain digits, dots, and hyphens. Empty values are allowed; duplicate names are rejected. Use `{{name}}` in URLs, query parameters, JSON bodies, active authentication fields, and timeout settings. For example, use `{{token}}` as a Bearer token or `{"id":{{user_id}}}` for a JSON number. Query-editor values are URL-encoded after substitution.

Substitution happens once when sending and never overwrites the saved template. Values are literal, are not recursively expanded, and are not automatically JSON-escaped: values placed inside JSON strings must already be valid JSON string contents. Undefined variables or malformed references produce an error before a network request is sent. **No environment** works for requests without variables.

History retains the template, environment name, and resolved request values used for that execution. The response identifies the environment it was sent with, even if you switch environments while waiting. Reopening history uses its template with the currently selected environment on the next send. Variable values, including any credentials, are stored in the existing private, unencrypted local workspace.

## Import cURL

Click **Import cURL** in the top bar, paste a single command, and review the method, URL, auth, timeout, and compatibility notes before clicking **Import**. Relay opens a new editable tab and saves its draft; importing does not send the request. Use **Save request** to add it to a collection.

```sh
curl '{{base_url}}/users' \
  --request POST \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer {{token}}' \
  --data-raw '{"name":"Ada"}' \
  --max-time 20
```

Supported input uses POSIX-style single/double quotes and backslash line continuations. Supported options include standard HTTP methods (`-X`/`--request`), positional URLs or `--url`, HEAD (`-I`/`--head`), inline JSON (`-d`/`--data`, `--data-raw`, `--data-binary`, `--json`), URL-encoded forms (`--data`/`--data-urlencode`), multipart fields (`-F`/`--form` and `--form-string`), Basic Auth (`-u`/`--user` or an Authorization header), Bearer Auth (an Authorization header or `--oauth2-bearer`), and total timeout seconds (`-m`/`--max-time`). `--get`/`-G` with inline data or `--data-urlencode name=value` appends query parameters to the URL, preserving repeated names. Relay `{{variables}}` remain intact. Output-only flags `-s`, `-S`, and `-i`, and redirect flag `-L`, are accepted, including `-sSL`.

JSON, URL-encoded, and generated multipart Content-Type plus Basic/Bearer Authorization headers can be represented. Other explicit headers are listed as omitted in the review pane. `--json` also requests an Accept header in cURL; that omission is reported. A JSON body supplied without a Content-Type gets a note explaining that Relay sends it as `application/json` instead of cURL’s default form Content-Type. Relay follows redirects and defaults to a 30-second timeout; the preview calls out these defaults.

File/stdin JSON bodies, cookies, multiple URLs, unsupported methods/options, shell expansions, command substitution, pipelines, and other shell operators are rejected. Multipart `name=@path` references are supported without reading the file during import; stdin, file lists, `name=<file`, and per-part type/filename modifiers remain unsupported. Use separate file fields for multiple uploads. Commands are parsed as text and never executed. Windows cmd/PowerShell quoting and ANSI-C `$'…'` quoting are not supported. Import size is limited to 256 KiB. Imported credentials are persisted under Relay’s existing local storage rules.

## Response variable extraction

Open a request’s **Extract** tab and click **Add rule**. Set a **JSON path** and destination **Variable**, for example `$.access_token` → `token` and `$.user.id` → `user_id`. Rules can be edited, enabled/disabled, or removed, and are saved with request drafts, collection items, and history. **Last run** reports each rule’s success, failure, or skip without displaying extracted values.

Choose an environment and send the request. Relay evaluates rules only after a complete HTTP 2xx response, using the rules and environment captured when sending. Extracted values immediately override that environment’s manually configured variables for the current app session. A subsequent request can use `{{token}}` in Bearer authentication, `{{user_id}}` in a URL or JSON body, and other existing interpolation fields. Custom request headers are still unsupported; authentication headers use the existing auth controls.

Supported paths are `$` (a primitive root), dot properties such as `$.data.user.name`, and zero-based numeric array indexes such as `$.items[0].id`. Property names support ASCII letters, digits, underscores, and hyphens. Filters, wildcards, quoted/bracket property names, and full JSONPath expressions are not supported. No scripting or automatic request chaining is performed.

Strings are extracted unchanged (including empty strings); numbers retain their JSON spelling without floating-point rounding; booleans become `true` or `false`. Null, objects, arrays, missing paths, invalid JSON, and invalid destination names produce a per-rule failure and leave the previous variable value intact. Non-2xx, failed, cancelled, or incomplete transfers are skipped. Other successful rules still apply. With no environment selected, values are not stored and the results explain why.

Session values have precedence over manual environment values, which remain unchanged on disk. **Clear session** clears overrides for the selected environment; restarting clears all overrides. Switching environments preserves each environment’s isolated session values. Renaming an environment preserves its overrides; deleting it clears them, and a pending response cannot recreate a deleted environment’s values. Rules targeting the same variable apply in order, with the last successful rule winning. Concurrent requests update session variables in response-completion order on Fyne’s UI queue; the last applied successful extraction wins.

For privacy, responses for requests with enabled extraction rules are session-only. Requests that actually reference extracted session variables also keep their responses, resolved history requests, and error details off disk. These remain visible while Relay is open; after restarting, templates/rules/history metadata remain, but session-only responses must be fetched again. Extraction result values are never serialized. Existing manual variables, literal credentials, and unrelated responses retain their previous persistence behavior. Copying an extracted value into a literal request field explicitly makes it normal saved request data.

## URL-encoded forms and file uploads

In **Body**, select **application/x-www-form-urlencoded** or **multipart/form-data** and click **Add field**. Set each name and text value; repeated names and empty values are supported. Fields can be disabled or removed. URL-encoded values are escaped automatically, including spaces, Unicode, newlines, and `&`/`=` characters.

For uploads, choose **multipart/form-data**, set a row’s type to **file**, and click **Choose file…**. Linux prefers the operating system’s XDG Desktop Portal file chooser when available; Fyne’s file browser is the fallback and is used on other desktop platforms. You can also enter a local path directly. Add multiple file rows to upload several files, including multiple files under the same field name. File fields require multipart mode; disabled file fields are ignored.

Names, text values, and file paths support `{{variables}}`, including extracted session variables. Inactive JSON/form values and disabled rows are not interpolated. Body types, fields, disabled states, and file paths survive restarts in drafts, collections, and history. File contents are not copied into the workspace; each send reads the current file at that path. Missing, unreadable, or non-regular files produce an upload error.

Relay generates the multipart boundary, filename, and file Content-Type (extension first, content sniffing as a fallback). Files stream from disk with bounded memory; Cancel and the request timeout also cover uploads. Multipart requests can be replayed on 307/308 redirects and stream without a precomputed total length (chunked on HTTP/1.1). APIs requiring a fixed multipart Content-Length are not supported yet. Custom per-file names and MIME overrides are not implemented.

cURL examples:

```sh
curl --data-urlencode 'name=Ada & Grace' --data 'tag=one&tag=two' https://example.com/users
curl -F 'description=Profile photo' -F 'image=@/path/to/photo.png' https://example.com/upload
```

Importing these commands creates editable form fields, never reads file contents, and never sends the request. JSON imports and saved JSON requests continue to work.
