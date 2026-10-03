//
//  nativelog_darwin.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "nativelog.h"

extern void neruNativeLog(int level, const char *message, const char *detail);

void NeruLog(NeruLogLevel level, NSString *_Nonnull message, NSString *_Nullable detail) {
	@autoreleasepool {
		neruNativeLog((int)level, [message UTF8String], detail ? [detail UTF8String] : NULL);
	}
}
