// Command visualize parses a Terraform folder tree and prints the graph JSON
// used by the TerraFlow canvas. Phase 0: entry point only; the parser lands in
// Phase 1 (see docs/08-dev-plan.md).
package main

import (
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

	// Phase 1 will parse *dir and emit graph JSON to stdout.
	fmt.Fprintln(os.Stderr, "visualize: parser not implemented yet (Phase 1)")
	_ = hcl.Domains() // keep the package wired until Phase 1 fills it in
}
