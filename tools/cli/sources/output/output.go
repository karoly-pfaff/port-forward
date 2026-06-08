package output

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
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

// FormatBool returns "yes" for true and "no" for false.
func FormatBool(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// FormatBytes formats n as a human-readable byte count (e.g. "1.5 KB").
func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// FormatTimestamp parses an RFC 3339 timestamp and formats it as local time.
// Returns ts unchanged if parsing fails.
func FormatTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return ts
		}
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// PrintTable writes headers and rows as aligned columns to w.
func PrintTable(w io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i := range widths {
			if i < len(row) && len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}
	printTableRow(w, headers, widths)
	for _, row := range rows {
		printTableRow(w, row, widths)
	}
}

func printTableRow(w io.Writer, cells []string, widths []int) {
	for i, cell := range cells {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		if i < len(cells)-1 {
			fmt.Fprintf(w, "%-*s", widths[i], cell)
		} else {
			fmt.Fprint(w, cell)
		}
	}
	fmt.Fprintln(w)
}
