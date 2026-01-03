package todoist

import (
	"encoding/json"
	"fmt"
	"time"
)

// only date component in format: 2006-01-02
type JSONPlainDate struct {
	time.Time
}

var _ json.Unmarshaler = (*JSONPlainDate)(nil)

func (b *JSONPlainDate) UnmarshalJSON(input []byte) error {
	parsed, err := time.Parse(`"2006-01-02"`, string(input))
	if err != nil {
		return fmt.Errorf("JSONPlainDate: %w", err)
	}

	*b = JSONPlainDate{parsed}

	return nil
}
