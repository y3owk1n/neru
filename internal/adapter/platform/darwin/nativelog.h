//
//  nativelog.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef NATIVELOG_H
#define NATIVELOG_H

#import <Foundation/Foundation.h>

typedef NS_ENUM(int, NeruLogLevel) {
	NeruLogLevelDebug = 0,
	NeruLogLevelInfo = 1,
	NeruLogLevelWarn = 2,
	NeruLogLevelError = 3,
};

/// Log through the daemon's logger, so native code follows the same levels,
/// sinks and naming as the Go side.
/// @param level Severity
/// @param message Fixed message, no values interpolated
/// @param detail Optional value for the message, logged as its own field (may be nil)
void NeruLog(NeruLogLevel level, NSString *_Nonnull message, NSString *_Nullable detail);

#endif /* NATIVELOG_H */
