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

/// Hint placement constants (must match HintPlacement enum in overlay_darwin.m)
#define HINT_PLACEMENT_TOP 1
#define HINT_PLACEMENT_CENTER 2
#define HINT_PLACEMENT_BOTTOM 3

/// Hint style configuration
typedef struct {
	int fontSize;                   ///< Font size
	char *fontFamily;               ///< Font family
	char *backgroundColor;          ///< Background color
	char *textColor;                ///< Text color
	char *matchedTextColor;         ///< Matched text color
	char *borderColor;              ///< Border color
	int borderRadius;               ///< Border radius (-1 = auto)
	int borderWidth;                ///< Border width
	int paddingX;                   ///< Horizontal padding (-1 = auto)
	int paddingY;                   ///< Vertical padding (-1 = auto)
	int showArrow;                  ///< Show arrow (0 = no arrow, 1 = show arrow)
	int placement;                  ///< Label placement relative to target
	int forceFlush;                 ///< Force synchronous display flush after draw (0 = coalesce, 1 = flush)
	int boundaryHighlightEnabled;   ///< Draw target boundary highlight (0 = off, 1 = on)
	int boundaryBorderWidth;        ///< Target boundary border width
	int boundaryBorderRadius;       ///< Target boundary corner radius
	char *boundaryBackgroundColor;  ///< Target boundary fill color
	char *boundaryBorderColor;      ///< Target boundary stroke color
} HintStyle;

/// Hint data
typedef struct {
	char *label;              ///< Hint label
	CGPoint position;         ///< Hint position
	CGSize size;              ///< Hint size
	int matchedPrefixLength;  ///< Number of matched characters to highlight
} HintData;

/// Hints search input style configuration
typedef struct {
	int fontSize;           ///< Font size
	char *fontFamily;       ///< Font family
	char *backgroundColor;  ///< Background color
	char *textColor;        ///< Text color
	char *borderColor;      ///< Border color
	int borderRadius;       ///< Border radius (-1 = auto)
	int borderWidth;        ///< Border width
	int paddingX;           ///< Horizontal padding (-1 = auto)
	int paddingY;           ///< Vertical padding (-1 = auto)
} SearchInputStyle;

/// Hints search input data
typedef struct {
	char *query;       ///< Current search query
	int resultCount;   ///< Current filtered hint count
	CGPoint position;  ///< Input position in overlay-local coordinates
	double width;      ///< Input width
} SearchInputData;

/// Grid cell style configuration
typedef struct {
	int fontSize;                     ///< Font size
	char *fontFamily;                 ///< Font family
	char *backgroundColor;            ///< Background color
	char *labelBackgroundColor;       ///< Label background color
	char *textColor;                  ///< Text color
	char *matchedTextColor;           ///< Matched text color
	char *matchedBackgroundColor;     ///< Matched background color
	char *matchedBorderColor;         ///< Matched border color
	char *borderColor;                ///< Border color
	int borderWidth;                  ///< Border width
	int drawLabelBackground;          ///< Draw labels with a badge background
	int labelBackgroundPaddingX;      ///< Label badge horizontal padding (-1 = auto)
	int labelBackgroundPaddingY;      ///< Label badge vertical padding (-1 = auto)
	int labelBackgroundBorderRadius;  ///< Label badge border radius (-1 = auto)
	int labelBackgroundBorderWidth;   ///< Label badge border width
	int subKeyGridCols;               ///< Sub-key preview grid columns (next depth's cols)
	int subKeyGridRows;               ///< Sub-key preview grid rows (next depth's rows)
	int drawSubKeyPreview;            ///< Draw miniature key grid inside each cell (1 = yes, 0 = no)
	int subKeyFontSize;               ///< Font size for sub-key preview labels
	float subKeyAutohideMultiplier;   ///< Minimum cell size multiplier for sub-key preview autohide (0 = disable)
	char *subKeyTextColor;            ///< Text color for sub-key preview labels
	char *subKeyKeys;                 ///< Key string for sub-key preview (next depth's keys, uppercased)
} GridCellStyle;

/// Grid cell data
typedef struct {
	char *label;              ///< Cell label
	CGRect bounds;            ///< Cell rectangle
	int isMatched;            ///< Cell matches current input (1 = yes, 0 = no)
	int isSubgrid;            ///< Cell is part of subgrid (1 = yes, 0 = no)
	int matchedPrefixLength;  ///< Number of matched characters at beginning of label
} GridCell;

/// Virtual cursor indicator style configuration
typedef struct {
	double radius;    ///< Outer radius in points
	char *fillColor;  ///< Fill color
} CursorIndicatorStyle;

/// Mouse action indicator style configuration
typedef struct {
	int size;               ///< Indicator diameter in points
	int borderWidth;        ///< Border width in points
	char *backgroundColor;  ///< Fill color
	char *borderColor;      ///< Stroke color
	char *shape;            ///< circle or square
	int durationMS;         ///< Animation duration in milliseconds
	double startScale;      ///< Initial transform scale
	double endScale;        ///< Final transform scale
	double startOpacity;    ///< Initial opacity
	double endOpacity;      ///< Final opacity
	char *easing;           ///< linear, ease_in, ease_out, or ease_in_out
	int hideInScreenShare;  ///< Hide panel from screen sharing (1 = hidden, 0 = visible)
} MouseActionIndicatorStyle;

/// Callback type for async operations
/// @param context Context pointer
typedef void (*ResizeCompletionCallback)(void *context);

#pragma mark - Overlay Window Functions

/// Create overlay window
/// @return Overlay window handle
OverlayWindow NeruCreateOverlayWindow(void);

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

/// Draw hints search input
/// @param window Overlay window handle
/// @param input Search input data
/// @param style Search input style
void NeruDrawHintSearchInput(OverlayWindow window, SearchInputData input, SearchInputStyle style);

/// Hide hints search input
/// @param window Overlay window handle
void NeruHideHintSearchInput(OverlayWindow window);

/// Update hint match prefix (incremental update for typing)
/// @param window Overlay window handle
/// @param prefix Match prefix
void NeruUpdateHintMatchPrefix(OverlayWindow window, const char *prefix);

/// Set overlay level
/// @param window Overlay window handle
/// @param level Overlay level
void NeruSetOverlayLevel(OverlayWindow window, int level);

/// Set overlay sharing type for screen sharing visibility
/// @param window Overlay window handle
/// @param sharingType Sharing type: 0 = NSWindowSharingNone (hidden), 1 = NSWindowSharingReadOnly (visible)
void NeruSetOverlaySharingType(OverlayWindow window, int sharingType);

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
void NeruResizeOverlayToActiveScreenWithCallback(
    OverlayWindow window, ResizeCompletionCallback callback, void *context);

#pragma mark - Grid Functions

/// Draw grid cells
/// @param window Overlay window handle
/// @param cells Array of grid cells
/// @param count Number of cells
/// @param style Grid cell style
void NeruDrawGridCells(OverlayWindow window, GridCell *cells, int count, GridCellStyle style);

/// Animate recursive-grid cells between depth changes.
/// @param window Overlay window handle
/// @param cells Target grid cells
/// @param count Number of target cells
/// @param style Grid cell style
/// @param duration Animation duration in seconds
void NeruAnimateRecursiveGridTransition(
    OverlayWindow window, GridCell *cells, int count, GridCellStyle style, double duration);

/// Update grid match prefix
/// @param window Overlay window handle
/// @param prefix Match prefix
void NeruUpdateGridMatchPrefix(OverlayWindow window, const char *prefix);

/// Set hide unmatched cells
/// @param window Overlay window handle
/// @param hide Hide unmatched cells (1 = yes, 0 = no)
void NeruSetHideUnmatched(OverlayWindow window, int hide);

/// Draw grid cells incrementally (add/update/remove specific cells without clearing entire overlay)
/// @param window Overlay window handle
/// @param cellsToAdd Array of grid cells to add or update
/// @param addCount Number of cells to add/update
/// @param cellsToRemove Array of cell bounds to remove (by matching bounds)
/// @param removeCount Number of cells to remove
/// @param style Grid cell style (used for new/updated cells)
void NeruDrawIncrementGrid(
    OverlayWindow window, GridCell *cellsToAdd, int addCount, CGRect *cellsToRemove, int removeCount,
    GridCellStyle style);

/// Show a virtual cursor indicator at the specified point
/// @param window Overlay window handle
/// @param position Indicator center position in overlay coordinates
/// @param style Indicator style
void NeruShowCursorIndicator(OverlayWindow window, CGPoint position, CursorIndicatorStyle style);

/// Hide the virtual cursor indicator
/// @param window Overlay window handle
void NeruHideCursorIndicator(OverlayWindow window);

/// Show a transient mouse action indicator in its own overlay window.
/// @param position Global cursor position in Quartz coordinates
/// @param style Indicator style
void NeruShowMouseActionIndicator(CGPoint position, MouseActionIndicatorStyle style);

/// Position and resize overlay window dynamically to fit a hint badge.
/// Calculates the hint size using the provided label text and style configuration,
/// sizes the window accordingly (with a small margin), and centers it on the target absolute position.
/// Clamps the window frame to the containing display bounds.
/// Returns the calculated window width and height via outWidth and outHeight pointers.
void NeruPositionAndSizeOverlayToFitHint(
    OverlayWindow window, double absoluteX, double absoluteY, const char *label, HintStyle style, double *outWidth,
    double *outHeight);

#pragma mark - Monitor Select Functions

/// Monitor select target data for per-monitor overlay rendering.
typedef struct {
	int x;                 ///< Monitor origin X (screen coordinates)
	int y;                 ///< Monitor origin Y (screen coordinates)
	int width;             ///< Monitor width
	int height;            ///< Monitor height
	char *label;           ///< Display label (e.g. "1", "2")
	char *subtitle;        ///< Monitor name subtitle
	int isSelected;        ///< This target is currently selected (1 = yes, 0 = no)
	int matchedPrefixLen;  ///< Number of matched characters at start of label
} MonitorSelectTargetData;

/// Monitor select visual style configuration.
typedef struct {
	int fontSize;              ///< Label font size
	int subtitleFontSize;      ///< Subtitle font size
	char *fontFamily;          ///< Label font family
	char *subtitleFontFamily;  ///< Subtitle font family
	int borderRadius;          ///< Badge border radius (-1 = auto)
	int paddingX;              ///< Badge horizontal padding (-1 = auto)
	int paddingY;              ///< Badge vertical padding (-1 = auto)
	int borderWidth;           ///< Badge border width
	char *backgroundColor;     ///< Badge background color
	char *textColor;           ///< Label text color
	char *matchedTextColor;    ///< Matched prefix text color
	char *borderColor;         ///< Badge border color
	char *backdropColor;       ///< Monitor backdrop tint color
	char *subtitleTextColor;   ///< Subtitle text color
	int hideInScreenShare;     ///< Hide panels from screen sharing (1 = hidden, 0 = visible)
} MonitorSelectStyle;

/// Show monitor select overlay panels on each target monitor.
/// @param targets Array of monitor select target data
/// @param count Number of targets
/// @param style Visual style configuration
void NeruShowMonitorSelectPanels(MonitorSelectTargetData *targets, int count, MonitorSelectStyle style);

/// Hide all monitor select overlay panels.
void NeruHideMonitorSelectPanels(void);

#endif  // OVERLAY_H
