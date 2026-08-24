package activities

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// activityStructs is every struct whose methods are registered as activities by
// registerActivitiesInStruct.
var activityStructs = map[string]any{
	"Audio":      Audio,
	"Video":      Video,
	"Util":       Util,
	"Live":       Live,
	"Vidispine":  Vidispine,
	"Platform":   Platform,
	"Directus":   Directus,
	"ClickUp":    ClickUp,
	"Vizualizer": Vizualizer,
}

// Activities are registered and routed by their short method name, so a name
// used twice is ambiguous in both places at once. The worker runs with
// DisableRegistrationAliasing and the debug queue registers every struct, so a
// collision panics the worker at startup rather than misrouting quietly — which
// is better, but only if it never reaches a worker.
func TestActivityNamesAreUniqueAcrossStructs(t *testing.T) {
	owner := map[string]string{}

	for _, structName := range sortedKeys(activityStructs) {
		for _, method := range GetMethodNames(activityStructs[structName]) {
			previous, taken := owner[method]
			require.False(t, taken,
				"activity %s is defined on both %s and %s", method, previous, structName)
			owner[method] = structName
		}
	}

	assert.Greater(t, len(owner), 100, "expected the full set of activities to be reachable")
}

func TestGetQueueForActivityRoutesByStruct(t *testing.T) {
	assert.Equal(t, environment.GetAudioQueue(), GetQueueForActivity(Audio.TranscodeToAudioWav))
	assert.Equal(t, environment.GetTranscodeQueue(), GetQueueForActivity(Video.TranscodeToProResActivity))
	assert.Equal(t, environment.GetTranscodeQueue(), GetQueueForActivity(Video.CropShortActivity))
	assert.Equal(t, environment.GetLiveIngestQueue(), GetQueueForActivity(Live.StartReaper))

	// Anything not claimed by a specialised queue runs on the worker queue.
	assert.Equal(t, environment.GetWorkerQueue(), GetQueueForActivity(Util.CreateFolder))
	assert.Equal(t, environment.GetWorkerQueue(), GetQueueForActivity("SomethingNobodyDefined"))
}

// ffmpegOnWorkerQueue lists the activities that reach for ffmpeg from a struct
// that does not route to an ffmpeg queue. Every entry is a latent failure: the
// worker image does not install ffmpeg, so the call fails there.
var ffmpegOnWorkerQueue = map[string]string{}

// The routing fallback cannot be anything but silent — a name says nothing
// about what the activity needs — so an ffmpeg activity hung on the wrong
// struct lands on the worker queue and fails there as an opaque timeout. This
// reads the source instead.
func TestEveryFFmpegActivityIsOnAnFFmpegQueue(t *testing.T) {
	found := ffmpegUsingActivityMethods(t)

	require.Contains(t, found, "CropShortActivity",
		"the scan found no known ffmpeg user, so it is not looking at anything")

	for method, receiver := range found {
		if _, allowed := ffmpegOnWorkerQueue[method]; allowed {
			continue
		}
		assert.Contains(t, []string{"AudioActivities", "VideoActivities"}, receiver,
			"%s uses ffmpeg but is defined on %s, so it runs on the worker queue where ffmpeg is not installed",
			method, receiver)
	}
}

// ffmpegUsingActivityMethods returns activity methods whose body references the
// ffmpeg or transcode service packages, keyed by method name and mapped to the
// receiver type they hang off.
func ffmpegUsingActivityMethods(t *testing.T) map[string]string {
	t.Helper()

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	require.NoError(t, err)

	found := map[string]string{}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			mediaPackages := mediaPackageNames(file)
			if len(mediaPackages) == 0 {
				continue
			}

			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || fn.Body == nil || !fn.Name.IsExported() {
					continue
				}

				receiver := receiverTypeName(fn)
				if !strings.HasSuffix(receiver, "Activities") {
					continue
				}

				if usesAnyPackage(fn.Body, mediaPackages) {
					found[fn.Name.Name] = receiver
				}
			}
		}
	}

	return found
}

// mediaPackageNames returns the local names the file imports the ffmpeg and
// transcode service packages under.
func mediaPackageNames(file *ast.File) map[string]bool {
	names := map[string]bool{}

	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path != "github.com/bcc-code/bcc-media-flows/services/ffmpeg" &&
			path != "github.com/bcc-code/bcc-media-flows/services/transcode" {
			continue
		}

		name := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = true
	}

	return names
}

func receiverTypeName(fn *ast.FuncDecl) string {
	switch typ := fn.Recv.List[0].Type.(type) {
	case *ast.Ident:
		return typ.Name
	case *ast.StarExpr:
		if ident, ok := typ.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

func usesAnyPackage(body *ast.BlockStmt, packages map[string]bool) bool {
	used := false

	ast.Inspect(body, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := selector.X.(*ast.Ident); ok && packages[ident.Name] {
			used = true
			return false
		}
		return true
	})

	return used
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Sorted so a duplicate is always reported against the same pair of structs.
	slices.Sort(keys)
	return keys
}
