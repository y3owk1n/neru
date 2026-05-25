//
//  keyfeed.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef KEYFEED_H
#define KEYFEED_H

#import <Foundation/Foundation.h>

/// Post a key or key chord to macOS.
/// @param keyString Key string (e.g., "Cmd+Shift+Space")
/// @return 1 on success, 0 on invalid key, -1 on system failure
int postKeyFeed(const char *keyString);

#endif /* KEYFEED_H */
