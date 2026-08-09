//
//  callback_context.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef CALLBACK_CONTEXT_H
#define CALLBACK_CONTEXT_H

#import <stdint.h>

/// Async overlay resize callback context. Allocated on the C heap so native
/// code can retain it across dispatch boundaries, and cast straight to
/// overlayutil.CallbackContext by the Go side that reads it back.
///
/// The two layouts are held together by
/// internal/architecture/callback_context_layout_test.go, which pins the field
/// names, their order and their widths. Change one side there and it fails.
typedef struct {
	uint64_t callbackID;
	uint64_t generation;
} callbackContext;

#endif /* CALLBACK_CONTEXT_H */
