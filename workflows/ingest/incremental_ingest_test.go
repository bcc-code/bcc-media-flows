package ingestworkflows

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateRollingTransferRate_Basic(t *testing.T) {
	now := time.Date(2025, 7, 4, 10, 0, 0, 0, time.UTC)
	window := 5 * time.Minute

	samples := []transferSample{
		{time: now.Add(-6 * time.Minute), bytes: 0},
		{time: now.Add(-4 * time.Minute), bytes: 4_000_000},
		{time: now.Add(-2 * time.Minute), bytes: 8_000_000},
		{time: now.Add(-1 * time.Minute), bytes: 12_000_000},
		{time: now, bytes: 16_000_000},
	}

	rate, _ := CalculateRollingTransferRate(samples, now, window)
	// (16_000_000 - 4_000_000) bytes over 4 minutes = 12_000_000 bytes in 240s
	// 12_000_000 * 8 / 240 / 1_000_000 = 0.4 Mbps
	assert.InDelta(t, 0.4, rate, 0.01)
}

func TestCalculateRollingTransferRate_AtLeastFourSamplesAreKept(t *testing.T) {
	now := time.Date(2025, 7, 4, 10, 0, 0, 0, time.UTC)
	window := 5 * time.Minute

	samples := []transferSample{
		{time: now.Add(-40 * time.Minute), bytes: 0},
		{time: now.Add(-30 * time.Minute), bytes: 1000},
		{time: now.Add(-20 * time.Minute), bytes: 2000},
		{time: now.Add(-10 * time.Minute), bytes: 3000},
		{time: now, bytes: 4000},
	}

	_, pruned := CalculateRollingTransferRate(samples, now, window)
	assert.GreaterOrEqual(t, len(pruned), 1)
	assert.LessOrEqual(t, len(pruned), 4)
	assert.Equal(t, samples[len(samples)-len(pruned):], pruned)
}

func TestCalculateRollingTransferRate_PrunesSamples(t *testing.T) {
	now := time.Date(2025, 7, 4, 10, 0, 0, 0, time.UTC)
	window := 5 * time.Minute

	samples := []transferSample{
		{time: now.Add(-40 * time.Minute), bytes: 0},
		{time: now.Add(-30 * time.Minute), bytes: 1000},
		{time: now.Add(-20 * time.Minute), bytes: 2000},
		{time: now.Add(-10 * time.Minute), bytes: 3000},
		{time: now, bytes: 4000},
	}

	_, pruned := CalculateRollingTransferRate(samples, now, window)
	// Only the last sample is within the window, so pruned should be last 4 samples
	assert.GreaterOrEqual(t, len(pruned), 1)
	assert.LessOrEqual(t, len(pruned), 4)
	assert.Equal(t, samples[len(samples)-len(pruned):], pruned)
}

func TestCalculateRollingTransferRate_TooFewSamples(t *testing.T) {
	now := time.Now()
	window := 5 * time.Minute
	// Only one sample
	samples := []transferSample{{time: now, bytes: 1000}}
	_, pruned := CalculateRollingTransferRate(samples, now, window)
	assert.Equal(t, 1, len(pruned))
}

func TestCalculateRollingTransferRate_ZeroTimeDelta(t *testing.T) {
	now := time.Now()
	window := 5 * time.Minute
	samples := []transferSample{
		{time: now, bytes: 1000},
		{time: now, bytes: 2000},
	}
	_, pruned := CalculateRollingTransferRate(samples, now, window)
	assert.Equal(t, 2, len(pruned))
}
