// Package architecture holds the guardrails that keep the codebase's shape
// honest. It has no runtime code — every file here is a test.
//
// The rules these enforce are all documented in prose elsewhere, and prose did
// not hold them: every violation the layering tests now catch was present in
// the tree before they were written. So each rule has a test, each test names
// the document that owns the rule, and a violation fails `just test` instead of
// waiting for someone to notice in review.
//
// What is pinned:
//
//   - layering_test.go        the hexagon's direction — domain stays pure, and
//     the domain, port and adapter layers never reach up into the app or UI.
//   - dependency_boundary_test.go  the "One Rule": only darwin-only code may
//     import the darwin platform bridge.
//   - platform_slots_test.go  the filename vocabulary for build-tagged files,
//     and the exemption single-platform packages earn.
//   - ports_test.go           every port has a mock.
//   - cgo_includes_test.go    every relative #include still resolves. This one
//     exists because a broken include is invisible to `go vet` and to the
//     cross-platform vet, and only appears on a real cgo build.
//   - doc_links_test.go       documentation never links to a path that does
//     not exist.
//   - test_quality_test.go    tests assert something.
//   - role_vocabulary_docs_test.go  the semantic role list matches its docs.
//
// A rule that needs an exemption list keeps it visible and self-checking: a
// companion test fails when an entry stops being real, so the list can only
// shrink.
package architecture
