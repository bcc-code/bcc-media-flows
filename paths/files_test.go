package paths

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// permutations returns every ordering of the given files.
func permutations(files Files) []Files {
	if len(files) <= 1 {
		return []Files{files}
	}

	var out []Files
	for i := range files {
		rest := make(Files, 0, len(files)-1)
		rest = append(rest, files[:i]...)
		rest = append(rest, files[i+1:]...)

		for _, p := range permutations(rest) {
			perm := append(Files{files[i]}, p...)
			out = append(out, perm)
		}
	}
	return out
}

// The exact pair the old comparison got wrong: swapping drive and path order made
// both directions report "less than", which is what left sort.Sort free to return
// anything.
func TestFilesLess_IsAntisymmetric_OnTheBrokenPair(t *testing.T) {
	f := Files{
		{Drive: Drive{Value: "b"}, Path: "a"},
		{Drive: Drive{Value: "a"}, Path: "b"},
	}

	assert.False(t, f.Less(0, 1) && f.Less(1, 0),
		"Less must not report both directions as less-than")
}

func TestFilesLess_IsAntisymmetric_OverAllPairs(t *testing.T) {
	f := Files{
		{Drive: IsilonDrive, Path: "b/1.wav"},
		{Drive: IsilonDrive, Path: "a/2.wav"},
		{Drive: TempDrive, Path: "a/1.wav"},
		{Drive: TempDrive, Path: "b/2.wav"},
		{Drive: BrunstadDrive, Path: "a/1.wav"},
		{Drive: IsilonDrive, Path: "a/1.wav"},
	}

	for i := range f {
		for j := range f {
			if i == j {
				assert.False(t, f.Less(i, j), "an element cannot be less than itself")
				continue
			}
			assert.False(t, f.Less(i, j) && f.Less(j, i),
				"pair (%d,%d) reports less-than in both directions", i, j)
		}
	}
}

func TestFilesLess_OrdersByDriveThenPath(t *testing.T) {
	f := Files{
		{Drive: TempDrive, Path: "b.wav"},
		{Drive: IsilonDrive, Path: "b.wav"},
		{Drive: TempDrive, Path: "a.wav"},
		{Drive: IsilonDrive, Path: "a.wav"},
	}

	sort.Sort(f)

	assert.Equal(t, Files{
		{Drive: IsilonDrive, Path: "a.wav"},
		{Drive: IsilonDrive, Path: "b.wav"},
		{Drive: TempDrive, Path: "a.wav"},
		{Drive: TempDrive, Path: "b.wav"},
	}, f)
}

// The property that actually matters for Temporal replay: the sorted sequence is a
// function of the set, not of the order the elements happened to arrive in.
//
// The data deliberately puts drive order and path order in conflict — drives sort
// brunstad < isilon < temp, while the paths run the other way — because that is
// what makes the old comparison report both directions as less-than and leaves the
// output dependent on the input order.
func TestFilesSort_IsOrderIndependent(t *testing.T) {
	files := Files{
		{Drive: TempDrive, Path: "a.wav"},
		{Drive: IsilonDrive, Path: "b.wav"},
		{Drive: BrunstadDrive, Path: "c.wav"},
		{Drive: TempDrive, Path: "b.wav"},
		{Drive: IsilonDrive, Path: "c.wav"},
		{Drive: BrunstadDrive, Path: "d.wav"},
	}

	perms := permutations(files)
	require.Len(t, perms, 720, "6 elements should give 6! orderings")

	var want Files
	for i, perm := range perms {
		got := make(Files, len(perm))
		copy(got, perm)
		sort.Sort(got)

		if i == 0 {
			want = got
			continue
		}
		assert.Equal(t, want, got,
			"permutation %d sorted differently; the ordering is not well defined", i)
	}
}

// The only place Files is sorted today is multitrack channel ordering, and there
// every element shares a drive: workflows/ingest/multitrack.go sorts one
// directory's listing, then the per-channel outputs that SplitAudioChannels all
// writes into tempDir. With one drive the broken clause was always false and the
// old expression collapsed to a plain path comparison, which is valid — which is
// why this defect stayed latent rather than corrupting channel order. This test
// pins the realistic single-drive case; the cross-drive cases above cover the
// comparison itself.
func TestFilesSort_MultitrackChannelOrderIsStable(t *testing.T) {
	channels := Files{
		{Drive: TempDrive, Path: "track2_ch2.wav"},
		{Drive: TempDrive, Path: "track1_ch1.wav"},
		{Drive: TempDrive, Path: "track2_ch1.wav"},
		{Drive: TempDrive, Path: "track1_ch2.wav"},
	}

	first := make(Files, len(channels))
	copy(first, channels)
	sort.Sort(first)

	assert.Equal(t, Files{
		{Drive: TempDrive, Path: "track1_ch1.wav"},
		{Drive: TempDrive, Path: "track1_ch2.wav"},
		{Drive: TempDrive, Path: "track2_ch1.wav"},
		{Drive: TempDrive, Path: "track2_ch2.wav"},
	}, first)

	// Sorting an already-sorted slice must not move anything.
	second := make(Files, len(first))
	copy(second, first)
	sort.Sort(second)
	assert.Equal(t, first, second, "sorting is not idempotent")
}

func TestFilesSort_HandlesEmptyAndSingle(t *testing.T) {
	var empty Files
	sort.Sort(empty)
	assert.Empty(t, empty)

	single := Files{{Drive: IsilonDrive, Path: "only.wav"}}
	sort.Sort(single)
	assert.Equal(t, Files{{Drive: IsilonDrive, Path: "only.wav"}}, single)
}
