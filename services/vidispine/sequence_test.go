package vidispine_test

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/stretchr/testify/assert"
)

// Just make sure decoding works
func Test_DecodeSequenceXML(t *testing.T) {
	glob, err := filepath.Glob("testdata/sequences/*.xml")
	assert.NoError(t, err)

	for _, file := range glob {
		f, err := os.ReadFile(file)
		assert.NoError(t, err)

		doc := &vidispine.SequenceDocument{}
		err = xml.Unmarshal(f, doc)
		assert.NoError(t, err)
	}
}
