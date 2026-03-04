//
//  theme.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "theme.h"
#import <Cocoa/Cocoa.h>

#pragma mark - External Function Declarations

extern void handleThemeChanged(int isDark);

#pragma mark - Theme Observer

@interface ThemeObserver : NSObject
@end

@implementation ThemeObserver

/// Handle effective appearance change via KVO
- (void)observeValueForKeyPath:(NSString *)keyPath
                      ofObject:(id)object
                        change:(NSDictionary<NSKeyValueChangeKey, id> *)change
                       context:(void *)context {
	if ([keyPath isEqualToString:@"effectiveAppearance"]) {
		int dark = isDarkMode();
		handleThemeChanged(dark);
	}
}

@end

#pragma mark - Static State

static ThemeObserver *g_themeObserver = nil;

#pragma mark - Theme Detection Functions

/// Check if macOS Dark Mode is active
/// @return 1 if Dark Mode is active, 0 if Light Mode
int isDarkMode(void) {
	@autoreleasepool {
		NSAppearance *appearance = [NSApp effectiveAppearance];
		if (!appearance) {
			// Fallback: use system defaults
			NSString *style =
			    [[NSUserDefaults standardUserDefaults] stringForKey:@"AppleInterfaceStyle"];
			return [style isEqualToString:@"Dark"] ? 1 : 0;
		}

		NSAppearanceName bestMatch = [appearance
		    bestMatchFromAppearancesWithNames:@[ NSAppearanceNameAqua, NSAppearanceNameDarkAqua ]];
		return [bestMatch isEqualToString:NSAppearanceNameDarkAqua] ? 1 : 0;
	}
}

/// Start observing macOS theme changes using KVO on NSApp.effectiveAppearance
void startThemeObserver(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (g_themeObserver != nil) {
			return; // Already observing
		}

		g_themeObserver = [[ThemeObserver alloc] init];
		[NSApp addObserver:g_themeObserver
		        forKeyPath:@"effectiveAppearance"
		           options:NSKeyValueObservingOptionNew
		           context:nil];
	});
}

/// Stop observing macOS theme changes
void stopThemeObserver(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (g_themeObserver != nil) {
			@try {
				[NSApp removeObserver:g_themeObserver forKeyPath:@"effectiveAppearance"];
			} @catch (NSException *exception) {
				// Observer was already removed or not registered
			}
			g_themeObserver = nil;
		}
	});
}

