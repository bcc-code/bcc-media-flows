// Package enumjson encodes string-valued enum members (types declared as
// `type T enum.Member[string]`) as their bare value in JSON, so a payload
// carries "xdcam" rather than {"Value":"xdcam"}. That keeps Temporal
// workflow histories written when a field was a plain string replayable
// after the field is retyped to an enum.
package enumjson

import (
	"encoding/json"
	"fmt"

	"github.com/orsinium-labs/enum"
)

// Member is any type whose underlying type is enum.Member[string].
type Member interface {
	~struct{ Value string }
}

// member is the underlying type; Go only permits field access through it,
// not through the type parameter.
type member = struct{ Value string }

// Marshal encodes m as its bare string value.
func Marshal[M Member](m M) ([]byte, error) {
	return json.Marshal(member(m).Value)
}

// UnmarshalStrict decodes a bare string into *dst and rejects values that are
// not members of e, wrapping notFound so callers can errors.Is against it.
// Use it for values this codebase produces itself (workflow inputs, form
// fields), where an unknown value is a bug or a bad request.
func UnmarshalStrict[M Member](data []byte, e enum.Enum[M, string], dst *M, notFound error) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	member := e.Parse(value)
	if member == nil {
		return fmt.Errorf("%w: %q", notFound, value)
	}
	*dst = *member
	return nil
}

// UnmarshalOpen decodes a bare string into *dst, keeping values that are not
// among the declared members. Use it for values an external system reports
// (Vidispine job states, Directus statuses), where an unlisted value must
// still round-trip and compare unequal to every named member rather than
// fail decoding.
func UnmarshalOpen[M Member](data []byte, dst *M) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*dst = M(member{Value: value})
	return nil
}
