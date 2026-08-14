package transcode

// EncodeResult names the file an encode produced. Every encoder in this package
// returns it.
type EncodeResult struct {
	Path string
}
