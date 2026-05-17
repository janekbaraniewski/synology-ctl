package dsm

import (
	"bytes"
	"strconv"
)

// flexBool decodes a JSON value that DSM might send as either a JSON
// boolean (true/false) or a JSON number (0/1) or the string "true"/"false"/"0"/"1".
// Several DSM endpoints disagree about which encoding to use for the
// same field across firmware versions, so we accept all three rather
// than break on the inconsistency.
type flexBool bool

// UnmarshalJSON implements json.Unmarshaler.
func (b *flexBool) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*b = false
		return nil
	}
	switch data[0] {
	case 't', 'T':
		*b = true
		return nil
	case 'f', 'F':
		*b = false
		return nil
	case '"':
		// Quoted variant — try the unquoted forms.
		if len(data) >= 2 {
			s := string(data[1 : len(data)-1])
			switch s {
			case "true", "1", "yes":
				*b = true
				return nil
			case "false", "0", "no", "":
				*b = false
				return nil
			}
		}
	}
	// Numeric variant.
	if n, err := strconv.ParseFloat(string(data), 64); err == nil {
		*b = n != 0
		return nil
	}
	*b = false
	return nil
}

// Bool returns the underlying bool, convenient for callers that don't
// want to leak the flexBool type out of the package.
func (b flexBool) Bool() bool { return bool(b) }
