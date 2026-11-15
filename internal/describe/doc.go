// Package describe builds the manifest `neru describe` prints and the
// daemon's describe command answers with: every word Neru reads, joined from
// the declarations that own each one, with the platforms each works on.
//
// It adds no facts of its own. A word missing here is missing from the
// vocabulary that declares it, and the guardrails there are what catch it.
package describe
