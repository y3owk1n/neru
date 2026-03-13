//
//  secureinput.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef SECUREINPUT_H
#define SECUREINPUT_H

#import <Foundation/Foundation.h>

#pragma mark - Secure Input Detection

/// Check if macOS secure input mode is currently enabled
/// @return 1 if secure input is enabled, 0 otherwise
int isSecureInputEnabled(void);

/// Show a notification informing the user that secure input is active
void showSecureInputNotification(void);

#endif /* SECUREINPUT_H */
