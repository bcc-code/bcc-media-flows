package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// OutputFileMode and OutputDirMode are what transcode output is left as.
	//
	// Group-writable rather than world-writable: these land on a shared Isilon
	// mount where 0777 masters can be modified by anything that can reach the
	// share.
	OutputFileMode os.FileMode = 0664
	OutputDirMode  os.FileMode = 0775
)

// Job is a single-input, single-output ffmpeg run.
//
// Args carries only the codec, filter and metadata arguments. Run supplies the
// rest of the command line, so the pieces every wrapper was repeating —
// progress reporting, the input flag, overwrite, the output path — are in one
// place and in one order.
type Job struct {
	// Input is the file to read. Required.
	Input string

	// ExtraInputs are further files to read, in order, after Input. Their
	// stream indices start at 1.
	ExtraInputs []string

	// Output is the file to write. Its directory is created if missing.
	Output string

	// Args are the arguments between the input and the output.
	Args []string

	// Info is the probe result used to turn ffmpeg's progress output into a
	// percentage. Run probes Input when it is nil.
	Info *StreamInfo

	// FileMode is what Output is chmodded to. Zero means OutputFileMode.
	FileMode os.FileMode
}

// Run assembles and executes the job, and returns the stream info it probed or
// was given.
//
// The output directory is created first. Writing to a directory that does not
// exist is the single most common way for one of these to fail, because the
// caller is usually the first thing to put a file there.
func Run(job Job, cb ProgressCallback) (StreamInfo, error) {
	if job.Input == "" {
		return StreamInfo{}, fmt.Errorf("ffmpeg job has no input")
	}
	if job.Output == "" {
		return StreamInfo{}, fmt.Errorf("ffmpeg job for %s has no output", job.Input)
	}

	info := StreamInfo{}
	if job.Info != nil {
		info = *job.Info
	} else {
		probed, err := GetStreamInfo(job.Input)
		if err != nil {
			return StreamInfo{}, err
		}
		info = probed
	}

	if err := os.MkdirAll(filepath.Dir(job.Output), OutputDirMode); err != nil {
		return info, err
	}

	if _, err := Do(job.Arguments(), info, cb); err != nil {
		return info, err
	}

	mode := job.FileMode
	if mode == 0 {
		mode = OutputFileMode
	}
	if err := os.Chmod(job.Output, mode); err != nil {
		return info, err
	}

	return info, nil
}

// Arguments returns the full ffmpeg command line for the job.
func (j Job) Arguments() []string {
	args := make([]string, 0, len(j.Args)+2*len(j.ExtraInputs)+8)
	args = append(args,
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", j.Input,
	)
	for _, input := range j.ExtraInputs {
		args = append(args, "-i", input)
	}
	args = append(args, j.Args...)
	return append(args, "-y", j.Output)
}
