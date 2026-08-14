package ffmpeg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobArguments(t *testing.T) {
	args := Job{
		Input:  "/in/source.mxf",
		Output: "/out/result.wav",
		Args:   []string{"-codec:a", "pcm_s24le"},
	}.Arguments()

	assert.Equal(t, []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", "/in/source.mxf",
		"-codec:a", "pcm_s24le",
		"-y", "/out/result.wav",
	}, args)
}

// The output path has to come last and after -y, and the input before the
// codec arguments: ffmpeg reads options positionally, so an argument's meaning
// depends on which file it precedes.
func TestJobArgumentsWithNoArgs(t *testing.T) {
	args := Job{Input: "/in/a.mov", Output: "/out/b.mov"}.Arguments()

	assert.Equal(t, "-i", args[len(args)-4])
	assert.Equal(t, "/in/a.mov", args[len(args)-3])
	assert.Equal(t, "-y", args[len(args)-2])
	assert.Equal(t, "/out/b.mov", args[len(args)-1])
}

func TestRunRejectsAnIncompleteJob(t *testing.T) {
	_, err := Run(Job{Output: "/out/x.wav"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input")

	_, err = Run(Job{Input: "/in/x.mxf"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no output")
}

// The caller is usually the first thing to put a file in the output directory,
// so Run has to create it.
func TestRunCreatesTheOutputDirectory(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "nested", "deeper", "out.wav")

	// The ffmpeg call fails on a nonexistent input, but only after the directory
	// has been made — which is what this is checking.
	_, err := Run(Job{
		Input:  filepath.Join(root, "missing.mxf"),
		Output: output,
		Info:   &StreamInfo{},
	}, nil)
	require.Error(t, err)

	info, statErr := os.Stat(filepath.Dir(output))
	require.NoError(t, statErr, "the output directory should exist even though the run failed")
	assert.True(t, info.IsDir())
}

func TestRunUsesTheGivenInfoWithoutProbing(t *testing.T) {
	root := t.TempDir()

	// A nonexistent input would fail probing with a different error; passing Info
	// means Run gets as far as trying to run ffmpeg.
	_, err := Run(Job{
		Input:  filepath.Join(root, "missing.mxf"),
		Output: filepath.Join(root, "out.wav"),
		Info:   &StreamInfo{TotalSeconds: 10},
	}, nil)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "ffprobe")
}

func TestJobArgumentsWithExtraInputs(t *testing.T) {
	args := Job{
		Input:       "/in/video.mxf",
		ExtraInputs: FileInputs([]string{"/in/nor.wav", "/in/eng.wav"}),
		Output:      "/out/result.mov",
		Args:        []string{"-map", "1"},
	}.Arguments()

	// The extra inputs sit between the primary input and the arguments, so the
	// stream indices the arguments refer to are the order they were given in.
	assert.Equal(t, []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", "/in/video.mxf",
		"-i", "/in/nor.wav",
		"-i", "/in/eng.wav",
		"-map", "1",
		"-y", "/out/result.mov",
	}, args)
}

// -itsoffset and -f lavfi apply to the input they precede, so they cannot be
// lumped in with the codec arguments.
func TestJobArgumentsWithPerInputOptions(t *testing.T) {
	args := Job{
		InputArgs: []string{"-f", "lavfi"},
		Input:     "color=c=black:s=1920x1080",
		ExtraInputs: []Input{
			{Args: []string{"-itsoffset", "-0.022"}, Path: "/in/nor.wav"},
			{Path: "/in/nor.srt"},
		},
		Output: "/out/muxed.mxf",
		Args:   []string{"-c:v", "copy"},
	}.Arguments()

	assert.Equal(t, []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-f", "lavfi",
		"-i", "color=c=black:s=1920x1080",
		"-itsoffset", "-0.022",
		"-i", "/in/nor.wav",
		"-i", "/in/nor.srt",
		"-c:v", "copy",
		"-y", "/out/muxed.mxf",
	}, args)
}
