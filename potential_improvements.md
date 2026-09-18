# Potential improvements

Revalidated against the code 2026-09-15. Items confirmed fixed were removed; the rest were checked to still apply.

## Bugs

- The ~60 `wfutils.SendTelegramText` / `telegram.SendText` call sites hand-build legacy Markdown with `fmt.Sprintf`, interpolating filenames, paths and error strings raw. Any unbalanced `_`, `*`, `` ` `` or `[` in those values makes Telegram reject the message ("can't parse entities"); the sender now retries in plain text, so the alert survives but loses its formatting. The root fix is an escaping send helper (`notifications.escapeMarkdown` is the piece to export or mirror) or a move to MarkdownV2, which can also escape inside code entities.
- QScan's repository id is an unvalidated default (`QSCAN_REPOSITORY_ID`, 2), and nothing in the code can tell a wrong repository from an unreachable one: the workflow sends only a repository-relative path, so the server/share half of the resolved path lives entirely in QScan's config. A `ListRepositories` call in `QScanActivities.ready()` (or a startup check) asserting that repository's root would turn a silent per-file `file_error` into one clear configuration failure.
- `runQScanFile` (`workflows/misc/qscan_master.go`, shared by `QScanMaster` and the `QScanFile` children of `QScanRawImport`) turns every non-success terminal status into a non-retryable `QScanAnalysisFailed`. `file_error` is not always about the file: an SMB outage on the QScan host reports the same status, and that transient infrastructure failure becomes a permanent "QC ERROR" alert with no retry. Worth distinguishing `file_error` (retry a few times, widely spaced) from `analysis_error`/`unsupported` (genuinely permanent).
- `notifications.Simple.RenderMarkdown` emits `# Title`, which legacy Markdown has no heading syntax for — Telegram shows the literal `#`. It also forwards `Message` unescaped, which is deliberate for callers that pass their own markup but means `Simple` cannot be used for text from another system.
- `RawMaterialForm` (`workflows/ingest/raw_material.go`) has had no production caller since commit 0ac6d6c removed the XML order form. The watcher path (`cmd/httpin/watchers.go` `doRawImport`) starts one `RawMaterial` per file event with no metadata, so files uploaded together land in separate runs and output folders, get separate QC mails at best, and carry no uploader address at all. A JSON sidecar form for raw material (like `jsonFormSpecs` has for masters) would restore batching and the uploader.
- `utils.IsMedia` (`utils/files.go`) lists only `.mxf`, `.mov` and `.wav`, so an `.mp4` raw upload is imported but gets no ffprobe analysis, thumbnails, previews, transcription or QC.

## Security

- `cmd/httpin/main.go:162` — `ExecuteFFmpeg` trigger gives arbitrary ffmpeg argv (read/write/exfil primitive). Delete or gate it.
- No in-process auth on any httpin route; only CORS middleware. Includes state-changing routes.
- `cmd/httpin/main.go:266` — `cors.Default()` allows all origins; drop or allowlist.
- `cmd/httpin/main.go:269` — workflow triggers reachable over `GET`; no CSRF tokens in any trigger_ui form.
- `cmd/httpin/main.go:30` — `triggeredBy` comes from the request, so the audit trail is caller-controlled.
- `cmd/trigger_ui/masters.go:157` and `cmd/trigger_ui/main.go:67` — form paths are joined under the configured master-trigger and overlay roots, but `..` segments are not rejected, so a crafted value still escapes the root.
- `services/vidispine/vsapi/xml_templates.go` — metadata interpolated into XML via `text/template`, no escaping. Use `encoding/xml`.
- `services/rclone/upload.go:15` — hardcoded internal endpoint, and rclone sends Basic auth over plain HTTP. (The ClickUp token is a public view share token by design, not a secret.)
- `cmd/trigger_ui/templates/*` — 13 templates load Tailwind from a CDN, no SRI/CSP. Vendor the CSS.
- gin runs in debug mode in production (no `SetMode` outside tests); `bootstrap.Serve` (`internal/bootstrap/bootstrap.go`) uses `router.Run`, so there are no server timeouts, body size limits, or graceful shutdown.
- `cmd/httpin/watchers.go` — hardcoded `/mnt/...` paths bypass the mount-prefix overrides; unknown paths fall through to transcode; fixed `LIVE-INGEST` workflow ID collides on concurrent events.
- `services/transcode/multitrack.go:49` — filenames interpolated unescaped into a `drawtext` filter (ffmpeg filter-grammar injection).
- `worker.Dockerfile` — runs as root (the httpin and trigger_ui images set `USER`), unpinned `alpine:latest`, no ffmpeg (audio/video queues panic in that image).

## CI and tooling

- CI (`.github/workflows/ci.yml`) runs vet, tests, workflowcheck and golangci-lint, but golangci-lint uses `only-new-issues`, so the pre-existing backlog (errcheck, staticcheck, ineffassign, unused) stays. Burn it down and drop the grandfathering.
- No image scanning in `deploy-images.yml` (a Trivy step was drafted and dropped; revisit once base-image noise is acceptable).
- Low or zero test coverage: `workflows/export`, `activities`, `services/ffmpeg`, and no tests at all for `services/baton`, `services/ftp`, `services/filecatalyst`.
- Only `vidispine.Client` has an interface + mock; the other service clients expose a `Config` interface but are concrete structs, untestable without network.
- Test helpers in `utils/testutils` (`audio.go`, `video.go`) panic instead of `t.Fatal`/`t.Skip`, so a machine without ffmpeg sees the ingest and activities suites panic rather than skip.
- `services/transcode` tests were seen failing on ffmpeg 7.x (`-vsync` removed, ProRes `top` AVOption gone) in `Test_H264Video_WeirdResolutions` and `Test_ProResHyperdeck`. CI installs Ubuntu's ffmpeg; not re-verified locally (no ffmpeg installed). Either pin the ffmpeg version the tests expect or move to `-fps_mode` / drop `top`.

## Simplification and cleanup

- `workflows/vb_export/` — `abekas`, `hyperdeck` re-run `AnalyzeFile` although `VBExport` (`vb_export.go:210`) already analysed the input and could pass the result down.
- Enum stragglers, second round (the first round is done: watch folders, Vidispine job states and shape tags, Directus short status / media item type / image style, `VBExportParams.Destinations`; all encode as bare strings via `internal/enumjson`, so no history migration was needed): `VXExportParams.Destinations []string` (`vx_export.go:42`) although `AssetExportDestinations` exists; `MoveMBFileParams.Shapes []string` and `VBExportParams.SubtitleShapeTag string` could be `vsapi.ShapeTag`; `languages.MBPreviewTag string` is a shape tag too; Cantemo task states `"STARTED"`/`"SUCCESS"` in `activities/cantemo/files.go:54`; `paths.Drive` hand-rolls the JSON methods `internal/enumjson` now provides.
- JSON tags on workflow payload structs are inconsistent (~200 snake_case vs ~250 camelCase in `workflows/`); settle one convention — the choice is permanent for replay.
- `interface{}`/`any` where types would do; 15 activities return `(any, error)` to satisfy `Execute`.
- Dead code: `cmd/fakerclone/` (unreferenced), `vsapi.ListFilesForStorage` + `ListFilesFilter` (defined, never called), assorted unused helpers flagged by `unused`.
- Long functions: `doIncremental` (`incremental_ingest.go`), `VXExport` (`vx_export.go`), `GenerateShort` (`generate_short.go`) are each well over 120 lines and decompose naturally.
- `emails.Message` carries `CC`/`BCC` that `UtilActivities.SendEmail` never reads; every recipient gets a separate `To`-only mail. Either honour them or drop the fields.

## Config

- `cmd/httpin/watchers.go:128` — the transcode-root regexp is a package-level var built from `environment.Get()`, so it is evaluated before `bootstrap.LoadEnv` runs in `main` and ignores a `.env` override.
- `cmd/worker/readme.md` documents 4 of the 6 queues in `environment/queues.go` (missing `live-ingest` and `debug`).
- Mount-prefix getters in `environment/environment.go` still hand-roll the default-fallback pattern; defaults belong in `Config`.

## Demux check (added 2026-09-17)

- `services/ffmpeg/demuxcheck.go` runs `-c copy -f null -`, so it demuxes and parses but never decodes. Corruption inside a frame that leaves the container and NAL/KLV structure intact (broken ProRes slices, damaged DCT blocks) passes. A `Decode` option on the activity that drops `-c copy` would catch it, at the cost of a full decode per file; worth offering for masters at least.
- The verdict hinges on ffmpeg's log severities plus a regexp (`demuxCorruptionWarning`) that promotes "corrupt", "sync lost", "partial file" and "truncat" warnings to errors. Any other demuxer warning that means damage still yields PASSED WITH WARNINGS. Review real-world reports for a while and extend the list.
- The check runs on every ingest site except `Incremental` (growing live files, where a full read is meaningless) and the derived imports (previews, subtitles, transcriptions). The raw-material path filters with `utils.IsMedia`, so its `.mp4` gap (see Bugs) applies here too.
- `demuxCheckBeforeImport` (`workflows/ingest/demux_check.go`) never blocks the import. A `FailOnErrors` switch, or routing FAILED verdicts to a holding folder instead of Mediabanken, is the natural next step once the reports have earned trust.
- `ffmpeg.ProbeResultToInfo` (`services/ffmpeg/progress.go`) indexes `info.Streams[0]` without checking, so a probe of a file with zero streams panics the activity. `DemuxCheck` guards it locally; `GetStreamInfo` and every caller of it do not.
- `BmmIngestUpload` mails `params.UploadedBy` as if it were an address; the demux check filters non-addresses out (`emailRecipients`), but the "BMM Upload successful" mail at the end of the workflow does not, and SendGrid rejects a bare name.
