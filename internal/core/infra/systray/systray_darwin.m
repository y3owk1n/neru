//
//  systray.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "systray.h"

#import <Cocoa/Cocoa.h>

#pragma mark - External Function Declarations

extern void systray_menu_item_selected(int menuId);
extern void systray_on_ready(void);
extern void systray_on_exit(void);

#pragma mark - Static State

static BOOL _showSystray = YES;

#pragma mark - App Delegate

@interface AppDelegate : NSObject <NSApplicationDelegate, NSMenuDelegate>
@property(strong) NSStatusItem *statusItem;
@property(strong) NSMenu *menu;
@end

@implementation AppDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
	if (_showSystray) {
		self.statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
		self.menu = [[NSMenu alloc] init];
		[self.menu setAutoenablesItems:NO];
		[self.menu setDelegate:self];
		[self.statusItem setMenu:self.menu];
		[self.statusItem.button setImagePosition:NSImageOnly];
	}

	// Notify Go that we are ready
	systray_on_ready();
}

- (void)applicationWillTerminate:(NSNotification *)notification {
	systray_on_exit();
}

- (void)itemClicked:(id)sender {
	NSMenuItem *item = (NSMenuItem *)sender;
	systray_menu_item_selected((int)[item tag]);
}

@end

#pragma mark - Native Loop Functions

AppDelegate *appDelegate;

void registerSystray(void) {
	// Placeholder if needed for init
}

void internalNativeLoop(void) {
	@autoreleasepool {
		[NSApplication sharedApplication];
		appDelegate = [[AppDelegate alloc] init];
		[NSApp setDelegate:appDelegate];
		[NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];
		[NSApp run];
	}
}

void nativeLoop(void) {
	_showSystray = YES;
	internalNativeLoop();
}

void nativeLoopHeadless(void) {
	_showSystray = NO;
	internalNativeLoop();
}

void quit(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp terminate:nil];
	});
}

#pragma mark - Status Item Functions

void setIcon(const char *iconBytes, int length, bool isTemplate) {
	// Copy the icon bytes before dispatching so the caller can free
	// the original buffer immediately after this function returns.
	NSData *data = [NSData dataWithBytes:iconBytes length:length];

	dispatch_async(dispatch_get_main_queue(), ^{
		NSImage *image = [[NSImage alloc] initWithData:data];

		// Menu bar icons are 22×22 points (44×44 @2x retina). Setting the size
		// explicitly ensures macOS renders the icon at the correct dimensions
		// regardless of the source PNG pixel size.
		[image setSize:NSMakeSize(22, 22)];
		[image setTemplate:isTemplate];

		if (appDelegate && appDelegate.statusItem) {
			appDelegate.statusItem.button.image = image;
		}
	});
}

void setTitle(const char *title) {
	NSString *str = [NSString stringWithUTF8String:title];

	dispatch_async(dispatch_get_main_queue(), ^{
		if (appDelegate && appDelegate.statusItem) {
			appDelegate.statusItem.button.title = str;
		}
	});
}

void setTooltip(const char *tooltip) {
	NSString *str = [NSString stringWithUTF8String:tooltip];

	dispatch_async(dispatch_get_main_queue(), ^{
		if (appDelegate && appDelegate.statusItem) {
			appDelegate.statusItem.button.toolTip = str;
		}
	});
}

#pragma mark - Menu Item Lookup

NSMenuItem *findItemByTagInMenu(NSMenu *menu, int menuId) {
	if (!menu)
		return nil;

	// Check top-level items first
	NSMenuItem *item = [menu itemWithTag:menuId];
	if (item)
		return item;

	// Recursively search submenus
	for (NSMenuItem *menuItem in [menu itemArray]) {
		if ([menuItem hasSubmenu]) {
			item = findItemByTagInMenu([menuItem submenu], menuId);
			if (item)
				return item;
		}
	}

	return nil;
}

NSMenuItem *findItemByTag(int menuId) {
	if (!appDelegate || !appDelegate.menu)
		return nil;

	return findItemByTagInMenu(appDelegate.menu, menuId);
}

#pragma mark - Helper Functions

void runOnMainThread(void (^block)(void)) {
	if ([NSThread isMainThread]) {
		block();
	} else {
		dispatch_async(dispatch_get_main_queue(), block);
	}
}

#pragma mark - Menu Item Functions

void add_menu_item(int menuId, const char *title, short disabled, short checked) {
	NSString *titleStr = [NSString stringWithUTF8String:title];

	runOnMainThread(^{
		NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:titleStr action:@selector(itemClicked:) keyEquivalent:@""];
		[item setTarget:appDelegate];
		[item setTag:menuId];
		[item setEnabled:!disabled];
		[item setState:checked ? NSControlStateValueOn : NSControlStateValueOff];

		[appDelegate.menu addItem:item];
	});
}

void add_sub_menu_item(int parentId, int menuId, const char *title, short disabled, short checked) {
	NSString *titleStr = [NSString stringWithUTF8String:title];

	runOnMainThread(^{
		NSMenuItem *parent = findItemByTag(parentId);
		if (!parent)
			return;

		if (![parent submenu]) {
			NSMenu *submenu = [[NSMenu alloc] init];
			[submenu setAutoenablesItems:NO];
			[parent setSubmenu:submenu];
		}

		NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:titleStr action:@selector(itemClicked:) keyEquivalent:@""];
		[item setTarget:appDelegate];
		[item setTag:menuId];
		[item setEnabled:!disabled];
		[item setState:checked ? NSControlStateValueOn : NSControlStateValueOff];

		[[parent submenu] addItem:item];
	});
}

void add_separator(int parentId) {
	runOnMainThread(^{
		if (parentId == 0) {
			[appDelegate.menu addItem:[NSMenuItem separatorItem]];
			return;
		}

		NSMenuItem *parent = findItemByTag(parentId);
		if (!parent)
			return;

		if (![parent submenu]) {
			NSMenu *submenu = [[NSMenu alloc] init];
			[submenu setAutoenablesItems:NO];
			[parent setSubmenu:submenu];
		}

		[[parent submenu] addItem:[NSMenuItem separatorItem]];
	});
}

void hide_menu_item(int menuId) {
	runOnMainThread(^{
		NSMenuItem *item = findItemByTag(menuId);
		if (item)
			[item setHidden:YES];
	});
}

void show_menu_item(int menuId) {
	runOnMainThread(^{
		NSMenuItem *item = findItemByTag(menuId);
		if (item)
			[item setHidden:NO];
	});
}

void set_item_checked(int menuId, short checked) {
	runOnMainThread(^{
		NSMenuItem *item = findItemByTag(menuId);
		if (item)
			[item setState:checked ? NSControlStateValueOn : NSControlStateValueOff];
	});
}

void set_item_disabled(int menuId, short disabled) {
	runOnMainThread(^{
		NSMenuItem *item = findItemByTag(menuId);
		if (item)
			[item setEnabled:!disabled];
	});
}

void set_item_title(int menuId, const char *title) {
	NSString *str = [NSString stringWithUTF8String:title];

	runOnMainThread(^{
		NSMenuItem *item = findItemByTag(menuId);
		if (item)
			[item setTitle:str];
	});
}
