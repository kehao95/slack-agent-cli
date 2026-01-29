package output

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Printable defines data structures providing human-readable lines.
type Printable interface {
	Lines() []string
}

// Print writes output in the desired format based on --json flag.
func Print(cmd *cobra.Command, data interface{}) error {
	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		return printJSON(data)
	}
	return printHuman(data)
}

func printJSON(data interface{}) error {
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	fmt.Println(string(encoded))
	return nil
}

func printHuman(data interface{}) error {
	switch v := data.(type) {
	case Printable:
		for _, line := range v.Lines() {
			fmt.Println(line)
		}
		return nil
	case fmt.Stringer:
		fmt.Println(v.String())
		return nil
	default:
		fmt.Printf("%v\n", v)
		return nil
	}
}

// ListFormatter implements Printable for slices of strings.
type ListFormatter struct {
	Title     string
	LinesData []string
}

func (lf ListFormatter) Lines() []string {
	var out []string
	if lf.Title != "" {
		out = append(out, lf.Title)
		out = append(out, strings.Repeat("-", len(lf.Title)))
	}
	out = append(out, lf.LinesData...)
	return out
}
