//
//  callback_context.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef CALLBACK_CONTEXT_H
#define CALLBACK_CONTEXT_H

#import <stdint.h>

/// Async overlay resize callback context (matches overlayutil.CallbackContext in Go).
/// Allocated on the C heap so native code can retain it across dispatch boundaries.
typedef struct {
	uint64_t callbackID;
	uint64_t generation;
} callbackContext;

#endif /* CALLBACK_CONTEXT_H */
