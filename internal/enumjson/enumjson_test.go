package enumjson

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/orsinium-labs/enum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type colour enum.Member[string]

var (
	red           = colour{Value: "red"}
	blue          = colour{Value: "blue"}
	colours       = enum.New(red, blue)
	errNoSuchHue  = errors.New("no such colour")
	errOtherCause = errors.New("other")
)

func TestMarshal_BareString(t *testing.T) {
	b, err := Marshal(red)
	require.NoError(t, err)
	assert.Equal(t, `"red"`, string(b))
}

func TestUnmarshalStrict(t *testing.T) {
	var got colour
	require.NoError(t, UnmarshalStrict([]byte(`"blue"`), colours, &got, errNoSuchHue))
	assert.Equal(t, blue, got)

	err := UnmarshalStrict([]byte(`"green"`), colours, &got, errNoSuchHue)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoSuchHue)
	assert.Contains(t, err.Error(), `"green"`)
	assert.NotErrorIs(t, err, errOtherCause)

	var jsonErr *json.SyntaxError
	assert.ErrorAs(t, UnmarshalStrict([]byte(`{`), colours, &got, errNoSuchHue), &jsonErr)
}

func TestUnmarshalOpen_KeepsUnknownValues(t *testing.T) {
	var got colour
	require.NoError(t, UnmarshalOpen([]byte(`"green"`), &got))
	assert.Equal(t, "green", got.Value)
	assert.False(t, colours.Contains(got))

	require.NoError(t, UnmarshalOpen([]byte(`"red"`), &got))
	assert.Equal(t, red, got)
}
