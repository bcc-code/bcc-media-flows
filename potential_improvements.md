# Potential improvements

Condensed 2026-08-21. Items confirmed fixed were removed. Bugs section validated against the code 2026-08-21.

## Bugs

- The ~60 `wfutils.SendTelegramText` / `telegram.SendText` call sites hand-build legacy Markdown with `fmt.Sprintf`, interpolating filenames, paths and error strings raw. Any unbalanced `_`, `*`, `` ` `` or `[` in those values makes Telegram reject the message ("can't parse entities"); the sender now retries in plain text, so the alert survives but loses its formatting. The root fix is an escaping send helper (`notifications.escapeMarkdown` is the piece to export or mirror) or a move to MarkdownV2, which can also escape inside code entities.
- QScan's repository id is an unvalidated default (`QSCAN_REPOSITORY_ID`, 2), and nothing in the code can tell a wrong repository from an unreachable one: the workflow sends only a repository-relative path, so the server/share half of the resolved path lives entirely in QScan's config. A `ListRepositories` call in `QScanActivities.ready()` (or a startup check) asserting that repository's root would turn a silent per-file `file_error` into one clear configuration failure.
- `QScanMaster` turns every non-success terminal status into a non-retryable `QScanAnalysisFailed` (`workflows/misc/qscan_master.go:100`). `file_error` is not always about the file: an SMB outage on the QScan host reports the same status, and that transient infrastructure failure becomes a permanent "QC ERROR" alert with no retry. Worth distinguishing `file_error` (retry a few times, widely spaced) from `analysis_error`/`unsupported` (genuinely permanent).
- `notifications.Simple.RenderMarkdown` emits `# Title`, which legacy Markdown has no heading syntax for — Telegram shows the literal `#`. It also forwards `Message` unescaped, which is deliberate for callers that pass their own markup but means `Simple` cannot be used for text from another system.

## Security

- `cmd/httpin/main.go:162` — `ExecuteFFmpeg` trigger gives arbitrary ffmpeg argv (read/write/exfil primitive). Delete or gate it.
- No in-process auth on any route; only CORS middleware. Includes admin and state-changing routes.
- `cmd/httpin/main.go:266` — `cors.Default()` allows all origins; drop or allowlist.
- `cmd/httpin/main.go:269` — workflow triggers reachable over `GET`; no CSRF tokens in any trigger_ui form.
- `cmd/httpin/main.go:30` — `triggeredBy` comes from the request, so the audit trail is caller-controlled.
- `GET /schemas` + `POST /trigger-dynamic` expose and run every triggerable workflow, including destructive scheduled ones.
- `cmd/trigger_ui/masters.go` — form paths not confined to `MASTER_TRIGGER_DIR`; watermark paths unvalidated.
- `services/vidispine/vsapi/xml_templates.go` — metadata interpolated into XML via `text/template`, no escaping. Use `encoding/xml`.
- `services/clickup/client.go:27` — hardcoded token in source; other hardcoded internal endpoints (rclone, reaper, baton, emails). rclone sends Basic auth over plain HTTP.
- `cmd/trigger_ui/templates/*` — 13 templates load Tailwind from a CDN, no SRI/CSP. Vendor the CSS.
- gin runs in debug mode in production (no `SetMode` outside tests); no server timeouts, body size limits, or graceful shutdown.
- `cmd/httpin/watchers.go` — hardcoded `/mnt/...` paths bypass the mount-prefix overrides; unknown paths fall through to transcode; fixed `LIVE-INGEST` workflow ID collides on concurrent events.
- `services/transcode/multitrack.go:44` — filenames interpolated unescaped into a `drawtext` filter (ffmpeg filter-grammar injection). Same class in `merge.go` concat lists.
- `worker.Dockerfile` — runs as root (other images set `USER nonroot`), unpinned `alpine:latest`, no ffmpeg (audio/video queues panic in that image).

## CI and tooling

- No CI runs `go test`, `go vet`, `workflowcheck`, or a linter; no `pull_request` trigger. `make test` exists and nothing calls it.
- `workflowcheck` is not pinned; add it as a `go.mod` tool directive.
- No `go mod tidy` check, `govulncheck`, image scanning, or `.dockerignore` (Dockerfiles `COPY . .`).
- golangci-lint (default set) reports ~89 issues: errcheck, staticcheck, ineffassign, unused.
- Low or zero test coverage: `workflows/export`, `activities`, `services/ffmpeg`, and no tests for `directus`, `baton`, `ftp`, `subtrans`, `filecatalyst`.
- Only `vidispine.Client` has an interface + mock; other service clients are concrete structs and untestable without network.
- `services/transcode/testdata/generated/` is partly committed; tests should use `t.TempDir()`.
- Test helpers in `utils/testutils` panic instead of `t.Fatal`/`t.Skip`.
- `services/transcode` tests fail on a current ffmpeg (7.x): `-vsync` was removed and the ProRes `top` AVOption is no longer an encoder option, so `Test_H264Video_WeirdResolutions` and `Test_ProResHyperdeck` are red locally. Either pin the ffmpeg version the tests expect or move to `-fps_mode` / drop `top`.

## Simplification and cleanup

- `workflows/vb_export/` — `bstage` and `gfx` children are near-identical; the same preamble/postamble repeats at ~10 sites. Extract one shared child wrapper. Also: half the children omit `VBExportResult.Title`; `abekas`/`hyperdeck` re-run `AnalyzeFile` although the result is already passed in.
- `common/merge.go` imports `services/vidispine`, inverting the layering; move `AudioStream` down.
- Enum stragglers: watch-folder names, Vidispine job states, shape tags, shorts type/status, `Destinations []string` where enum types exist. (Watch-folder names need a history migration first.)
- JSON tags on workflow payload structs are inconsistent (camelCase vs snake_case vs none); settle one convention — the choice is permanent for replay.
- `interface{}`/`any` where types would do; ~12 activities return `(any, error)` to satisfy `Execute`.
- Dead code: `cmd/fakerclone/`, `vsapi.ListFilesForStorage` + `ListFilesFilter`, assorted unused helpers flagged by `unused`.
- Long functions (150–280 lines) in `incremental_ingest`, `vx_export`, `vx_export_vod`, `generate_short`, `masv_import` decompose naturally.
- `activities/shorts.go` and `activities/reaper.go` still build ad-hoc resty clients outside `internal/httpx`.
- `emails.Message` carries `CC`/`BCC` that `UtilActivities.SendEmail` never reads; every recipient gets a separate `To`-only mail. Either honour them or drop the fields.
- Typos in exported names: `EmtpySRTFile`, `PlacholderTplData`, `SubSteams`, `BmmTargetEnvionment`, `sanitizeDuplicatdPath`.
- go.mod: two cache libraries, `golang-set` used once next to `lo`, `shortid` overlaps `uuid`, `go-spew` pinned to a pseudo-version, stranded direct deps in `// indirect` blocks.
- Two stray SQLite databases in `cmd/trigger_ui/` (ignored, but invite dev misuse).

## Config

- 55+ env variables read all over; almost none validated at startup, so missing secrets fail late and opaquely. One `Config` struct with required fields, loaded per `main`.
- `godotenv` loads in only 2 of 4 entrypoints; the `.env` files for `httpin` and `trigger_ui` are dead.
- Env read at package-var/`init()` time in several files, so `.env` loading and `t.Setenv` cannot affect them; `vb_export.go` case is a replay hazard.
- `.env.example` files drift from the code (wrong variable names, wrong defaults, missing files); `cmd/worker/readme.md` documents 4 of 6 queues.
- Mount-prefix getters in `environment/` still hand-roll the default-fallback pattern; defaults belong in `Config`.
