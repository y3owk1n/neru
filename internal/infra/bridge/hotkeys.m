//
//  hotkeys.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "hotkeys.h"
#import <Carbon/Carbon.h>
#import <Cocoa/Cocoa.h>

#pragma mark - Forward Declarations

static OSStatus hotkeyHandler(EventHandlerCallRef nextHandler, EventRef event, void *userData);

#pragma mark - Global Variables

static NSMutableDictionary *hotkeyRefs = nil;
static NSMutableDictionary *hotkeyCallbacks = nil;
static EventHandlerRef eventHandlerRef = NULL;
static dispatch_queue_t hotkeyQueue = nil;

#pragma mark - Storage Functions

/// Initialize storage
static void initializeStorage(void) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		hotkeyRefs = [NSMutableDictionary dictionaryWithCapacity:20]; // Pre-size for typical hotkey count
		hotkeyCallbacks = [NSMutableDictionary dictionaryWithCapacity:20];
		hotkeyQueue = dispatch_queue_create("com.neru.hotkeys", DISPATCH_QUEUE_SERIAL);

		// Install event handler only once
		EventTypeSpec eventType;
		eventType.eventClass = kEventClassKeyboard;
		eventType.eventKind = kEventHotKeyPressed;
		InstallApplicationEventHandler(&hotkeyHandler, 1, &eventType, NULL, &eventHandlerRef);
	});
}

#pragma mark - Event Handler Functions

/// Hotkey handler
/// @param nextHandler Next event handler
/// @param event Event reference
/// @param userData User data pointer
/// @return OSStatus
static OSStatus hotkeyHandler(EventHandlerCallRef nextHandler, EventRef event, void *userData) {
	EventHotKeyID hotkeyID;
	GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(hotkeyID), NULL, &hotkeyID);

	int hotkeyId = (int)hotkeyID.id;

	// Thread-safe callback retrieval
	__block HotkeyCallback callback = NULL;
	__block void *callbackUserData = NULL;

	dispatch_sync(hotkeyQueue, ^{
		NSNumber *key = @(hotkeyId);
		NSDictionary *callbackInfo = hotkeyCallbacks[key];
		if (callbackInfo) {
			callback = [callbackInfo[@"callback"] pointerValue];
			callbackUserData = [callbackInfo[@"userData"] pointerValue];
		}
	});

	// Invoke callback outside the lock
	if (callback) {
		callback(hotkeyId, callbackUserData);
	}

	return noErr;
}

#pragma mark - Hotkey Functions

/// Register hotkey
/// @param keyCode Key code
/// @param modifiers Modifier keys
/// @param hotkeyId Hotkey identifier
/// @param callback Callback function
/// @param userData User data pointer
/// @return 1 on success, 0 on failure
int registerHotkey(int keyCode, int modifiers, int hotkeyId, HotkeyCallback callback, void *userData) {
	initializeStorage();

	// Convert modifiers
	UInt32 carbonModifiers = 0;
	if (modifiers & ModifierCmd)
		carbonModifiers |= cmdKey;
	if (modifiers & ModifierShift)
		carbonModifiers |= shiftKey;
	if (modifiers & ModifierAlt)
		carbonModifiers |= optionKey;
	if (modifiers & ModifierCtrl)
		carbonModifiers |= controlKey;

	// Create hotkey ID
	EventHotKeyID hotkeyID;
	hotkeyID.signature = 'gvim';
	hotkeyID.id = hotkeyId;

	// Register hotkey
	EventHotKeyRef hotkeyRef;
	OSStatus status =
	    RegisterEventHotKey(keyCode, carbonModifiers, hotkeyID, GetApplicationEventTarget(), 0, &hotkeyRef);

	if (status != noErr) {
		return 0;
	}

	// Store reference and callback (thread-safe)
	NSNumber *key = @(hotkeyId);
	NSDictionary *callbackInfo =
	    @{@"callback" : [NSValue valueWithPointer:callback], @"userData" : [NSValue valueWithPointer:userData]};

	dispatch_sync(hotkeyQueue, ^{
		hotkeyRefs[key] = [NSValue valueWithPointer:hotkeyRef];
		hotkeyCallbacks[key] = callbackInfo;
	});

	return 1;
}

/// Unregister hotkey
/// @param hotkeyId Hotkey identifier
void unregisterHotkey(int hotkeyId) {
	if (!hotkeyRefs)
		return;

	NSNumber *key = @(hotkeyId);

	__block EventHotKeyRef hotkeyRef = NULL;

	// Get ref (thread-safe)
	dispatch_sync(hotkeyQueue, ^{
		NSValue *refValue = hotkeyRefs[key];
		if (refValue) {
			hotkeyRef = [refValue pointerValue];
			[hotkeyRefs removeObjectForKey:key];
			[hotkeyCallbacks removeObjectForKey:key];
		}
	});

	// Unregister outside lock
	if (hotkeyRef) {
		UnregisterEventHotKey(hotkeyRef);
	}
}

/// Unregister all hotkeys
void unregisterAllHotkeys(void) {
	if (!hotkeyRefs)
		return;

	__block NSArray *allRefs = nil;

	// Get all refs (thread-safe)
	dispatch_sync(hotkeyQueue, ^{
		allRefs = [hotkeyRefs allValues];
		[hotkeyRefs removeAllObjects];
		[hotkeyCallbacks removeAllObjects];
	});

	// Unregister outside lock
	for (NSValue *refValue in allRefs) {
		EventHotKeyRef hotkeyRef = [refValue pointerValue];
		UnregisterEventHotKey(hotkeyRef);
	}
}

/// Cleanup (call this before app termination)
void cleanupHotkeys(void) {
	unregisterAllHotkeys();

	if (eventHandlerRef) {
		RemoveEventHandler(eventHandlerRef);
		eventHandlerRef = NULL;
	}

	if (hotkeyQueue) {
		// Don't release the queue on modern systems (ARC handles it)
		hotkeyQueue = nil;
	}
}

/// Parse key string (e.g., "Cmd+Shift+Space")
/// @param keyString Key string
/// @param keyCode Output parameter for key code
/// @param modifiers Output parameter for modifiers
/// @return 1 on success, 0 on failure
int parseKeyString(const char *keyString, int *keyCode, int *modifiers) {
	if (!keyString || !keyCode || !modifiers)
		return 0;

	@autoreleasepool {
		NSString *keyStr = @(keyString);
		NSArray *parts = [keyStr componentsSeparatedByString:@"+"];

		*modifiers = ModifierNone;
		NSString *mainKey = nil;

		for (NSString *part in parts) {
			NSString *trimmed = [part stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];

			if ([trimmed isEqualToString:@"Cmd"] || [trimmed isEqualToString:@"Command"]) {
				*modifiers |= ModifierCmd;
			} else if ([trimmed isEqualToString:@"Shift"]) {
				*modifiers |= ModifierShift;
			} else if ([trimmed isEqualToString:@"Alt"] || [trimmed isEqualToString:@"Option"]) {
				*modifiers |= ModifierAlt;
			} else if ([trimmed isEqualToString:@"Ctrl"] || [trimmed isEqualToString:@"Control"]) {
				*modifiers |= ModifierCtrl;
			} else {
				mainKey = trimmed;
			}
		}

		if (!mainKey)
			return 0;

		// Map key names to key codes
		NSDictionary *keyMap = @{
			@"Space" : @(49),
			@"Return" : @(36),
			@"Enter" : @(36),
			@"Escape" : @(53),
			@"Tab" : @(48),
			@"Delete" : @(51),
			@"Backspace" : @(51),

			// Letters
			@"A" : @(0),
			@"B" : @(11),
			@"C" : @(8),
			@"D" : @(2),
			@"E" : @(14),
			@"F" : @(3),
			@"G" : @(5),
			@"H" : @(4),
			@"I" : @(34),
			@"J" : @(38),
			@"K" : @(40),
			@"L" : @(37),
			@"M" : @(46),
			@"N" : @(45),
			@"O" : @(31),
			@"P" : @(35),
			@"Q" : @(12),
			@"R" : @(15),
			@"S" : @(1),
			@"T" : @(17),
			@"U" : @(32),
			@"V" : @(9),
			@"W" : @(13),
			@"X" : @(7),
			@"Y" : @(16),
			@"Z" : @(6),

			// Numbers
			@"0" : @(29),
			@"1" : @(18),
			@"2" : @(19),
			@"3" : @(20),
			@"4" : @(21),
			@"5" : @(23),
			@"6" : @(22),
			@"7" : @(26),
			@"8" : @(28),
			@"9" : @(25),

			// Function keys
			@"F1" : @(122),
			@"F2" : @(120),
			@"F3" : @(99),
			@"F4" : @(118),
			@"F5" : @(96),
			@"F6" : @(97),
			@"F7" : @(98),
			@"F8" : @(100),
			@"F9" : @(101),
			@"F10" : @(109),
			@"F11" : @(103),
			@"F12" : @(111),

			// Arrow keys
			@"Left" : @(123),
			@"Right" : @(124),
			@"Down" : @(125),
			@"Up" : @(126),
		};

		NSNumber *keyCodeNum = keyMap[mainKey];
		if (!keyCodeNum) {
			// Try uppercase version
			keyCodeNum = keyMap[[mainKey uppercaseString]];
		}

		if (!keyCodeNum) {
			return 0;
		}

		*keyCode = [keyCodeNum intValue];
		return 1;
	}
}
