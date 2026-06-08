package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// PrintJSON writes v as indented JSON to w.
func PrintJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// PrintField writes a label/value pair in aligned human-readable format.
func PrintField(w io.Writer, label, value string) {
	fmt.Fprintf(w, "%-20s %s\n", label+":", value)
}
