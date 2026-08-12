package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	selfupdate "github.com/creativeprojects/go-selfupdate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const publishWorkflow = "../../.github/workflows/publish.yml"

// The worker refuses to install a release whose checksum asset is missing, so
// the file publish.yml uploads has to be named the way the validator asks for
// it. Nothing else connects the two.
func TestPublishWorkflowUploadsTheChecksumTheUpdaterRequires(t *testing.T) {
	suffix := strings.TrimPrefix((&selfupdate.SHAValidator{}).GetValidationAssetName("BINARY"), "BINARY")
	require.Equal(t, ".sha256", suffix)

	yml, err := os.ReadFile(publishWorkflow)
	require.NoError(t, err)

	assert.Contains(t, string(yml), `sha256sum "$BINARY_NAME" > "$BINARY_NAME`+suffix+`"`,
		"publish.yml must write the checksum file")
	assert.Contains(t, string(yml), "${{env.BINARY_NAME}}"+suffix,
		"publish.yml must attach the checksum file to the release")
}

// sha256sum prints "<hash>  <name>", and BINARY_NAME contains a slash, so the
// second column is a path. The validator reads the first 64 characters and
// ignores the rest — this pins that it really does.
func TestSHAValidatorAcceptsSha256sumOutput(t *testing.T) {
	binary := []byte("pretend this is the worker binary")
	checksumFile := fmt.Sprintf("%x  bcc-code/bcc-media-flows-worker-linux-amd64\n", sha256.Sum256(binary))

	err := (&selfupdate.SHAValidator{}).Validate("bcc-media-flows-worker-linux-amd64", binary, []byte(checksumFile))
	assert.NoError(t, err)
}

func TestSHAValidatorRejectsAMismatch(t *testing.T) {
	checksumFile := fmt.Sprintf("%x  bcc-media-flows-worker-linux-amd64\n", sha256.Sum256([]byte("the binary that was published")))

	err := (&selfupdate.SHAValidator{}).Validate(
		"bcc-media-flows-worker-linux-amd64",
		[]byte("the binary that was downloaded"),
		[]byte(checksumFile),
	)
	assert.Error(t, err)
}
