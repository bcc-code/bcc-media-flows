package activities

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"

	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

type CropShortInput struct {
	InputVideoPath  paths.Path
	OutputVideoPath paths.Path
	SubtitlePath    *paths.Path
	KeyFrames       []Keyframe
	InSeconds       float64
	OutSeconds      float64
}

type CropShortResult struct {
	Arguments []string
}

func (ua UtilActivities) CropShortActivity(ctx context.Context, params CropShortInput) (*CropShortResult, error) {
	cropFilter := buildCropFilter(params.KeyFrames)

	info, err := ffmpeg.GetStreamInfo(params.InputVideoPath.Local())
	rate := 25
	if err == nil && info.FrameRate > 40 {
		rate = 50
	}

	// Build filter: crop, then optional subtitle burn-in, then label as [v]
	filter := fmt.Sprintf("[0:v]%s", cropFilter)
	if params.SubtitlePath != nil {
		filter += ",subtitles=" + params.SubtitlePath.Local()
	}
	filter += "[v]"

	args := []string{
		"-i", params.InputVideoPath.Local(),
		"-progress", "pipe:1",
		"-hide_banner",
		"-strict", "unofficial",
		"-filter_complex",
		filter,
		"-map", "[v]",
		"-c:v", "prores",
		"-profile:v", "3",
		"-vendor", "ap10",
		"-bits_per_mb", "8000",
		"-r", strconv.Itoa(rate),
		"-pix_fmt", "yuv422p10le",
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-colorspace", "bt709",
		"-y",
		params.OutputVideoPath.Local(),
	}
	return &CropShortResult{Arguments: args}, nil
}

func buildCropFilter(keyframes []Keyframe) string {
	if len(keyframes) == 0 {
		// Default: portrait 9:16 crop, centered horizontally, full frame height.
		// Ensure even dimensions for codec compatibility.
		// width = floor(in_h*9/16) rounded to even, height = in_h, x = centered even, y = 0
		return "crop=floor(in_h*9/16/2)*2:in_h:floor((in_w-out_w)/2/2)*2:0"
	}
	if len(keyframes) == 1 {
		kf := keyframes[0]
		return fmt.Sprintf("crop=%d:%d:%d:%d", kf.W, kf.H, kf.X, kf.Y)
	}

	cropW := keyframes[0].W
	cropH := keyframes[0].H

	xExpr := buildSmoothTransitionExpression(keyframes, "X")
	yExpr := buildSmoothTransitionExpression(keyframes, "Y")

	return fmt.Sprintf("crop=%d:%d:x='%s':y='%s'", cropW, cropH, xExpr, yExpr)
}

func buildSmoothTransitionExpression(keyframes []Keyframe, param string) string {
	var conditions []string
	for i := len(keyframes) - 1; i >= 1; i-- {
		currentKf := keyframes[i]
		if currentKf.JumpCut {
			target := getParamValue(currentKf, param)
			conditions = append(conditions,
				fmt.Sprintf("if(gte(t,%.3f),%d,", currentKf.StartTimestamp, target))
		} else {
			prev := getParamValue(keyframes[i-1], param)
			target := getParamValue(currentKf, param)

			dist := calculateDistance(keyframes[i-1], currentKf)
			panDur := calculatePanDuration(dist)
			end := currentKf.StartTimestamp + panDur

			norm := fmt.Sprintf("(t-%.3f)/%.3f", currentKf.StartTimestamp, panDur)
			ease := "(1-pow(1-(" + norm + "),2))"

			smooth := fmt.Sprintf("if(lte(t,%.3f),%d+(%d-%d)*%s,%d)",
				end, prev, target, prev, ease, target)
			conditions = append(conditions,
				fmt.Sprintf("if(gte(t,%.3f),%s,", currentKf.StartTimestamp, smooth))
		}
	}
	result := strings.Join(conditions, "")
	first := getParamValue(keyframes[0], param)
	result += strconv.Itoa(first) + strings.Repeat(")", len(conditions))
	return result
}

func calculateDistance(kf1, kf2 Keyframe) float64 {
	dx := float64(kf2.X - kf1.X)
	dy := float64(kf2.Y - kf1.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

func calculatePanDuration(distance float64) float64 {
	const (
		minDur = 0.1
		maxDur = 3.0
		speed  = 100.0
	)
	d := distance / speed
	if d < minDur {
		d = minDur
	}
	if d > maxDur {
		d = maxDur
	}
	return d
}

func getParamValue(kf Keyframe, param string) int {
	switch param {
	case "X":
		return kf.X
	case "Y":
		return kf.Y
	}
	return 0
}
