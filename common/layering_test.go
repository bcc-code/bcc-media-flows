package common_test

import (
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCommonDoesNotImportServices pins the layering. common holds the types the
// service clients exchange, so it sits below them: services/transcode, services/ffmpeg,
// activities and every workflow package import common, and an import back into
// services/ puts a service client behind all of them. It was services/vidispine, for
// one two-field struct.
func TestCommonDoesNotImportServices(t *testing.T) {
	packages, err := parser.ParseDir(token.NewFileSet(), ".", nil, parser.ImportsOnly)
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	for name, pkg := range packages {
		for path, file := range pkg.Files {
			for _, spec := range file.Imports {
				imported, err := strconv.Unquote(spec.Path.Value)
				require.NoError(t, err)

				require.NotContains(t, imported, "bcc-media-flows/services/",
					"%s (package %s) imports %s", path, name, imported)
			}
		}
	}
}
