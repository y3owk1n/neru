//
//  accessibility_visibility.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import <CoreGraphics/CoreGraphics.h>
#import <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

char *cfStringToCString(CFStringRef cfStr);
bool isPointVisible(CGPoint point, pid_t elementPid);

#ifdef __cplusplus
}
#endif
