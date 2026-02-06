#import <Cocoa/Cocoa.h>
#import "systray.h"

extern void systray_menu_item_selected(int menuId);
extern void systray_on_ready(void);
extern void systray_on_exit(void);

@interface AppDelegate : NSObject <NSApplicationDelegate, NSMenuDelegate>
@property(strong) NSStatusItem *statusItem;
@property(strong) NSMenu *menu;
@end

@implementation AppDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    self.statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
    self.menu = [[NSMenu alloc] init];
    [self.menu setAutoenablesItems:NO];
    [self.menu setDelegate:self];
    [self.statusItem setMenu:self.menu];
    
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

AppDelegate *appDelegate;

void registerSystray(void) {
    // Placeholder if needed for init
}

void nativeLoop(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        appDelegate = [[AppDelegate alloc] init];
        [NSApp setDelegate:appDelegate];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];
        [NSApp run];
    }
}

void quit(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp terminate:nil];
    });
}

void setIcon(const char* iconBytes, int length, bool template) {
    NSData* data = [NSData dataWithBytes:iconBytes length:length];
    NSImage* image = [[NSImage alloc] initWithData:data];
    [image setTemplate:template];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (appDelegate && appDelegate.statusItem) {
            appDelegate.statusItem.button.image = image;
        }
    });
}

void setTitle(const char* title) {
    NSString* str = [NSString stringWithUTF8String:title];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (appDelegate && appDelegate.statusItem) {
            appDelegate.statusItem.button.title = str;
        }
    });
}

void setTooltip(const char* tooltip) {
    NSString* str = [NSString stringWithUTF8String:tooltip];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (appDelegate && appDelegate.statusItem) {
            appDelegate.statusItem.button.toolTip = str;
        }
    });
}

NSMenuItem* findItemByTag(int menuId) {
    if (!appDelegate || !appDelegate.menu) return nil;
    return [appDelegate.menu itemWithTag:menuId];
}

void add_menu_item(int menuId, const char* title, const char* tooltip, short disabled, short checked) {
    NSString* titleStr = [NSString stringWithUTF8String:title];
    NSString* tooltipStr = [NSString stringWithUTF8String:tooltip];
    
    dispatch_async(dispatch_get_main_queue(), ^{
        NSMenuItem* item = [[NSMenuItem alloc] initWithTitle:titleStr action:@selector(itemClicked:) keyEquivalent:@""];
        [item setTarget:appDelegate];
        [item setTag:menuId];
        [item setToolTip:tooltipStr];
        [item setEnabled:!disabled];
        [item setState:checked ? NSControlStateValueOn : NSControlStateValueOff];
        
        [appDelegate.menu addItem:item];
    });
}

void add_sub_menu_item(int parentId, int menuId, const char* title, const char* tooltip, short disabled, short checked) {
    NSString* titleStr = [NSString stringWithUTF8String:title];
    NSString* tooltipStr = [NSString stringWithUTF8String:tooltip];
    
    dispatch_async(dispatch_get_main_queue(), ^{
        NSMenuItem* parent = findItemByTag(parentId);
        if (!parent) return;
        
        if (![parent submenu]) {
            NSMenu* submenu = [[NSMenu alloc] init];
            [submenu setAutoenablesItems:NO];
            [parent setSubmenu:submenu];
        }
        
        NSMenuItem* item = [[NSMenuItem alloc] initWithTitle:titleStr action:@selector(itemClicked:) keyEquivalent:@""];
        [item setTarget:appDelegate];
        [item setTag:menuId];
        [item setToolTip:tooltipStr];
        [item setEnabled:!disabled];
        [item setState:checked ? NSControlStateValueOn : NSControlStateValueOff];
        
        [[parent submenu] addItem:item];
    });
}

void add_separator(int menuId) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [appDelegate.menu addItem:[NSMenuItem separatorItem]];
    });
}

void hide_menu_item(int menuId) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSMenuItem* item = findItemByTag(menuId);
        if (item) [item setHidden:YES];
    });
}

void show_menu_item(int menuId) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSMenuItem* item = findItemByTag(menuId);
        if (item) [item setHidden:NO];
    });
}

void set_item_checked(int menuId, short checked) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSMenuItem* item = findItemByTag(menuId);
        if (item) [item setState:checked ? NSControlStateValueOn : NSControlStateValueOff];
    });
}

void set_item_disabled(int menuId, short disabled) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSMenuItem* item = findItemByTag(menuId);
        if (item) [item setEnabled:!disabled];
    });
}

void set_item_title(int menuId, const char* title) {
    NSString* str = [NSString stringWithUTF8String:title];
    dispatch_async(dispatch_get_main_queue(), ^{
        NSMenuItem* item = findItemByTag(menuId);
        if (item) [item setTitle:str];
    });
}

void set_item_tooltip(int menuId, const char* tooltip) {
    NSString* str = [NSString stringWithUTF8String:tooltip];
    dispatch_async(dispatch_get_main_queue(), ^{
        NSMenuItem* item = findItemByTag(menuId);
        if (item) [item setToolTip:str];
    });
}
