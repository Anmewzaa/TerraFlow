// Command visualize parses a Terraform folder tree and prints the graph JSON
// used by the TerraFlow canvas.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
)

func main() {
	dir := flag.String("dir", ".", "path to a Terraform folder tree to visualize")
	env := flag.String("env", "dev", "environment (selects env/<env>.tfvars)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	logger.Info("terraflow visualize", "dir", *dir, "env", *env)

	graph, err := hcl.BuildGraph(*dir, *env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(graph); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}
