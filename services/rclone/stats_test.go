package rclone

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

const testData = `{
  "bytes": 20271610994,
  "checks": 96,
  "deletedDirs": 0,
  "deletes": 0,
  "elapsedTime": 586931.772442667,
  "errors": 1,
  "eta": 52,
  "fatalError": false,
  "lastError": "update stor: 1 error occurred:\n\t* context canceled\n\n",
  "renames": 0,
  "retryError": true,
  "serverSideCopies": 0,
  "serverSideCopyBytes": 0,
  "serverSideMoveBytes": 0,
  "serverSideMoves": 0,
  "speed": 158895074.81386793,
  "totalBytes": 28627138421,
  "totalChecks": 96,
  "totalTransfers": 1000,
  "transferTime": 312.742943197,
  "transferring": [
    {
      "bytes": 912261120,
      "dstFs": "s3prod:",
      "eta": 43,
      "group": "job/157877",
      "name": "temp/workflows/9aecc24e-f1c3-4438-8d63-86264a8fe431/output/vod/aaa.mp4",
      "percentage": 45,
      "size": 2002813315,
      "speed": 25988660.337309636,
      "speedAvg": 25019179.976419266,
      "srcFs": "isilon:"
    },
    {
      "bytes": 392101888,
      "dstFs": "s3prod:",
      "eta": 61,
      "group": "job/157901",
      "name": "temp/workflows/9aecc24e-f1c3-4438-8d63-86264a8fe431/output/vod/aaaaaaaa.mp4",
      "percentage": 19,
      "size": 2002735204,
      "speed": 26052330.701924283,
      "speedAvg": 26071590.28334097,
      "srcFs": "isilon:"
    },
    {
      "bytes": 238485504,
      "dstFs": "s3prod:",
      "eta": 71,
      "group": "job/157908",
      "name": "temp/workflows/9aecc24e-f1c3-4438-8d63-86264a8fe431/output/vod/lkajsdlk.mp4",
      "percentage": 11,
      "size": 2002594703,
      "speed": 24290882.82658985,
      "speedAvg": 24624559.158032026,
      "srcFs": "isilon:"
    },
    {
      "bytes": 115343360,
      "dstFs": "s3prod:",
      "eta": 81,
      "group": "job/157914",
      "name": "temp/workflows/9aecc24e-f1c3-4438-8d63-86264a8fe431/output/vod/tplkajsd.mp4",
      "percentage": 5,
      "size": 2002544778,
      "speed": 23194316.9882288,
      "speedAvg": 23018968.78755493,
      "srcFs": "isilon:"
    },
    {
      "bytes": 0,
      "dstFs": "s3prod:",
      "eta": null,
      "group": "job/157920",
      "name": "temp/workflows/9aecc24e-f1c3-4438-8d63-86264a8fe431/output/vod/alsjd.mp4",
      "percentage": 0,
      "size": 2003031299,
      "speed": 0,
      "speedAvg": 0,
      "srcFs": "isilon:"
    }
  ],
  "transfers": 995
}
`

func TestRcloneCoreStats(t *testing.T) {
	var s CoreStats
	err := json.Unmarshal([]byte(testData), &s)
	assert.NoError(t, err)

	assert.Equal(t, 5, len(s.Transferring))
	assert.Equal(t, 0, s.Transferring[4].Eta)

	j := s.ForJob("157914")
	assert.Equal(t, s.Transferring[3], j)
}
