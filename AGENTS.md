# Working on Relay

Relay is a desktop HTTP client written in Go with Fyne. Reuse the existing request, workspace, and UI mechanisms before introducing new abstractions or dependencies. Keep changes focused on the requested feature and preserve unrelated behavior.

Use English for UI copy, documentation, and commit messages. Keep the conversation in English unless the user requests otherwise.

## Run and verify

Use the Go version declared in `go.mod`. Desktop builds require Fyne's native prerequisites; see `README.md`.

```sh
go run ./cmd/relay
go build -o /tmp/relay ./cmd/relay
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

Run the package, not `go run ./cmd/relay/main.go`: the entry point depends on other files in that package.

- Format changed Go files with `gofmt`.
- Run focused tests during implementation and the relevant full checks before handing off functional changes. Documentation-only changes need a review and whitespace check, not a desktop rebuild.
- HTTP integration tests use local `httptest` servers and require loopback sockets. If a sandbox blocks them, use the available authorized execution mechanism; do not remove or weaken tests to work around the restriction.
- If the default Go cache is read-only, prefix Go commands with `GOCACHE=/tmp/relay-go-build`.
- For UI changes, inspect the app using dummy data and an isolated display where available. Use a temporary `XDG_CONFIG_HOME` and `XDG_CACHE_HOME`, creating the directories first. On Linux, `FYNE_PLATFORM=x11` allows an Xvfb smoke test without attaching to the user's Wayland session.
- Keep test artifacts and binaries in temporary directories. Never exercise uploads, save dialogs, or migrations against real user files or the user's workspace during automated checks.

## Repository map

| Location | Responsibility |
| --- | --- |
| `cmd/relay/main.go` | App startup, workspace loading, window lifecycle |
| `cmd/relay/tabs.go` | Shared workspace view, independent request editors, tab lifecycle |
| `cmd/relay/editor.go` | Editor state, execution orchestration, response metrics |
| `cmd/relay/body.go`, `file_picker*.go` | Body controls and platform file dialogs |
| `cmd/relay/library.go`, `environments.go` | Collections, history, environment controls |
| `cmd/relay/extract.go` | Extraction rule controls and last-run feedback |
| `cmd/relay/response_*.go` | Response viewer widgets, search navigation, copy/save actions |
| `cmd/relay/colors.go`, `json_highlight.go` | Semantic colors and bounded JSON highlighting |
| `internal/httpclient` | Request models, validation/building, HTTP execution, streaming bodies, interpolation, error messages |
| `internal/workspace` | Persistent collections/history/drafts/environments, body blobs, volatile session variables |
| `internal/extraction` | Restricted JSON-path evaluation, independent of Fyne |
| `internal/curlimport` | Text-only cURL parsing and compatibility notes |
| `internal/responseview` | Content classification, bounded representations, JSON trees, XML formatting, search, exact-byte writing |

Keep processing logic testable without Fyne. Put widget behavior in `cmd/relay`; do not move it into the internal processing packages.

## UI and concurrency

- Follow the existing native Fyne components and semantic colors. Keep response content read-only and legible in light and dark themes. Do not use disabled input fields as response viewers: their text is dimmed.
- Perform network/file transfers and expensive processing outside the UI goroutine. Marshal widget updates through `fyne.Do` or the workspace view's `dispatch` function.
- Preserve request, environment, and rule snapshots taken at Send. Editing a tab or switching environments must not change an in-flight request.
- Keep cancellation independent per tab. Closing a running tab cancels its request without resurrecting the tab when its callback arrives.
- Preserve generation checks and preparation cancellation so stale response/search work cannot overwrite current content. Do not read mutable widget/editor state from workers.
- Register window shortcuts once and route them to the active editor. Preserve Ctrl+F and the macOS Cmd+F equivalent.
- Fyne `widget.Tree` requires its hidden root to be a branch. Preserve that contract when changing tree callbacks; otherwise a populated model can render as an empty tree.

## Request and import behavior

- An empty body type means JSON for compatibility with existing saved requests. Preserve inactive body values when switching formats, but resolve/send only the active body type and enabled form fields.
- Support repeated query/form names and empty values. Resolve variables before encoding form/query values.
- Stream multipart files instead of buffering complete uploads. Preserve cancellation, timeout handling, reader cleanup, generated boundaries, and replay support for 307/308 redirects.
- Persist upload paths, not uploaded file contents. Missing or unreadable files must produce clear errors when sending.
- cURL import parses text only. Never invoke a shell, execute cURL, read an imported file, or send a request as part of import. Reject unsupported transfer options and explicitly disclose omitted headers or changed defaults.

## Environments, extraction, and sensitive data

- Preserve `{{variable}}` templates in editor state, collections, and history. Interpolation is a single literal substitution pass, not scripting or recursive expansion.
- Session extraction values override manual environment values without mutating those manual values. Scope overrides by the stable environment ID captured at Send, not its display name or the environment selected at completion.
- Session variables stay outside persisted `State`. Clearing a session or restarting reveals the original manual defaults. Deleting an environment must prevent pending extractions from recreating its values.
- Run extraction only on complete 2xx JSON responses. Keep per-rule failures separate from HTTP errors and retain previous values on failure. Use `json.Decoder.UseNumber()` to preserve numeric precision.
- Preserve session-only persistence protection: requests with enabled extraction rules, and requests that reference session-derived values, must not write response bodies/headers, resolved credentials, or potentially sensitive error details to disk.
- Extraction result values must not appear in serialized results or diagnostic logs. Explicit Copy/Save actions remain user-directed operations, distinct from automatic workspace persistence.
- Manual variables and literal credentials retain their existing persistence semantics. Do not silently erase or rewrite them.

## Persistence and response integrity

- Use `workspace.DefaultPath()` and the existing store. Metadata lives in the OS user configuration directory under `relay/workspace.json`; response blobs are in the sibling `bodies` directory.
- Preserve atomic metadata replacement, private file permissions, body deduplication, reference validation, and cleanup ordering. Remove unreferenced blobs only after the metadata save succeeds.
- Preserve compatibility with supported workspace versions. Malformed, unsupported, missing-body, or damaged-body data must produce an explicit error rather than silently resetting the workspace.
- Treat retained response bodies as immutable. Pretty, Raw, Tree, search, and image previews must not modify them. Save writes the original retained bytes; Pretty copy may use the formatted representation.
- Raw means the payload available to the HTTP client after transport processing, not guaranteed pre-decompression wire bytes.
- Keep capture and processing limits explicit. Consult `httpclient.MaxResponseBytes` and `internal/responseview` constants rather than duplicating limits across components.
- Do not present incomplete captures as complete files. Preserve partial status/headers/body and clearly label partial save actions and filenames.
- Do not render large text as an unbounded grid or construct oversized JSON trees/images. Preserve bounded paging, worker limits, and stale-result protection.
- HTML, SVG, XML, and PDF inspection must not execute scripts or fetch external resources. Unsupported binaries should show metadata and offer saving, not unreadable text.

## Tests and handoff

Extend existing tests for changed behavior, including compatibility and failure paths where relevant. Prefer local servers, temporary files, and Fyne's test app over external network calls or extensive UI automation. Keep asynchronous tests deterministic using the dispatch queue pattern already used in `cmd/relay` tests.

For changes involving extraction, interpolation, or persistence, verify that rules/templates survive restart while session secrets do not leak into metadata or body files. For response changes, cover original-byte integrity, large inputs, limits, and stale results. For uploads, cover missing files, streaming cancellation, and redirects.

Update `README.md` when behavior or limits change. In the handoff, summarize behavior, checks performed, and material limitations. When commits are requested, group related changes into coherent commits and verify the final working-tree status.
