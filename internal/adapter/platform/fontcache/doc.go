// Package fontcache remembers what a platform font resolver answered for a
// written font family name, so the platform's font system is asked once per
// name rather than once per frame.
//
// It exists as one implementation because remembering an answer must not
// change it. Every platform resolver carried its own copy of this cache, and
// each of them keyed it on a lowercased, trimmed name while storing the answer
// for the name as written: after somebody resolved "Arial", a later "ARIAL"
// was answered "Arial" — the first caller's spelling, not its own. The answer
// was a function of what had been asked before, which is the one thing a cache
// may never be. ADR 0007 has the reasoning for collapsing the copies.
//
// A name is therefore remembered exactly as it was written. Two spellings of
// one family cost two entries — a bounded price, since family names come from
// configuration — and each caller is answered from its own name.
package fontcache
