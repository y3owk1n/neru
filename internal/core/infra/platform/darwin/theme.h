//
//  theme.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef THEME_H
#define THEME_H

#import <Foundation/Foundation.h>

#pragma mark - Theme Detection Functions

/// Check if macOS Dark Mode is active
/// @return 1 if Dark Mode is active, 0 if Light Mode
int isDarkMode(void);

/// Start observing macOS theme changes
/// Calls the Go callback handleThemeChanged when the system appearance changes
void startThemeObserver(void);

/// Stop observing macOS theme changes
void stopThemeObserver(void);

#endif /* THEME_H */

