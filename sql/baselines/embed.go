package baselines

import _ "embed"

// CandidateSchemaV1 is the review-only schema snapshot generated from the
// current local working tree. It is not an approved release baseline.
//
//go:embed schema-v1-candidate-20261113.sql
var CandidateSchemaV1 []byte
