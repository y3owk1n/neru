//
//  overlay.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef OVERLAY_H
#define OVERLAY_H

#import <CoreGraphics/CoreGraphics.h>
#import <Foundation/Foundation.h>

#pragma mark - Type Definitions

/// Overlay window handle
typedef void *OverlayWindow;

/// Hint style configuration
typedef struct {
	int fontSize;           ///< Font size
	char *fontFamily;       ///< Font family
	char *backgroundColor;  ///< Background color
	char *textColor;        ///< Text color
	char *matchedTextColor; ///< Matched text color
	char *borderColor;      ///< Border color
	int borderRadius;       ///< Border radius
	int borderWidth;        ///< Border width
	int padding;            ///< Padding
	double opacity;         ///< Opacity
	int showArrow;          ///< Show arrow (0 = no arrow, 1 = show arrow)
} HintStyle;

/// Hint data
typedef struct {
	char *label;             ///< Hint label
	CGPoint position;        ///< Hint position
	CGSize size;             ///< Hint size
	int matchedPrefixLength; ///< Number of matched characters to highlight
} HintData;

/// Grid cell style configuration
typedef struct {
	int fontSize;                 ///< Font size
	char *fontFamily;             ///< Font family
	char *backgroundColor;        ///< Background color
	char *textColor;              ///< Text color
	char *matchedTextColor;       ///< Matched text color
	char *matchedBackgroundColor; ///< Matched background color
	char *matchedBorderColor;     ///< Matched border color
	char *borderColor;            ///< Border color
	int borderWidth;              ///< Border width
	double backgroundOpacity;     ///< Background opacity
	double textOpacity;           ///< Text opacity
} GridCellStyle;

/// Grid cell data
typedef struct {
	char *label;             ///< Cell label
	CGRect bounds;           ///< Cell rectangle
	int isMatched;           ///< Cell matches current input (1 = yes, 0 = no)
	int isSubgrid;           ///< Cell is part of subgrid (1 = yes, 0 = no)
	int matchedPrefixLength; ///< Number of matched characters at beginning of label
} GridCell;

/// Callback type for async operations
/// @param context Context pointer
typedef void (*ResizeCompletionCallback)(void *context);

#pragma mark - Overlay Window Functions

/// Create overlay window
/// @return Overlay window handle
OverlayWindow createOverlayWindow(void);

/// Destroy overlay window
/// @param window Overlay window handle
void NeruDestroyOverlayWindow(OverlayWindow window);

/// Show overlay window
/// @param window Overlay window handle
void NeruShowOverlayWindow(OverlayWindow window);

/// Hide overlay window
/// @param window Overlay window handle
void NeruHideOverlayWindow(OverlayWindow window);

/// Clear overlay
/// @param window Overlay window handle
void NeruClearOverlay(OverlayWindow window);

#pragma mark - Drawing Functions

/// Draw hints
/// @param window Overlay window handle
/// @param hints Array of hint data
/// @param count Number of hints
/// @param style Hint style
void NeruDrawHints(OverlayWindow window, HintData *hints, int count, HintStyle style);

/// Draw scroll highlight
/// @param window Overlay window handle
/// @param bounds Highlight bounds
/// @param color Highlight color
/// @param width Highlight width
void NeruDrawScrollHighlight(OverlayWindow window, CGRect bounds, char *color, int width);

/// Set overlay level
/// @param window Overlay window handle
/// @param level Overlay level
void NeruSetOverlayLevel(OverlayWindow window, int level);

/// Draw target dot
/// @param window Overlay window handle
/// @param center Dot center
/// @param radius Dot radius
/// @param color Dot color
/// @param borderColor Border color
/// @param borderWidth Border width
void NeruDrawTargetDot(OverlayWindow window, CGPoint center, double radius, const char *color, const char *borderColor,
                       double borderWidth);

/// Replace overlay window
/// @param pwindow Pointer to overlay window handle
void NeruReplaceOverlayWindow(OverlayWindow *pwindow);

/// Resize overlay to main screen
/// @param window Overlay window handle
void NeruResizeOverlayToMainScreen(OverlayWindow window);

/// Resize overlay to active screen
/// @param window Overlay window handle
void NeruResizeOverlayToActiveScreen(OverlayWindow window);

/// Resize overlay to active screen with callback
/// @param window Overlay window handle
/// @param callback Completion callback
/// @param context Callback context
void NeruResizeOverlayToActiveScreenWithCallback(OverlayWindow window, ResizeCompletionCallback callback,
                                                 void *context);

#pragma mark - Grid Functions

/// Draw grid cells
/// @param window Overlay window handle
/// @param cells Array of grid cells
/// @param count Number of cells
/// @param style Grid cell style
void NeruDrawGridCells(OverlayWindow window, GridCell *cells, int count, GridCellStyle style);

/// Draw window border lines
/// @param window Overlay window handle
/// @param lines Array of line rectangles
/// @param count Number of lines
/// @param color Line color
/// @param width Line width
/// @param opacity Line opacity
void NeruDrawWindowBorder(OverlayWindow window, CGRect *lines, int count, char *color, int width, double opacity);

/// Update grid match prefix
/// @param window Overlay window handle
/// @param prefix Match prefix
void NeruUpdateGridMatchPrefix(OverlayWindow window, const char *prefix);

/// Set hide unmatched cells
/// @param window Overlay window handle
/// @param hide Hide unmatched cells (1 = yes, 0 = no)
void NeruSetHideUnmatched(OverlayWindow window, int hide);

#endif // OVERLAY_H
