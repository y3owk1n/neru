//
//  accessibility_space_darwin.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility.h"

#import <ApplicationServices/ApplicationServices.h>
#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <dispatch/dispatch.h>
#import <mach-o/dyld.h>
#import <mach-o/loader.h>
#import <mach-o/nlist.h>
#import <objc/message.h>
#import <objc/objc.h>
#import <objc/runtime.h>

#pragma mark - SkyLight External Declarations

// Private SkyLight / WindowServer symbols (not in the public SDK).
extern int SLSMainConnectionID(void);
extern CFArrayRef SLSCopyManagedDisplaySpaces(int cid);
extern CFStringRef SLSCopyManagedDisplayForSpace(int cid, uint64_t sid);
extern uint64_t SLSManagedDisplayGetCurrentSpace(int cid, CFStringRef uuid);
extern CGError SLSSetActiveMenuBarDisplayIdentifier(int cid, CFStringRef uuid, CFStringRef repeat_uuid);
extern CGError SLSGetCurrentCursorLocation(int cid, CGPoint *point);
extern AXError _AXUIElementGetWindow(AXUIElementRef element, CGWindowID *out);
extern CGError SLSMoveWindowsToManagedSpace(int cid, CFArrayRef window_list, uint64_t sid);

#pragma mark - Display / Space Helpers

/// Translate a display ID to its UUID string.
/// @param did Display identifier
/// @return Retained CFStringRef (caller must CFRelease), or NULL on failure
static CFStringRef neruDisplayUUID(uint32_t did) {
	CFUUIDRef uuidRef = CGDisplayCreateUUIDFromDisplayID(did);
	if (!uuidRef) {
		return NULL;
	}

	CFStringRef uuidStr = CFUUIDCreateString(NULL, uuidRef);
	CFRelease(uuidRef);

	return uuidStr;
}

/// Translate a display UUID string back to a display ID.
/// @param uuid UUID string
/// @return Display ID, or 0 on failure
static uint32_t neruDisplayIDFromUUID(CFStringRef uuid) {
	if (!uuid) {
		return 0;
	}

	CFUUIDRef uuidRef = CFUUIDCreateFromString(NULL, uuid);
	if (!uuidRef) {
		return 0;
	}

	uint32_t did = CGDisplayGetDisplayIDFromUUID(uuidRef);
	CFRelease(uuidRef);

	return did;
}

/// Get the current Mission Control space for a display.
/// @param did Display identifier
/// @return Space ID, or 0 on failure
static uint64_t neruDisplaySpaceID(uint32_t did) {
	CFStringRef uuid = neruDisplayUUID(did);
	if (!uuid) {
		return 0;
	}

	uint64_t sid = SLSManagedDisplayGetCurrentSpace(SLSMainConnectionID(), uuid);
	CFRelease(uuid);

	return sid;
}

/// Return the center point of a display's bounds (in CG coordinates).
static CGPoint neruDisplayCenter(uint32_t did) {
	CGRect bounds = CGDisplayBounds(did);

	return (CGPoint){bounds.origin.x + bounds.size.width / 2.0, bounds.origin.y + bounds.size.height / 2.0};
}

/// Return the display ID that currently contains the cursor.
static uint32_t neruCursorDisplayID(void) {
	CGPoint cursor;
	SLSGetCurrentCursorLocation(SLSMainConnectionID(), &cursor);

	uint32_t matchingDisplays[16];
	uint32_t matchingCount = 0;

	CGError err = CGGetDisplaysWithPoint(cursor, 16, matchingDisplays, &matchingCount);
	if (err == kCGErrorSuccess && matchingCount > 0) {
		return matchingDisplays[0];
	}

	// Fall back to the main display if the cursor is somehow outside every display.
	return CGMainDisplayID();
}

/// Set the active menu bar display, which updates which display is the
/// "focused" one for Space purposes.
static void neruSetActiveMenuBarDisplay(uint32_t did) {
	CFStringRef uuid = neruDisplayUUID(did);
	if (!uuid) {
		return;
	}

	SLSSetActiveMenuBarDisplayIdentifier(SLSMainConnectionID(), uuid, uuid);
	CFRelease(uuid);
}

#pragma mark - Mission Control Index Resolution

/// Find the 1-based Mission Control indices of two space IDs in a single
/// pass over SLSCopyManagedDisplaySpaces. Returns false if either sid is
/// not found in the current Mission Control ordering.
static bool neruResolveMCIndices(uint64_t curSid, uint64_t newSid, int *outCurIndex, int *outNewIndex) {
	*outCurIndex = 0;
	*outNewIndex = 0;

	@autoreleasepool {
		CFArrayRef displaySpaces = SLSCopyManagedDisplaySpaces(SLSMainConnectionID());
		if (!displaySpaces) {
			return false;
		}

		int counter = 1;

		CFIndex displayCount = CFArrayGetCount(displaySpaces);
		for (CFIndex i = 0; i < displayCount; i++) {
			CFDictionaryRef displayRef = (CFDictionaryRef)CFArrayGetValueAtIndex(displaySpaces, i);
			CFArrayRef spacesRef = (CFArrayRef)CFDictionaryGetValue(displayRef, CFSTR("Spaces"));
			if (!spacesRef) {
				continue;
			}

			CFIndex spacesCount = CFArrayGetCount(spacesRef);
			for (CFIndex j = 0; j < spacesCount; j++) {
				CFDictionaryRef spaceRef = (CFDictionaryRef)CFArrayGetValueAtIndex(spacesRef, j);
				CFNumberRef sidRef = (CFNumberRef)CFDictionaryGetValue(spaceRef, CFSTR("id64"));
				if (!sidRef) {
					continue;
				}

				uint64_t sid = 0;
				CFNumberGetValue(sidRef, CFNumberGetType(sidRef), &sid);

				if (sid == curSid) {
					*outCurIndex = counter;
				}

				if (sid == newSid) {
					*outNewIndex = counter;
				}

				counter++;
			}
		}

		CFRelease(displaySpaces);

		return (*outCurIndex > 0) && (*outNewIndex > 0);
	}
}

#pragma mark - Public Space API

/// Get the total number of Mission Control spaces across all displays
/// in their current ordering.
int NeruCountMissionControlSpaces(void) {
	@autoreleasepool {
		CFArrayRef displaySpaces = SLSCopyManagedDisplaySpaces(SLSMainConnectionID());
		if (!displaySpaces) {
			return 0;
		}

		int total = 0;
		CFIndex displayCount = CFArrayGetCount(displaySpaces);
		for (CFIndex i = 0; i < displayCount; i++) {
			CFDictionaryRef displayRef = (CFDictionaryRef)CFArrayGetValueAtIndex(displaySpaces, i);
			CFArrayRef spacesRef = (CFArrayRef)CFDictionaryGetValue(displayRef, CFSTR("Spaces"));
			if (!spacesRef) {
				continue;
			}

			total += (int)CFArrayGetCount(spacesRef);
		}

		CFRelease(displaySpaces);

		return total;
	}
}

/// Get the space ID at the given 1-based Mission Control index.
uint64_t NeruMissionControlSpaceID(int index) {
	if (index < 1) {
		return 0;
	}

	@autoreleasepool {
		CFArrayRef displaySpaces = SLSCopyManagedDisplaySpaces(SLSMainConnectionID());
		if (!displaySpaces) {
			return 0;
		}

		uint64_t result = 0;
		int counter = 1;

		CFIndex displayCount = CFArrayGetCount(displaySpaces);
		for (CFIndex i = 0; i < displayCount; i++) {
			CFDictionaryRef displayRef = (CFDictionaryRef)CFArrayGetValueAtIndex(displaySpaces, i);
			CFArrayRef spacesRef = (CFArrayRef)CFDictionaryGetValue(displayRef, CFSTR("Spaces"));
			if (!spacesRef) {
				continue;
			}

			CFIndex spacesCount = CFArrayGetCount(spacesRef);
			for (CFIndex j = 0; j < spacesCount; j++) {
				if (counter == index) {
					CFDictionaryRef spaceRef = (CFDictionaryRef)CFArrayGetValueAtIndex(spacesRef, j);
					CFNumberRef sidRef = (CFNumberRef)CFDictionaryGetValue(spaceRef, CFSTR("id64"));
					if (sidRef) {
						CFNumberGetValue(sidRef, CFNumberGetType(sidRef), &result);
					}

					CFRelease(displaySpaces);

					return result;
				}

				counter++;
			}
		}

		CFRelease(displaySpaces);

		return 0;
	}
}

/// Get the display ID that owns a given space.
uint32_t NeruSpaceDisplayID(uint64_t sid) {
	CFStringRef uuid = SLSCopyManagedDisplayForSpace(SLSMainConnectionID(), sid);
	if (!uuid) {
		return 0;
	}

	uint32_t did = neruDisplayIDFromUUID(uuid);
	CFRelease(uuid);

	return did;
}

/// Get the space ID currently active on the cursor's display.
uint64_t NeruActiveSpaceID(void) { return neruDisplaySpaceID(neruCursorDisplayID()); }

#pragma mark - Gesture-Based Space Focus

// Private Core Graphics event field IDs used to synthesize a high-velocity
// horizontal dock swipe that the Dock treats as a real multi-finger swipe
// gesture. These constants are not part of the public SDK and require
// suppressing -Wdeprecated-declarations around the implementation.
static const int kNeruCGSEventTypeField = 55;              // kCGSEventTypeField
static const int kNeruCGSEventDockControl = 30;            // kCGSEventDockControl
static const int kNeruCGEventGestureHIDType = 110;         // kCGEventGestureHIDType
static const int kNeruIOHIDEventTypeDockSwipe = 23;        // kIOHIDEventTypeDockSwipe
static const int kNeruCGEventGestureSwipeMotion = 123;     // kCGEventGestureSwipeMotion
static const int kNeruCGGestureMotionHorizontal = 1;       // kCGGestureMotionHorizontal
static const int kNeruCGEventGestureSwipeProgress = 124;   // kCGEventGestureSwipeProgress
static const int kNeruCGEventGestureSwipeVelocityX = 129;  // kCGEventGestureSwipeVelocityX
static const int kNeruCGEventGesturePhase = 132;           // kCGEventGesturePhase
static const int kNeruCGSGesturePhaseBegan = 1;            // kCGSGesturePhaseBegan
static const int kNeruCGSGesturePhaseEnded = 4;            // kCGSGesturePhaseEnded

/// Focus a space using a synthetic high-velocity horizontal dock swipe
/// gesture to skip the standard Mission Control swipe animation — macOS
/// exposes no public API to activate a space directly.
///
/// Technique attribution: reverse-engineered from BetterTouchTool. Prior
/// art: https://github.com/jurplel/InstantSpaceSwitcher and the wacom-driver-fix
/// project by thenickdude.
int NeruFocusSpaceUsingGesture(uint32_t new_did, uint64_t new_sid) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

	uint32_t curDid = neruCursorDisplayID();
	uint64_t curSid = neruDisplaySpaceID(curDid);
	CGPoint point = neruDisplayCenter(new_did);
	bool focusDisplay = curDid != new_did;

	if (focusDisplay) {
		CGWarpMouseCursorPosition(point);
	}

	int curIndex = 0;
	int newIndex = 0;
	if (!neruResolveMCIndices(curSid, new_sid, &curIndex, &newIndex)) {
		// Could not resolve Mission Control indices (e.g. transient state).
		// Best-effort fallback: ensure the right display is active so the OS
		// picks the closest matching space on that display.
		neruSetActiveMenuBarDisplay(new_did);

		return 1;
	}

	int count = abs(newIndex - curIndex);
	if (count == 0) {
		// Already on the same Mission Control index. Make sure the right
		// display is active in case the destination space sits on a
		// different display at the same index.
		if (focusDisplay) {
			neruSetActiveMenuBarDisplay(new_did);
			if (neruDisplaySpaceID(new_did) != new_sid) {
				CGPostMouseEvent(point, false, 1, true);
				CGPostMouseEvent(point, false, 1, false);
			}
		}

		return 1;
	}

	CGEventRef event = CGEventCreate(NULL);
	if (!event) {
		return 0;
	}

	double sign = (newIndex - curIndex) > 0 ? 1.0 : -1.0;

	CGEventSetIntegerValueField(event, kNeruCGSEventTypeField, kNeruCGSEventDockControl);
	CGEventSetIntegerValueField(event, kNeruCGEventGestureHIDType, kNeruIOHIDEventTypeDockSwipe);
	CGEventSetIntegerValueField(event, kNeruCGEventGestureSwipeMotion, kNeruCGGestureMotionHorizontal);
	CGEventSetDoubleValueField(event, kNeruCGEventGestureSwipeProgress, sign);
	CGEventSetDoubleValueField(event, kNeruCGEventGestureSwipeVelocityX, sign * 9999.0);

	for (int i = 0; i < count; i++) {
		CGEventSetIntegerValueField(event, kNeruCGEventGesturePhase, kNeruCGSGesturePhaseBegan);
		CGEventPost(kCGSessionEventTap, event);
		CGEventSetIntegerValueField(event, kNeruCGEventGesturePhase, kNeruCGSGesturePhaseEnded);
		CGEventPost(kCGSessionEventTap, event);
	}

	CFRelease(event);

	if (focusDisplay) {
		neruSetActiveMenuBarDisplay(new_did);
		if (neruDisplaySpaceID(new_did) != new_sid) {
			CGPostMouseEvent(point, false, 1, true);
			CGPostMouseEvent(point, false, 1, false);
		}
	}

	return 1;

#pragma clang diagnostic pop
}

#pragma mark - Mach-O / Symbol Resolution Helpers

static struct mach_header_64 *neru_macho_find_image_header(const char *target_name, uint64_t *slide) {
	uint32_t image_count = _dyld_image_count();
	for (uint32_t i = 0; i < image_count; ++i) {
		const char *image_name = _dyld_get_image_name(i);
		if (!image_name)
			continue;
		if (strcmp(image_name, target_name) == 0) {
			*slide = _dyld_get_image_vmaddr_slide(i);
			return (struct mach_header_64 *)_dyld_get_image_header(i);
		}
	}
	return NULL;
}

static struct segment_command_64 *neru_macho_find_linkedit_segment(struct mach_header_64 *header) {
	uint64_t offset = sizeof(struct mach_header_64);
	for (uint32_t i = 0; i < header->ncmds; ++i) {
		struct load_command *cmd = (struct load_command *)(((uint8_t *)header) + offset);
		if (cmd->cmd == LC_SEGMENT_64) {
			struct segment_command_64 *segment = (struct segment_command_64 *)cmd;
			if (strcmp(segment->segname, SEG_LINKEDIT) == 0) {
				return segment;
			}
		}
		offset += cmd->cmdsize;
	}
	return NULL;
}

static struct symtab_command *neru_macho_find_symtab_command(struct mach_header_64 *header) {
	uint64_t offset = sizeof(struct mach_header_64);
	for (uint32_t i = 0; i < header->ncmds; ++i) {
		struct load_command *cmd = (struct load_command *)(((uint8_t *)header) + offset);
		if (cmd->cmd == LC_SYMTAB) {
			return (struct symtab_command *)cmd;
		}
		offset += cmd->cmdsize;
	}
	return NULL;
}

static void *neru_macho_find_symbol(const char *target_image, const char *target_symbol) {
	uint64_t slide = 0;
	struct mach_header_64 *header = neru_macho_find_image_header(target_image, &slide);
	if (!header)
		return NULL;
	struct segment_command_64 *linkedit_segment = neru_macho_find_linkedit_segment(header);
	if (!linkedit_segment)
		return NULL;
	struct symtab_command *symtab_command = neru_macho_find_symtab_command(header);
	if (!symtab_command)
		return NULL;
	uint32_t symbol_count = symtab_command->nsyms;
	void *symbol_str = (void *)(linkedit_segment->vmaddr - linkedit_segment->fileoff) + symtab_command->stroff + slide;
	void *symbol_sym = (void *)(linkedit_segment->vmaddr - linkedit_segment->fileoff) + symtab_command->symoff + slide;
	for (uint32_t i = 0; i < symbol_count; ++i) {
		struct nlist_64 *list = (void *)symbol_sym + (i * sizeof(struct nlist_64));
		char *symbol_name = (char *)symbol_str + list->n_un.n_strx;
		if (strcmp(symbol_name, target_symbol) == 0) {
			return (void *)(list->n_value + slide);
		}
	}
	return NULL;
}

#pragma mark - Window-to-Space Movement

int NeruMoveWindowToSpace(void *windowElement, uint64_t spaceID) {
	if (!windowElement) {
		return 0;
	}

	CGWindowID windowId = 0;
	AXError err = _AXUIElementGetWindow((AXUIElementRef)windowElement, &windowId);
	if (err != kAXErrorSuccess || windowId == 0) {
		return 0;
	}

	// Create CFArray of window ID
	CFNumberRef windowNumber = CFNumberCreate(NULL, kCFNumberSInt32Type, &windowId);
	if (!windowNumber) {
		return 0;
	}
	CFArrayRef windowList = CFArrayCreate(NULL, (const void **)&windowNumber, 1, &kCFTypeArrayCallBacks);
	CFRelease(windowNumber);
	if (!windowList) {
		return 0;
	}

	int success = 0;

	// Resolve SLSPerformAsynchronousBridgedWindowManagementOperation dynamically
	static int64_t (*SLSPerformAsynchronousBridgedWindowManagementOperation)(void *) = NULL;
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		SLSPerformAsynchronousBridgedWindowManagementOperation = (int64_t (*)(void *))neru_macho_find_symbol(
		    "/System/Library/PrivateFrameworks/SkyLight.framework/Versions/A/SkyLight",
		    "__"
		    "ZL54SLSPerformAsynchronousBridgedWindowManagementOperationP47SLSAsynchronousBridgedWindowManagementOperati"
		    "on");
	});

	if (SLSPerformAsynchronousBridgedWindowManagementOperation) {
		Class cls = objc_getClass("SLSBridgedMoveWindowsToManagedSpaceOperation");
		if (cls) {
			SEL sel = sel_registerName("initWithWindows:spaceID:");
			id operation =
			    ((id (*)(id, SEL, id, uint64_t))objc_msgSend)([cls alloc], sel, (__bridge id)windowList, spaceID);
			if (operation) {
				SLSPerformAsynchronousBridgedWindowManagementOperation((__bridge void *)operation);
				success = 1;
			}
		}
	}

	// Fallback to SLSMoveWindowsToManagedSpace
	if (!success) {
		CGError cgErr = SLSMoveWindowsToManagedSpace(SLSMainConnectionID(), windowList, spaceID);
		if (cgErr == kCGErrorSuccess) {
			success = 1;
		}
	}

	CFRelease(windowList);
	return success;
}
