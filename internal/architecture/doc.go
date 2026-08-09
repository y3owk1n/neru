// Package architecture holds the guardrail tests that keep the codebase's
// shape honest. Prose rules alone don't hold; each rule here fails `just test`
// when broken. Exemption lists are self-checking: a companion test fails when
// an entry stops being real, so a list can only shrink.
//
// Every guardrail has a file named for it, and this is all of them —
// doc_inventory_test.go fails when the two disagree:
//
//   - agent_contract_test.go — the agent guide layout: an AGENTS.md beside
//     every CLAUDE.md symlink, .agents/skills canonical, worktrees ignored.
//   - callback_context_layout_test.go — the C callback context struct and the
//     Go struct it is cast to have the same fields, in the same order, at the
//     same widths.
//   - cgo_includes_test.go — relative #include paths resolve, and native
//     headers are reached through internal/adapter/platform/<os>/.
//   - comment_paths_test.go — a path header, or a comment pointing at a
//     sibling source file, names a file that exists.
//   - config_chain_test.go — the links no projection can show: every config
//     validator reaching the ladder, every schema field reaching a default.
//   - config_derivation_test.go — every derived config value is derived in the
//     chain rather than recomputed at its reader.
//   - config_example_test.go — the pairing between the config schema and the
//     example TOML shipped with it.
//   - dependency_boundary_test.go — the darwin One Rule: only darwin-tagged
//     code reaches internal/adapter/platform/darwin.
//   - doc_inventory_test.go — this list, against the directory.
//   - doc_links_test.go — no contributor doc names a path that does not exist.
//   - foundation_slice_test.go — the test-foundation recipe holds every
//     package that runs everywhere, and CI runs the recipe.
//   - hint_placement_vocabulary_test.go — the hint placement vocabulary is the
//     same on both sides of the Go/Objective-C boundary.
//   - keyvocab_wire_test.go — the native key event emitters and keyvocab agree
//     on what they put on the wire.
//   - label_autohide_rule_test.go — the label autohide rule is pinned across
//     that same boundary, by running the native copy.
//   - layering_test.go — domain stays pure, infra does not import app, and app
//     reaches infra only through ports.
//   - mode_flag_contract_test.go — mode commands register exactly what the
//     grammar declares, and docs/CLI.md is generated from the descriptor table.
//   - named_key_tables_test.go — the two Objective-C key-name tables spell the
//     named-key vocabulary, agree with each other, and gap only where macOS has
//     no keycode.
//   - native_constants_test.go — the shared reader every language-boundary pin
//     goes through, rather than a second way to read a .h or a .m.
//   - native_rule_test.go — the comparison vocabulary the rule-shaped
//     language-boundary pins share, so no two of them read an operator
//     differently, plus the reader that finds a native definition's body.
//   - overlay_frame_test.go — an overlay Frame carries domain values only.
//   - platform_slots_test.go — platform files use the documented file slots,
//     tagged packages tag every file, package comments reach every target.
//   - ports_test.go — every port has a mock, and every mock asserts that it
//     satisfies the interface.
//   - repo_walk_test.go — the one walk the suite shares: what it prunes, what
//     it hands over, and the vacuity floor every caller asserts on it.
//   - role_vocabulary_docs_test.go — the config docs cover the current
//     semantic role vocabulary and nothing retired.
//   - sub_key_preview_autohide_rule_test.go — the sub-key-preview autohide rule
//     is pinned across that same boundary, by running the native copy against
//     the shared one.
//   - subgrid_cells_test.go — a subgrid's rectangles are computed once, so the
//     cell drawn and the cell clicked are the same one.
//   - subgrid_keys_test.go — the subgrid key set is decided once, and handed
//     over by every draw that can put one on screen.
//   - test_quality_test.go — no test swallows an error, and every test body
//     can fail.
//   - wayland_keypad_folds_test.go — the keypad names the Wayland keymap folds
//     to are the names the X11 and evdev taps answer with, and the ones their
//     tests expect.
package architecture
