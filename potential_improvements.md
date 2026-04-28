# Potential improvements

- `services/transcode/preview.go::GrowingPreview` has the same `trc:reserved` swscaler-failure exposure that was just fixed in `Preview` and `VideoH264`. The function reads from a pipe (`pipe:0`) so it cannot probe upfront. Options to consider: probe `input.FilePath` once before starting (race-y while the file is still growing), unconditionally insert `setparams=color_trc=bt709` since the workflow is SDR-only, or accept the trc explicitly from the caller.
