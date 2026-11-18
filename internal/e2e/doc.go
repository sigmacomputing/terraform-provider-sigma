// Package e2e holds cross-resource, end-to-end acceptance tests that wire
// multiple Sigma resource types together and assert the relationships between
// them (composite IDs and cross-references) resolve correctly. These complement
// the per-resource acceptance tests under internal/services/*. They are gated by
// TF_ACC and the standard acctest.PreCheck credentials.
package e2e
