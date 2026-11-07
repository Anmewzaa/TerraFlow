// Package hcl parses Terraform (.tf), evaluates var/locals/tfvars, builds the
// resource graph (nodes, cross-folder references) and, from Phase 4, generates
// module calls with hclwrite. This is the highest-risk code in TerraFlow and is
// guarded by golden-file and round-trip tests.
package hcl
