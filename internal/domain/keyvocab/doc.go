// Package keyvocab owns the key and modifier vocabulary shared between the
// event-tap backends and the mode handler: canonical key and modifier
// spellings, and the synthetic "__keyup_"/"__modifier_" wire events the taps
// emit and the modes consume.
//
// Every producer and consumer of these strings must go through this package.
// The two native emitters that cannot (the Wayland C overlay and the macOS
// Objective-C event tap format the events with printf) are pinned to the same
// prefixes by an architecture test, so a change here fails loudly instead of
// silently splitting the protocol.
package keyvocab
