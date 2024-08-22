package todoist

import (
	"encoding/json"
	"fmt"
	"time"
)

func multiCompare(results ...int) bool {
	for _, result := range results {
		if result != 0 { // not equal
			return result < 0
		}

		// continuing only if first comparison result equal
	}

	return false
}

func intCompare(a, b int) int {
	switch {
	case a == b:
		return 0
	case a < b:
		return -1
	default:
		return 1
	}
}

func int64Compare(a, b int64) int {
	switch {
	case a == b:
		return 0
	case a < b:
		return -1
	default:
		return 1
	}
}

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
