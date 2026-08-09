// Package keyvocab owns the key and modifier vocabulary shared between the
// event-tap backends, the configuration validators and the mode handler: the
// named keys and their aliases, the canonical modifier spellings, and the
// synthetic "__keyup_"/"__modifier_" wire events the taps emit and the modes
// consume.
//
// This is the one home for those names (ADR 0008,
// docs/adr/0008-a-vocabulary-has-one-home.md): a named key is added here and
// nowhere else, and every producer and consumer of these strings goes through
// this package. internal/config's validators and normalizers read the
// declaration rather than keeping a set of their own.
//
// The native emitters that cannot import it (the Wayland C overlay and the
// macOS Objective-C event tap format the events with printf) are pinned to the
// same prefixes by an architecture test, so a change here fails loudly instead
// of silently splitting the protocol.
package keyvocab
