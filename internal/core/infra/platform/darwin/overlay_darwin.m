//
//  overlay.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "overlay.h"

#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>

#pragma mark - HintItem Class

@interface HintItem : NSObject
@property(nonatomic, copy) NSString *label;
@property(nonatomic, assign) CGPoint position;
@property(nonatomic, assign) int matchedPrefixLength;
@property(nonatomic, assign) BOOL showArrow;
@end

@implementation HintItem

- (instancetype)init {
	self = [super init];
	if (self) {
		_showArrow = YES;
	}
	return self;
}

- (BOOL)isEqual:(id)object {
	if (self == object)
		return YES;
	if (![object isKindOfClass:[HintItem class]])
		return NO;

	HintItem *other = (HintItem *)object;
	return self.position.x == other.position.x && self.position.y == other.position.y;
}

- (NSUInteger)hash {
	// Combine x and y into a single hash using bit manipulation
	NSUInteger hx = [[NSNumber numberWithDouble:self.position.x] hash];
	NSUInteger hy = [[NSNumber numberWithDouble:self.position.y] hash];
	return hx ^ (hy * 31);
}

@end

#pragma mark - GridCellItem Class

@interface GridCellItem : NSObject
@property(nonatomic, copy) NSString *label;
@property(nonatomic, assign) CGRect bounds;
@property(nonatomic, assign) BOOL isMatched;
@property(nonatomic, assign) BOOL isSubgrid;
@property(nonatomic, assign) int matchedPrefixLength;
@end

@implementation GridCellItem

- (instancetype)init {
	self = [super init];
	if (self) {
		_isMatched = NO;
		_isSubgrid = NO;
		_matchedPrefixLength = 0;
	}
	return self;
}

- (BOOL)isEqual:(id)object {
	if (self == object)
		return YES;
	if (![object isKindOfClass:[GridCellItem class]])
		return NO;

	GridCellItem *other = (GridCellItem *)object;
	return CGRectEqualToRect(self.bounds, other.bounds);
}

- (NSUInteger)hash {
	// Combine bounds components into a single hash
	NSUInteger hx = [[NSNumber numberWithDouble:self.bounds.origin.x] hash];
	NSUInteger hy = [[NSNumber numberWithDouble:self.bounds.origin.y] hash];
	NSUInteger hw = [[NSNumber numberWithDouble:self.bounds.size.width] hash];
	NSUInteger hh = [[NSNumber numberWithDouble:self.bounds.size.height] hash];
	return hx ^ (hy * 31) ^ (hw * 127) ^ (hh * 8191);
}

@end

/// Default font size for hint overlays (bold system font).
static const CGFloat kDefaultHintFontSize = 10.0;
/// Default font size for grid overlays (regular system font).
static const CGFloat kDefaultGridFontSize = 10.0;

/// Height of the downward-pointing arrow on hint tooltips (0 when arrow is hidden).
static const CGFloat kHintArrowHeight = 1.0;
/// Width multiplier for the arrow base relative to its height.
static const CGFloat kHintArrowWidthMultiplier = 3.5;
/// Vertical gap between the arrow tip and the target element.
static const CGFloat kHintArrowGap = 1.0;

#pragma mark - Overlay View Interface

@interface OverlayView : NSView
@property(nonatomic, strong) NSMutableArray<HintItem *> *hints;  ///< Hints array
@property(nonatomic, strong) NSFont *hintFont;                   ///< Hint font
@property(nonatomic, strong) NSColor *hintTextColor;             ///< Hint text color
@property(nonatomic, strong) NSColor *hintMatchedTextColor;      ///< Hint matched text color
@property(nonatomic, strong) NSColor *hintBackgroundColor;       ///< Hint background color
@property(nonatomic, strong) NSColor *hintBorderColor;           ///< Hint border color
@property(nonatomic, assign) CGFloat hintBorderRadius;           ///< Hint border radius
@property(nonatomic, assign) CGFloat hintBorderWidth;            ///< Hint border width
@property(nonatomic, assign) CGFloat hintPaddingX;               ///< Hint horizontal padding
@property(nonatomic, assign) CGFloat hintPaddingY;               ///< Hint vertical padding

@property(nonatomic, strong) NSMutableArray<GridCellItem *> *gridCells;         ///< Grid cells array
@property(nonatomic, strong) NSArray<GridCellItem *> *transitionFromGridCells;  ///< Previous grid cells for animation
@property(nonatomic, strong) NSArray<GridCellItem *> *transitionToGridCells;    ///< Target grid cells for animation
@property(nonatomic, strong) NSTimer *gridTransitionTimer;            ///< Display timer for recursive-grid animation
@property(nonatomic, assign) CFTimeInterval gridTransitionStartTime;  ///< Animation start timestamp
@property(nonatomic, assign) CFTimeInterval gridTransitionDuration;   ///< Animation duration
@property(nonatomic, assign) BOOL gridTransitionActive;               ///< Whether recursive-grid animation is active
@property(nonatomic, assign)
    BOOL gridTransitionUseLinearEasing;               ///< Use linear easing when continuing animation (avoids stutter)
@property(nonatomic, strong) NSFont *gridFont;        ///< Grid font
@property(nonatomic, strong) NSColor *gridTextColor;  ///< Grid text color
@property(nonatomic, strong) NSColor *gridMatchedTextColor;            ///< Grid matched text color
@property(nonatomic, strong) NSColor *gridMatchedBackgroundColor;      ///< Grid matched background color
@property(nonatomic, strong) NSColor *gridMatchedBorderColor;          ///< Grid matched border color
@property(nonatomic, strong) NSColor *gridBackgroundColor;             ///< Grid background color
@property(nonatomic, strong) NSColor *gridLabelBackgroundColor;        ///< Grid label badge background color
@property(nonatomic, strong) NSColor *gridBorderColor;                 ///< Grid border color
@property(nonatomic, assign) CGFloat gridBorderWidth;                  ///< Grid border width
@property(nonatomic, assign) BOOL gridDrawLabelBackground;             ///< Draw label badge background
@property(nonatomic, assign) CGFloat gridLabelBackgroundPaddingX;      ///< Grid label badge horizontal padding
@property(nonatomic, assign) CGFloat gridLabelBackgroundPaddingY;      ///< Grid label badge vertical padding
@property(nonatomic, assign) CGFloat gridLabelBackgroundBorderRadius;  ///< Grid label badge border radius
@property(nonatomic, assign) CGFloat gridLabelBackgroundBorderWidth;   ///< Grid label badge border width
@property(nonatomic, assign) BOOL hideUnmatched;                       ///< Hide unmatched cells

// Sub-key preview: draws a miniature key grid inside each cell
@property(nonatomic, assign) BOOL gridDrawSubKeyPreview;        ///< Draw sub-key preview mini-grid
@property(nonatomic, assign) int gridSubKeyCols;                ///< Sub-key preview grid columns
@property(nonatomic, assign) int gridSubKeyRows;                ///< Sub-key preview grid rows
@property(nonatomic, strong) NSFont *gridSubKeyFont;            ///< Sub-key preview font
@property(nonatomic, strong) NSColor *gridSubKeyTextColor;      ///< Sub-key preview text color
@property(nonatomic, assign) CGFloat cachedGridSubKeyFontSize;  ///< Cached sub-key font size
@property(nonatomic, assign)
    CGFloat gridSubKeyAutohideMultiplier;  ///< Sub-key preview autohide multiplier (0 = disable)
@property(nonatomic, strong)
    NSMutableAttributedString *cachedGridSubKeyAttributedString;     ///< Cached attributed string for sub-key drawing
@property(nonatomic, strong) NSArray<NSString *> *gridSubKeyLabels;  ///< Labels for sub-key preview (next depth's keys)
@property(nonatomic, assign) BOOL cursorIndicatorVisible;            ///< Draw virtual cursor indicator
@property(nonatomic, assign) NSPoint cursorIndicatorPosition;        ///< Virtual cursor indicator center
@property(nonatomic, assign) CGFloat cursorIndicatorRadius;          ///< Virtual cursor indicator radius
@property(nonatomic, strong) NSColor *cursorIndicatorFillColor;      ///< Virtual cursor indicator fill
@property(nonatomic, assign)
    BOOL cursorIndicatorTransitionActive;  ///< Animate virtual pointer with recursive-grid transitions
@property(nonatomic, assign) NSPoint cursorIndicatorFromPosition;  ///< Previous virtual pointer position
@property(nonatomic, assign) NSPoint cursorIndicatorToPosition;    ///< Target virtual pointer position

// Cached grid text colors to reduce allocations during drawing
@property(nonatomic, strong) NSColor *cachedGridTextColor;
@property(nonatomic, strong) NSColor *cachedGridMatchedTextColor;

// Cached string buffers to reduce allocations during drawing.
// Each buffer is exclusively used by its corresponding method to avoid shared mutable state.
@property(nonatomic, strong)
    NSMutableAttributedString *cachedHintAttributedString;  ///< Cached attributed string buffer for drawHintsInRect:
@property(nonatomic, strong)
    NSMutableAttributedString *cachedHintMeasureString;  ///< Cached attributed string buffer for boundingRectForHint:
@property(nonatomic, strong) NSMutableAttributedString
    *cachedGridCellAttributedString;  ///< Cached attributed string buffer for drawGridCellsInRect:

// Cached font keys: only re-create NSFont when family or size actually changes.
@property(nonatomic, copy) NSString *cachedHintFontFamily;  ///< Last resolved hint font family
@property(nonatomic, assign) CGFloat cachedHintFontSize;    ///< Last resolved hint font size
@property(nonatomic, copy) NSString *cachedGridFontFamily;  ///< Last resolved grid font family
@property(nonatomic, assign) CGFloat cachedGridFontSize;    ///< Last resolved grid font size

/// Cached parsed colors keyed by normalized hex string
@property(nonatomic, strong) NSCache *colorCache;

/// When YES, drawLayer:inContext: clears the full bounds and redraws everything.
/// When NO, only the dirty region (clip box) is cleared and items intersecting it are redrawn.
/// Defaults to YES; set to NO by match-prefix-only updates that use setNeedsDisplayInRect:.
@property(nonatomic, assign) BOOL fullRedraw;

- (void)applyStyle:(HintStyle)style;                                                   ///< Apply hint style
- (NSColor *)colorFromHex:(NSString *)hexString defaultColor:(NSColor *)defaultColor;  ///< Color from hex string
- (CGFloat)currentBackingScaleFactor;                                                  ///< Current backing scale factor
- (NSRect)boundingRectForHint:(HintItem *)hint;            ///< Compute bounding rect for hint
- (NSRect)screenRectForGridCell:(GridCellItem *)cellItem;  ///< Compute screen-space rect for grid cell
- (void)drawGridLabel:(NSString *)label
             inCellRect:(NSRect)cellRect
              isMatched:(BOOL)isMatched
    matchedPrefixLength:(int)matchedPrefixLength;  ///< Draw grid label text or badge
- (void)drawGridLabel:(NSString *)label
             inCellRect:(NSRect)cellRect
              isMatched:(BOOL)isMatched
    matchedPrefixLength:(int)matchedPrefixLength
                  alpha:(CGFloat)alpha;                ///< Draw grid label with alpha
- (void)drawSubKeyPreviewInCellRect:(NSRect)cellRect;  ///< Draw miniature sub-key grid inside a cell
- (void)drawAnimatedGridCellsInRect:(NSRect)dirtyRect
                           progress:(CGFloat)progress;  ///< Draw interpolated recursive-grid cells
- (NSRect)cursorIndicatorRect;                          ///< Virtual cursor indicator rect in view coordinates
- (void)drawCursorIndicatorInRect:(NSRect)dirtyRect;    ///< Draw virtual cursor indicator
- (void)cancelGridTransition;                           ///< Stop recursive-grid animation
- (void)cancelCursorIndicatorTransition;                ///< Stop virtual pointer animation
- (void)startGridTransitionToCells:(NSArray<GridCellItem *> *)cells
                          duration:(CFTimeInterval)duration;  ///< Animate recursive-grid between states
- (NSArray<GridCellItem *> *)interpolatedGridCellsForProgress:(CGFloat)progress;  ///< Snapshot animated cells
- (NSColor *)color:(NSColor *)color withMultipliedAlpha:(CGFloat)alpha;  ///< Preserve configured alpha during fades
- (CGFloat)currentGridTransitionProgress;                                ///< Shared progress for grid/pointer animation
- (NSPoint)currentCursorIndicatorPosition;  ///< Current virtual pointer position for drawing

/// Resolve a font by name (accepts both PostScript names and family names).
/// Tries [NSFont fontWithName:] first, then NSFontManager family lookup.
/// Returns nil if the name cannot be resolved.
- (NSFont *)resolveFont:(NSString *)name size:(CGFloat)size bold:(BOOL)bold;

/// Resolve horizontal hint padding (-1 = auto based on font size).
- (CGFloat)resolvedHintPaddingX;

/// Resolve vertical hint padding (-1 = auto based on font size).
- (CGFloat)resolvedHintPaddingY;
@end

#pragma mark - Overlay View Implementation

@implementation OverlayView

/// Initialize with frame
/// @param frame View frame
/// @return Initialized instance
- (instancetype)initWithFrame:(NSRect)frame {
	self = [super initWithFrame:frame];
	if (self) {
		// Enable layer-backed rendering for GPU acceleration
		[self setWantsLayer:YES];
		self.layer.opaque = NO;
		self.layer.backgroundColor = [[NSColor clearColor] CGColor];
		self.layer.contentsScale = [self currentBackingScaleFactor];

		_colorCache = [[NSCache alloc] init];
		_colorCache.countLimit = 64;

		_hints = [NSMutableArray arrayWithCapacity:100];      // Pre-size for typical hint count
		_gridCells = [NSMutableArray arrayWithCapacity:100];  // Pre-size for typical grid size

		// Hint defaults
		_hintFont = [NSFont boldSystemFontOfSize:kDefaultHintFontSize];
		_hintTextColor = [NSColor blackColor];
		_hintMatchedTextColor = [NSColor systemBlueColor];
		_hintBackgroundColor = [[NSColor colorWithRed:1.0 green:0.84 blue:0.0 alpha:1.0] colorWithAlphaComponent:0.95];
		_hintBorderColor = [NSColor blackColor];
		_hintBorderRadius = -1.0;
		_hintBorderWidth = 1.0;
		_hintPaddingX = -1.0;
		_hintPaddingY = -1.0;

		// Grid defaults
		_gridFont = [NSFont systemFontOfSize:kDefaultGridFontSize];
		_gridTextColor = [NSColor colorWithWhite:0.2 alpha:1.0];
		_gridMatchedTextColor = [NSColor colorWithRed:0.0 green:0.4 blue:1.0 alpha:1.0];
		_gridBackgroundColor = [NSColor whiteColor];
		_gridLabelBackgroundColor = [[NSColor colorWithRed:1.0 green:0.84 blue:0.0
		                                             alpha:1.0] colorWithAlphaComponent:0.8];
		_gridBorderColor = [NSColor colorWithWhite:0.7 alpha:1.0];
		_gridBorderWidth = 1.0;
		_gridDrawLabelBackground = NO;
		_gridLabelBackgroundPaddingX = -1.0;
		_gridLabelBackgroundPaddingY = -1.0;
		_gridLabelBackgroundBorderRadius = -1.0;
		_gridLabelBackgroundBorderWidth = 1.0;
		_gridSubKeyAutohideMultiplier = 1.5;
		_hideUnmatched = NO;
		_cursorIndicatorVisible = NO;
		_cursorIndicatorRadius = 3.0;
		_cursorIndicatorFillColor = [NSColor colorWithWhite:1.0 alpha:1.0];
		_cursorIndicatorTransitionActive = NO;
		_cursorIndicatorFromPosition = NSZeroPoint;
		_cursorIndicatorToPosition = NSZeroPoint;

		// Initialize cached colors
		_cachedGridTextColor = _gridTextColor;
		_cachedGridMatchedTextColor = _gridMatchedTextColor;

		// Initialize cached string buffers
		_cachedHintAttributedString = [[NSMutableAttributedString alloc] initWithString:@""];
		_cachedHintMeasureString = [[NSMutableAttributedString alloc] initWithString:@""];
		_cachedGridCellAttributedString = [[NSMutableAttributedString alloc] initWithString:@""];
		_cachedGridSubKeyAttributedString = [[NSMutableAttributedString alloc] initWithString:@""];

		// Initialize cached font keys (match defaults above)
		_cachedHintFontFamily = nil;
		_cachedHintFontSize = kDefaultHintFontSize;
		_cachedGridFontFamily = nil;
		_cachedGridFontSize = kDefaultGridFontSize;

		// Initialize fullRedraw to YES for structural changes
		_fullRedraw = YES;
		_gridTransitionDuration = 0.18;
		_gridTransitionStartTime = 0;
		_gridTransitionActive = NO;
	}
	return self;
}

/// Return the backing scale factor for the current screen, with fallbacks.
/// Uses the window's actual screen (not mainScreen) to ensure correct rendering
/// when the overlay moves between displays with different scale factors
/// (e.g., Retina vs non-Retina). Falls back to mainScreen, then 1.0.
- (CGFloat)currentBackingScaleFactor {
	CGFloat scale = self.window.screen.backingScaleFactor;
	if (scale == 0) {
		scale = [NSScreen mainScreen].backingScaleFactor;
	}
	return scale > 0 ? scale : 1.0;
}

/// Set view frame and update contents scale for high-DPI displays
/// @param frame New frame
- (void)setFrame:(NSRect)frame {
	[super setFrame:frame];
	if (self.layer) {
		self.layer.contentsScale = [self currentBackingScaleFactor];
	}
}

/// Update contents scale when the view moves between screens with different
/// backing properties (e.g., Retina to non-Retina or vice versa).
/// This is the Apple-recommended callback for responding to scale factor changes.
- (void)viewDidChangeBackingProperties {
	[super viewDidChangeBackingProperties];
	if (self.layer) {
		self.layer.contentsScale = [self currentBackingScaleFactor];
	}
}

/// Required: AppKit uses the presence of drawRect: to determine that this
/// view has custom drawing content. Without it, setNeedsDisplay:YES may not
/// trigger layer redisplay. Actual rendering is handled by drawLayer:inContext:.
/// @param dirtyRect Dirty rectangle (unused)
- (void)drawRect:(NSRect)dirtyRect {
}

/// Draw layer (GPU-accelerated rendering for layer-backed views).
/// When fullRedraw is YES (structural changes), clears entire bounds and redraws all items.
/// When fullRedraw is NO (match-prefix-only changes), uses the CGContext clip box to
/// clear and redraw only the dirty regions, skipping items outside the dirty area.
/// @param layer Layer
/// @param ctx Graphics context
- (void)drawLayer:(CALayer *)layer inContext:(CGContextRef)ctx {
	[NSGraphicsContext saveGraphicsState];
	NSGraphicsContext *nsContext = [NSGraphicsContext graphicsContextWithCGContext:ctx flipped:NO];
	[NSGraphicsContext setCurrentContext:nsContext];

	if (self.gridTransitionActive) {
		CGFloat duration = self.gridTransitionDuration > 0 ? self.gridTransitionDuration : 0.18;
		CFTimeInterval elapsed = CACurrentMediaTime() - self.gridTransitionStartTime;
		CGFloat rawProgress = (CGFloat)(elapsed / duration);
		CGFloat progress = [self currentGridTransitionProgress];

		CGContextClearRect(ctx, self.bounds);
		[self drawAnimatedGridCellsInRect:NSZeroRect progress:progress];
		[self drawHints];
		[self drawCursorIndicatorInRect:NSZeroRect];

		if (rawProgress >= 1.0) {
			[self cancelGridTransition];
			[self cancelCursorIndicatorTransition];
			// The final animated frame may still include interpolated cells that
			// only existed in the previous layout (for example when shrinking from
			// a dense grid to fewer cells). Force one clean redraw in the settled
			// state so the overlay reflects only the target grid.
			[self setNeedsDisplay:YES];
		}

		self.fullRedraw = YES;
		[NSGraphicsContext restoreGraphicsState];

		return;
	}

	if (self.fullRedraw) {
		// Full redraw: clear everything and draw all items
		CGContextClearRect(ctx, self.bounds);
		[self drawGridCells];
		[self drawHints];
		[self drawCursorIndicatorInRect:NSZeroRect];
	} else {
		// Partial redraw: only clear and redraw items intersecting the dirty region.
		// Core Animation sets the clip to the union of invalidated rects.
		CGRect clipBox = CGContextGetClipBoundingBox(ctx);
		NSRect dirtyRect = NSRectFromCGRect(clipBox);

		if (NSContainsRect(dirtyRect, self.bounds)) {
			// If the clip box covers the full bounds, fall back to full redraw
			CGContextClearRect(ctx, self.bounds);
			[self drawGridCells];
			[self drawHints];
			[self drawCursorIndicatorInRect:NSZeroRect];
		} else {
			// Clear only the dirty region
			CGContextClearRect(ctx, clipBox);
			[self drawGridCellsInRect:dirtyRect];
			[self drawHintsInRect:dirtyRect];
			[self drawCursorIndicatorInRect:dirtyRect];
		}
	}

	// Reset to full redraw for next cycle; partial-redraw callers
	// set this to NO before calling setNeedsDisplayInRect:
	self.fullRedraw = YES;

	[NSGraphicsContext restoreGraphicsState];
}

- (void)cancelGridTransition {
	[self.gridTransitionTimer invalidate];
	self.gridTransitionTimer = nil;
	self.gridTransitionActive = NO;
	self.gridTransitionUseLinearEasing = NO;
	self.transitionFromGridCells = nil;
	self.transitionToGridCells = nil;
}

- (void)cancelCursorIndicatorTransition {
	self.cursorIndicatorTransitionActive = NO;
	self.cursorIndicatorFromPosition = NSZeroPoint;
	self.cursorIndicatorToPosition = NSZeroPoint;
}

- (NSColor *)color:(NSColor *)color withMultipliedAlpha:(CGFloat)alpha {
	if (!color) {
		return nil;
	}

	NSColor *resolvedColor = [color colorUsingColorSpace:[NSColorSpace deviceRGBColorSpace]];
	if (!resolvedColor) {
		resolvedColor = color;
	}

	return [resolvedColor colorWithAlphaComponent:(resolvedColor.alphaComponent * alpha)];
}

- (CGFloat)currentGridTransitionProgress {
	if (!self.gridTransitionActive) {
		return 1.0;
	}

	CGFloat duration = self.gridTransitionDuration > 0 ? self.gridTransitionDuration : 0.18;
	CFTimeInterval elapsed = CACurrentMediaTime() - self.gridTransitionStartTime;
	CGFloat rawProgress = MIN(MAX((CGFloat)(elapsed / duration), 0.0), 1.0);

	if (self.gridTransitionUseLinearEasing) {
		return rawProgress;
	}

	CAMediaTimingFunction *timingFunction =
	    [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
	float controlPoints[8];
	[timingFunction getControlPointAtIndex:1 values:&controlPoints[0]];
	[timingFunction getControlPointAtIndex:2 values:&controlPoints[2]];
	CGFloat t = rawProgress;
	CGFloat oneMinusT = 1.0 - t;

	return 3.0 * oneMinusT * oneMinusT * t * controlPoints[1] + 3.0 * oneMinusT * t * t * controlPoints[3] + t * t * t;
}

- (NSPoint)currentCursorIndicatorPosition {
	if (!self.cursorIndicatorTransitionActive) {
		return self.cursorIndicatorPosition;
	}

	CGFloat progress = [self currentGridTransitionProgress];

	return NSMakePoint(
	    self.cursorIndicatorFromPosition.x +
	        (self.cursorIndicatorToPosition.x - self.cursorIndicatorFromPosition.x) * progress,
	    self.cursorIndicatorFromPosition.y +
	        (self.cursorIndicatorToPosition.y - self.cursorIndicatorFromPosition.y) * progress);
}

- (NSArray<GridCellItem *> *)interpolatedGridCellsForProgress:(CGFloat)progress {
	NSArray<GridCellItem *> *fromCells = self.transitionFromGridCells ?: @[];
	NSArray<GridCellItem *> *toCells = self.transitionToGridCells ?: @[];
	NSUInteger count = MAX([fromCells count], [toCells count]);
	if (count == 0) {
		return @[];
	}

	CGRect fromBounds = CGRectNull;
	for (GridCellItem *cell in fromCells) {
		fromBounds = CGRectIsNull(fromBounds) ? cell.bounds : CGRectUnion(fromBounds, cell.bounds);
	}

	CGRect toBounds = CGRectNull;
	for (GridCellItem *cell in toCells) {
		toBounds = CGRectIsNull(toBounds) ? cell.bounds : CGRectUnion(toBounds, cell.bounds);
	}

	if (CGRectIsNull(fromBounds)) {
		fromBounds = CGRectIsNull(toBounds) ? CGRectZero : toBounds;
	}
	if (CGRectIsNull(toBounds)) {
		toBounds = fromBounds;
	}

	NSMutableArray<GridCellItem *> *cells = [NSMutableArray arrayWithCapacity:count];
	for (NSUInteger idx = 0; idx < count; idx++) {
		GridCellItem *fromCell = idx < [fromCells count] ? fromCells[idx] : nil;
		GridCellItem *toCell = idx < [toCells count] ? toCells[idx] : nil;

		CGRect startRect = fromCell ? fromCell.bounds : fromBounds;
		CGRect endRect = toCell ? toCell.bounds : toBounds;

		GridCellItem *cell = [[GridCellItem alloc] init];
		cell.label = toCell ? toCell.label : fromCell.label;
		cell.isMatched = toCell ? toCell.isMatched : fromCell.isMatched;
		cell.isSubgrid = toCell ? toCell.isSubgrid : fromCell.isSubgrid;
		cell.matchedPrefixLength = toCell ? toCell.matchedPrefixLength : fromCell.matchedPrefixLength;
		cell.bounds = CGRectMake(
		    startRect.origin.x + (endRect.origin.x - startRect.origin.x) * progress,
		    startRect.origin.y + (endRect.origin.y - startRect.origin.y) * progress,
		    startRect.size.width + (endRect.size.width - startRect.size.width) * progress,
		    startRect.size.height + (endRect.size.height - startRect.size.height) * progress);
		[cells addObject:cell];
	}

	return cells;
}

- (void)startGridTransitionToCells:(NSArray<GridCellItem *> *)cells duration:(CFTimeInterval)duration {
	if ([cells count] == 0 || [self.gridCells count] == 0 || duration <= 0) {
		[self cancelGridTransition];
		[self cancelCursorIndicatorTransition];
		self.gridCells = [cells mutableCopy];
		[self setNeedsDisplay:YES];
		return;
	}

	NSArray<GridCellItem *> *fromCells = nil;
	NSPoint currentCursorPosition = NSZeroPoint;
	BOOL shouldPreserveCursorPosition = self.cursorIndicatorVisible;
	BOOL continuingFromActive = self.gridTransitionActive;
	BOOL cellCountChanged = [cells count] != [self.gridCells count];
	if (self.gridTransitionActive) {
		CGFloat existingDuration = self.gridTransitionDuration > 0 ? self.gridTransitionDuration : 0.18;
		CFTimeInterval elapsed = CACurrentMediaTime() - self.gridTransitionStartTime;
		CGFloat progress = MIN(MAX((CGFloat)(elapsed / existingDuration), 0.0), 1.0);
		fromCells = [self interpolatedGridCellsForProgress:progress];
	} else {
		fromCells = [self.gridCells copy];
	}
	if (shouldPreserveCursorPosition) {
		currentCursorPosition = [self currentCursorIndicatorPosition];
	}

	if (cellCountChanged) {
		CGRect sourceBounds = CGRectNull;
		for (GridCellItem *cell in fromCells) {
			sourceBounds = CGRectIsNull(sourceBounds) ? cell.bounds : CGRectUnion(sourceBounds, cell.bounds);
		}
		CGRect targetBounds = CGRectNull;
		for (GridCellItem *cell in cells) {
			targetBounds = CGRectIsNull(targetBounds) ? cell.bounds : CGRectUnion(targetBounds, cell.bounds);
		}
		if (CGRectIsNull(sourceBounds) || CGRectGetWidth(sourceBounds) <= 0 || CGRectGetHeight(sourceBounds) <= 0) {
			sourceBounds = CGRectIsNull(targetBounds) ? CGRectZero : targetBounds;
		}
		if (CGRectIsNull(targetBounds) || CGRectGetWidth(targetBounds) <= 0 || CGRectGetHeight(targetBounds) <= 0) {
			targetBounds = sourceBounds;
		}

		NSMutableArray<GridCellItem *> *syntheticFromCells = [NSMutableArray arrayWithCapacity:[cells count]];
		for (GridCellItem *cell in cells) {
			CGRect endRect = cell.bounds;
			CGFloat relMinX = (CGRectGetMinX(endRect) - CGRectGetMinX(targetBounds)) / CGRectGetWidth(targetBounds);
			CGFloat relMinY = (CGRectGetMinY(endRect) - CGRectGetMinY(targetBounds)) / CGRectGetHeight(targetBounds);
			CGFloat relWidth = CGRectGetWidth(endRect) / CGRectGetWidth(targetBounds);
			CGFloat relHeight = CGRectGetHeight(endRect) / CGRectGetHeight(targetBounds);
			CGRect startRect = CGRectMake(
			    CGRectGetMinX(sourceBounds) + relMinX * CGRectGetWidth(sourceBounds),
			    CGRectGetMinY(sourceBounds) + relMinY * CGRectGetHeight(sourceBounds),
			    relWidth * CGRectGetWidth(sourceBounds), relHeight * CGRectGetHeight(sourceBounds));

			GridCellItem *fromCell = [[GridCellItem alloc] init];
			fromCell.label = cell.label;
			fromCell.isMatched = NO;
			fromCell.isSubgrid = NO;
			fromCell.matchedPrefixLength = 0;
			fromCell.bounds = startRect;
			[syntheticFromCells addObject:fromCell];
		}
		fromCells = syntheticFromCells;
		continuingFromActive = NO;
	}

	[self cancelGridTransition];
	if (shouldPreserveCursorPosition) {
		self.cursorIndicatorPosition = currentCursorPosition;
		[self cancelCursorIndicatorTransition];
	}

	self.transitionFromGridCells = fromCells;
	self.transitionToGridCells = [cells copy];
	self.gridCells = [cells mutableCopy];
	self.gridTransitionDuration = duration;
	self.gridTransitionStartTime = CACurrentMediaTime();
	self.gridTransitionActive = YES;
	self.gridTransitionUseLinearEasing = continuingFromActive;
	self.fullRedraw = YES;

	__weak typeof(self) weakSelf = self;
	self.gridTransitionTimer = [NSTimer timerWithTimeInterval:(1.0 / 120.0)
	                                                  repeats:YES
	                                                    block:^(__unused NSTimer *timer) {
		                                                    OverlayView *strongSelf = weakSelf;
		                                                    if (!strongSelf)
			                                                    return;
		                                                    strongSelf.fullRedraw = YES;
		                                                    [strongSelf setNeedsDisplay:YES];
	                                                    }];
	[[NSRunLoop mainRunLoop] addTimer:self.gridTransitionTimer forMode:NSRunLoopCommonModes];
	[self setNeedsDisplay:YES];
}

/// Apply hint style
/// @param style Hint style
- (void)applyStyle:(HintStyle)style {
	// Font resolution — only re-create when family or size actually changed
	CGFloat fontSize = style.fontSize > 0 ? style.fontSize : kDefaultHintFontSize;
	NSString *fontFamily = nil;
	if (style.fontFamily) {
		fontFamily = [NSString stringWithUTF8String:style.fontFamily];
		if (fontFamily.length == 0)
			fontFamily = nil;
	}

	BOOL familyChanged =
	    (fontFamily != self.cachedHintFontFamily && ![fontFamily isEqualToString:self.cachedHintFontFamily]);
	if (familyChanged || fontSize != self.cachedHintFontSize) {
		NSFont *font = fontFamily.length > 0 ? [self resolveFont:fontFamily size:fontSize bold:YES] : nil;
		if (!font)
			font = [NSFont boldSystemFontOfSize:fontSize];
		self.hintFont = font;
		self.cachedHintFontFamily = fontFamily;
		self.cachedHintFontSize = fontSize;
	}

	// Color defaults
	NSColor *defaultBg = [[NSColor colorWithRed:1.0 green:0.84 blue:0.0 alpha:1.0] colorWithAlphaComponent:0.95];
	NSColor *defaultText = [NSColor blackColor];
	NSColor *defaultMatchedText = [NSColor systemBlueColor];
	NSColor *defaultBorder = [NSColor blackColor];

	// Parse hex color strings
	NSString *backgroundHex = style.backgroundColor ? [NSString stringWithUTF8String:style.backgroundColor] : nil;
	NSString *textHex = style.textColor ? [NSString stringWithUTF8String:style.textColor] : nil;
	NSString *matchedTextHex = style.matchedTextColor ? [NSString stringWithUTF8String:style.matchedTextColor] : nil;
	NSString *borderHex = style.borderColor ? [NSString stringWithUTF8String:style.borderColor] : nil;

	// Apply colors
	self.hintBackgroundColor = [self colorFromHex:backgroundHex defaultColor:defaultBg];
	self.hintTextColor = [self colorFromHex:textHex defaultColor:defaultText];
	self.hintMatchedTextColor = [self colorFromHex:matchedTextHex defaultColor:defaultMatchedText];
	self.hintBorderColor = [self colorFromHex:borderHex defaultColor:defaultBorder];

	// Apply geometry properties
	self.hintBorderRadius = style.borderRadius;
	self.hintBorderWidth = style.borderWidth >= 0 ? style.borderWidth : 1.0;
	self.hintPaddingX = style.paddingX;
	self.hintPaddingY = style.paddingY;
}

/// Create color from hex string
/// @param hexString Hex color string
/// @param defaultColor Default color
/// @return NSColor instance
- (NSColor *)colorFromHex:(NSString *)hexString defaultColor:(NSColor *)defaultColor {
	if (!hexString || hexString.length == 0)
		return defaultColor;

	// Normalise the input string
	NSString *cleanString =
	    [hexString stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
	if ([cleanString hasPrefix:@"#"])
		cleanString = [cleanString substringFromIndex:1];
	cleanString = [cleanString lowercaseString];

	// Expand 3-char shorthand to 6-char for consistent cache keys (e.g. f0a -> ff00aa)
	NSString *cacheKey = cleanString;
	if (cacheKey.length == 3) {
		cacheKey =
		    [NSString stringWithFormat:@"%c%c%c%c%c%c", [cacheKey characterAtIndex:0], [cacheKey characterAtIndex:0],
		                               [cacheKey characterAtIndex:1], [cacheKey characterAtIndex:1],
		                               [cacheKey characterAtIndex:2], [cacheKey characterAtIndex:2]];
	}

	// Cache lookup
	NSColor *cachedColor = [self.colorCache objectForKey:cacheKey];
	if (cachedColor)
		return cachedColor;

	// Validate length and parse hex value
	cleanString = cacheKey;
	if (cleanString.length != 6 && cleanString.length != 8)
		return defaultColor;

	unsigned long long hexValue = 0;
	NSScanner *scanner = [NSScanner scannerWithString:cleanString];
	if (![scanner scanHexLongLong:&hexValue])
		return defaultColor;

	// Extract RGBA components
	CGFloat alpha = 1.0;
	if (cleanString.length == 8)
		alpha = ((hexValue & 0xFF000000) >> 24) / 255.0;
	CGFloat red = ((hexValue & 0x00FF0000) >> 16) / 255.0;
	CGFloat green = ((hexValue & 0x0000FF00) >> 8) / 255.0;
	CGFloat blue = (hexValue & 0x000000FF) / 255.0;

	NSColor *result = [NSColor colorWithRed:red green:green blue:blue alpha:alpha];
	[self.colorCache setObject:result forKey:cacheKey];
	return result;
}

/// Resolve a font by name, accepting both PostScript names (e.g. "SFMono-Bold")
/// and family/display names (e.g. "SF Mono", "JetBrains Mono").
/// Tries [NSFont fontWithName:] first (PostScript name lookup), then falls back
/// to NSFontManager family lookup which handles display/family names correctly.
/// @param name Font name (PostScript or family)
/// @param size Font size
/// @param bold Whether to prefer a bold variant
/// @return Resolved NSFont, or nil if the name cannot be resolved
- (NSFont *)resolveFont:(NSString *)name size:(CGFloat)size bold:(BOOL)bold {
	if (!name || name.length == 0)
		return nil;

	NSFontManager *fm = [NSFontManager sharedFontManager];

	// Try PostScript name first (fast path)
	NSFont *font = [NSFont fontWithName:name size:size];
	if (!font) {
		// Fall back to family name lookup via NSFontManager
		NSFontTraitMask traits = bold ? NSBoldFontMask : 0;
		NSInteger weight = bold ? 9 : 5;  // 9 = bold, 5 = regular in AppKit weight scale
		font = [fm fontWithFamily:name traits:traits weight:weight size:size];
	}

	if (font && bold) {
		// Verify the resolved font actually has the bold trait, regardless of
		// whether it came from the PostScript path or the family-name path.
		// A user-supplied PostScript name like "SFMono-Regular" would otherwise
		// bypass bold enforcement; NSFontManager family lookup may also return
		// a lighter weight (e.g. Medium instead of Bold).
		NSFontTraitMask actualTraits = [fm traitsOfFont:font];
		if (!(actualTraits & NSBoldFontMask)) {
			// convertFont:toHaveTrait: never returns nil per Apple docs —
			// it returns the original font unchanged if the trait cannot be added.
			// We must re-check traits to know whether the conversion succeeded.
			NSFont *boldFont = [fm convertFont:font toHaveTrait:NSBoldFontMask];
			NSFontTraitMask boldTraits = [fm traitsOfFont:boldFont];
			if (boldTraits & NSBoldFontMask) {
				font = boldFont;
			} else {
				NSLog(@"[Neru] Font family \"%@\" has no bold variant; using regular weight", name);
			}
		}
	}

	return font;
}

/// Resolve horizontal hint padding.
/// Returns hintPaddingX if >= 0, otherwise auto-computes from font size.
- (CGFloat)resolvedHintPaddingX {
	return self.hintPaddingX >= 0.0 ? self.hintPaddingX : MAX(4.0, round(self.hintFont.pointSize * 0.4));
}

/// Resolve vertical hint padding.
/// Returns hintPaddingY if >= 0, otherwise auto-computes from font size.
- (CGFloat)resolvedHintPaddingY {
	return self.hintPaddingY >= 0.0 ? self.hintPaddingY : MAX(2.0, round(self.hintFont.pointSize * 0.2));
}

/// Create tooltip path with arrow
/// @param rect Tooltip rectangle
/// @param arrowSize Arrow size
/// @param elementCenterX Element center X
/// @param elementCenterY Element center Y
/// @return NSBezierPath instance
- (NSBezierPath *)createTooltipPath:(NSRect)rect
                          arrowSize:(CGFloat)arrowSize
                     elementCenterX:(CGFloat)elementCenterX
                     elementCenterY:(CGFloat)elementCenterY {
	NSBezierPath *path = [NSBezierPath bezierPath];

	// Tooltip body rectangle (excluding arrow space)
	NSRect bodyRect = NSMakeRect(rect.origin.x, rect.origin.y, rect.size.width, rect.size.height - arrowSize);

	// Resolve border radius (-1 = auto pill)
	CGFloat radius = self.hintBorderRadius >= 0.0 ? self.hintBorderRadius : MIN(bodyRect.size.height / 2.0, 6.0);

	// Arrow dimensions
	CGFloat arrowTipX = elementCenterX;
	CGFloat arrowTipY = elementCenterY;
	CGFloat arrowBaseY = bodyRect.origin.y + bodyRect.size.height;
	CGFloat arrowWidth = arrowSize * kHintArrowWidthMultiplier;
	CGFloat arrowLeft = arrowTipX - arrowWidth / 2;
	CGFloat arrowRight = arrowTipX + arrowWidth / 2;

	// Clamp arrow to tooltip bounds
	CGFloat tooltipLeft = bodyRect.origin.x + radius;
	CGFloat tooltipRight = bodyRect.origin.x + bodyRect.size.width - radius;
	arrowLeft = MAX(arrowLeft, tooltipLeft);
	arrowRight = MIN(arrowRight, tooltipRight);
	arrowTipX = (arrowLeft + arrowRight) / 2;

	// Build path clockwise from top-left corner
	[path moveToPoint:NSMakePoint(bodyRect.origin.x + radius, bodyRect.origin.y)];

	// Top edge → top-right corner
	[path lineToPoint:NSMakePoint(bodyRect.origin.x + bodyRect.size.width - radius, bodyRect.origin.y)];
	[path appendBezierPathWithArcFromPoint:NSMakePoint(bodyRect.origin.x + bodyRect.size.width, bodyRect.origin.y)
	                               toPoint:NSMakePoint(
	                                           bodyRect.origin.x + bodyRect.size.width, bodyRect.origin.y + radius)
	                                radius:radius];

	// Right edge → bottom-right corner
	[path lineToPoint:NSMakePoint(bodyRect.origin.x + bodyRect.size.width, arrowBaseY - radius)];
	[path appendBezierPathWithArcFromPoint:NSMakePoint(bodyRect.origin.x + bodyRect.size.width, arrowBaseY)
	                               toPoint:NSMakePoint(bodyRect.origin.x + bodyRect.size.width - radius, arrowBaseY)
	                                radius:radius];

	// Bottom edge → arrow
	[path lineToPoint:NSMakePoint(arrowRight, arrowBaseY)];
	[path lineToPoint:NSMakePoint(arrowTipX, arrowTipY)];
	[path lineToPoint:NSMakePoint(arrowLeft, arrowBaseY)];

	// Continue bottom edge → bottom-left corner
	[path lineToPoint:NSMakePoint(bodyRect.origin.x + radius, arrowBaseY)];
	[path appendBezierPathWithArcFromPoint:NSMakePoint(bodyRect.origin.x, arrowBaseY)
	                               toPoint:NSMakePoint(bodyRect.origin.x, arrowBaseY - radius)
	                                radius:radius];

	// Left edge → top-left corner
	[path lineToPoint:NSMakePoint(bodyRect.origin.x, bodyRect.origin.y + radius)];
	[path appendBezierPathWithArcFromPoint:NSMakePoint(bodyRect.origin.x, bodyRect.origin.y)
	                               toPoint:NSMakePoint(bodyRect.origin.x + radius, bodyRect.origin.y)
	                                radius:radius];

	[path closePath];
	return path;
}

/// Draw all hint labels above target elements.
/// Delegates to drawHintsInRect: with NSZeroRect to signal "draw all, skip intersection checks".
- (void)drawHints {
	[self drawHintsInRect:NSZeroRect];
}

/// Draw all grid cells with labels and borders.
/// Delegates to drawGridCellsInRect: with NSZeroRect to signal "draw all, skip intersection checks".
- (void)drawGridCells {
	[self drawGridCellsInRect:NSZeroRect];
}

/// Compute the screen-space bounding rect for the virtual cursor indicator.
/// @return Bounding rectangle for the dot plus a small antialiasing margin
- (NSRect)cursorIndicatorRect {
	if (!self.cursorIndicatorVisible)
		return NSZeroRect;

	NSPoint position = [self currentCursorIndicatorPosition];
	CGFloat diameter = self.cursorIndicatorRadius * 2.0;
	CGFloat screenHeight = self.bounds.size.height;
	CGFloat flippedY = screenHeight - position.y - self.cursorIndicatorRadius;
	CGFloat expand = 1.0;
	return NSMakeRect(
	    position.x - self.cursorIndicatorRadius - expand, flippedY - expand, diameter + expand * 2.0,
	    diameter + expand * 2.0);
}

/// Draw the virtual cursor indicator when it intersects the dirty region.
/// @param dirtyRect Dirty region to redraw. Pass NSZeroRect to draw unconditionally.
- (void)drawCursorIndicatorInRect:(NSRect)dirtyRect {
	if (!self.cursorIndicatorVisible)
		return;

	NSRect indicatorRect = [self cursorIndicatorRect];
	BOOL filterByRect = !NSIsEmptyRect(dirtyRect);
	if (filterByRect && !NSIntersectsRect(indicatorRect, dirtyRect))
		return;

	NSPoint position = [self currentCursorIndicatorPosition];
	CGFloat screenHeight = self.bounds.size.height;
	CGFloat centerY = screenHeight - position.y;
	NSPoint center = NSMakePoint(position.x, centerY);
	NSBezierPath *dot = [NSBezierPath
	    bezierPathWithOvalInRect:NSMakeRect(
	                                 center.x - self.cursorIndicatorRadius, center.y - self.cursorIndicatorRadius,
	                                 self.cursorIndicatorRadius * 2.0, self.cursorIndicatorRadius * 2.0)];
	[self.cursorIndicatorFillColor setFill];
	[dot fill];
}

/// Compute the screen-space bounding rect for a hint item (view coordinates, bottom-left origin).
/// Mirrors the geometry logic in drawHintsInRect: so callers can determine dirty rects without drawing.
/// Uses cachedHintMeasureString (a dedicated buffer separate from cachedHintAttributedString)
/// to avoid allocations while not mutating the buffer used by drawHintsInRect:.
/// @param hint Hint item
/// @return Bounding rectangle including border and arrow
- (NSRect)boundingRectForHint:(HintItem *)hint {
	NSString *label = hint.label;
	if (!label || [label length] == 0)
		return NSZeroRect;

	// Reuse cachedHintMeasureString for text measurement.
	// This is a separate buffer from cachedHintAttributedString (used by drawHintsInRect:)
	// so the two methods can safely call each other without corrupting shared state.
	NSMutableAttributedString *measureString = self.cachedHintMeasureString;
	[[measureString mutableString] setString:label];
	NSRange fullRange = NSMakeRange(0, [label length]);
	[measureString
	    setAttributes:@{NSFontAttributeName : self.hintFont, NSForegroundColorAttributeName : self.hintTextColor}
	            range:fullRange];

	// Compute geometry
	NSSize textSize = [measureString size];
	CGFloat paddingX = [self resolvedHintPaddingX];
	CGFloat paddingY = [self resolvedHintPaddingY];
	CGFloat arrowHeight = hint.showArrow ? kHintArrowHeight : 0.0;
	CGFloat contentWidth = textSize.width + (paddingX * 2);
	CGFloat contentHeight = textSize.height + (paddingY * 2);
	CGFloat boxWidth = MAX(contentWidth, contentHeight);
	CGFloat boxHeight = contentHeight + arrowHeight;
	NSPoint position = hint.position;
	CGFloat elementCenterX = position.x;
	CGFloat elementCenterY = position.y;
	CGFloat tooltipX = elementCenterX - boxWidth / 2.0;
	CGFloat tooltipY = elementCenterY + arrowHeight + kHintArrowGap;
	CGFloat screenHeight = self.bounds.size.height;
	CGFloat flippedY = screenHeight - tooltipY - boxHeight;

	// Expand by border width + 1pt to cover anti-aliased stroke edges
	CGFloat expand = ceil(self.hintBorderWidth / 2.0) + 1.0;
	NSRect hintRect = NSMakeRect(tooltipX - expand, flippedY - expand, boxWidth + expand * 2, boxHeight + expand * 2);

	// Extend upward to include arrow tip if present
	if (hint.showArrow) {
		CGFloat flippedElementCenterY = screenHeight - elementCenterY;
		if (flippedElementCenterY > NSMaxY(hintRect)) {
			hintRect.size.height = flippedElementCenterY + 1.0 - hintRect.origin.y;
		}
	}

	return hintRect;
}

/// Compute the screen-space bounding rect for a grid cell item (view coordinates).
/// @param cellItem Grid cell item
/// @return Bounding rectangle including border stroke
- (NSRect)screenRectForGridCell:(GridCellItem *)cellItem {
	CGRect bounds = cellItem.bounds;
	CGFloat screenHeight = self.bounds.size.height;
	CGFloat flippedY = screenHeight - bounds.origin.y - bounds.size.height;

	CGFloat maxBorder = self.gridBorderWidth;
	if (self.gridDrawLabelBackground && self.gridLabelBackgroundBorderWidth > maxBorder) {
		maxBorder = self.gridLabelBackgroundBorderWidth;
	}

	CGFloat expand = ceil(maxBorder / 2.0) + 1.0;
	return NSMakeRect(
	    bounds.origin.x - expand, flippedY - expand, bounds.size.width + expand * 2, bounds.size.height + expand * 2);
}

/// Draw hint labels whose bounding rects intersect the given dirty rect.
/// This is the single implementation of hint drawing; drawHints delegates here.
/// When filtering is active, the intersection test is performed inline using the
/// same geometry computed for drawing, so text is measured only once per hint.
/// @param dirtyRect The dirty region to redraw. Pass NSZeroRect to draw all items (skips intersection checks).
- (void)drawHintsInRect:(NSRect)dirtyRect {
	BOOL filterByRect = !NSIsEmptyRect(dirtyRect);
	CGFloat screenHeight = self.bounds.size.height;
	CGFloat paddingX = [self resolvedHintPaddingX];
	CGFloat paddingY = [self resolvedHintPaddingY];

	for (HintItem *hint in self.hints) {
		NSString *label = hint.label;
		if (!label || [label length] == 0)
			continue;

		NSPoint position = hint.position;
		int matchedPrefixLength = hint.matchedPrefixLength;
		BOOL showArrow = hint.showArrow;

		// Set up attributed string with base font and colors
		NSMutableAttributedString *attrString = self.cachedHintAttributedString;
		[[attrString mutableString] setString:label];
		NSRange fullRange = NSMakeRange(0, [label length]);
		[attrString
		    setAttributes:@{NSFontAttributeName : self.hintFont, NSForegroundColorAttributeName : self.hintTextColor}
		            range:fullRange];
		if (matchedPrefixLength > 0 && matchedPrefixLength <= [label length]) {
			[attrString addAttribute:NSForegroundColorAttributeName
			                   value:self.hintMatchedTextColor
			                   range:NSMakeRange(0, matchedPrefixLength)];
		}

		// Compute geometry once — used for both intersection test and drawing,
		// avoiding the double text measurement that would occur if we called
		// boundingRectForHint: separately for the intersection check.
		NSSize textSize = [attrString size];
		CGFloat arrowHeight = showArrow ? kHintArrowHeight : 0.0;
		CGFloat contentWidth = textSize.width + (paddingX * 2);
		CGFloat contentHeight = textSize.height + (paddingY * 2);
		CGFloat boxWidth = MAX(contentWidth, contentHeight);
		CGFloat boxHeight = contentHeight + arrowHeight;
		CGFloat elementCenterX = position.x;
		CGFloat elementCenterY = position.y;
		CGFloat tooltipX = elementCenterX - boxWidth / 2.0;
		CGFloat tooltipY = elementCenterY + arrowHeight + kHintArrowGap;
		CGFloat flippedY = screenHeight - tooltipY - boxHeight;
		CGFloat flippedElementCenterY = screenHeight - elementCenterY;
		NSRect hintRect = NSMakeRect(tooltipX, flippedY, boxWidth, boxHeight);

		// Skip hints outside the dirty region
		if (filterByRect) {
			CGFloat expand = ceil(self.hintBorderWidth / 2.0) + 1.0;
			NSRect testRect =
			    NSMakeRect(tooltipX - expand, flippedY - expand, boxWidth + expand * 2, boxHeight + expand * 2);
			if (showArrow && flippedElementCenterY > NSMaxY(testRect)) {
				testRect.size.height = flippedElementCenterY + 1.0 - testRect.origin.y;
			}
			if (!NSIntersectsRect(testRect, dirtyRect))
				continue;
		}

		// Draw background and border
		CGFloat resolvedBorderRadius =
		    self.hintBorderRadius >= 0.0 ? self.hintBorderRadius : MIN(hintRect.size.height / 2.0, 6.0);
		NSBezierPath *path;
		if (showArrow) {
			path = [self createTooltipPath:hintRect
			                     arrowSize:arrowHeight
			                elementCenterX:elementCenterX
			                elementCenterY:flippedElementCenterY];
		} else {
			path = [NSBezierPath bezierPathWithRoundedRect:hintRect
			                                       xRadius:resolvedBorderRadius
			                                       yRadius:resolvedBorderRadius];
		}
		[self.hintBackgroundColor setFill];
		[path fill];
		if (self.hintBorderWidth > 0) {
			[self.hintBorderColor setStroke];
			[path setLineWidth:self.hintBorderWidth];
			[path stroke];
		}

		// Draw text
		CGFloat textX = hintRect.origin.x + (boxWidth - textSize.width) / 2.0;
		CGFloat textY = hintRect.origin.y + paddingY;
		[attrString drawAtPoint:NSMakePoint(textX, textY)];
	}
}

/// Draw grid cells whose bounding rects intersect the given dirty rect.
/// This is the single implementation of grid cell drawing; drawGridCells delegates here.
/// @param dirtyRect The dirty region to redraw. Pass NSZeroRect to draw all items (skips intersection checks).
- (void)drawGridCellsInRect:(NSRect)dirtyRect {
	if ([self.gridCells count] == 0)
		return;

	BOOL filterByRect = !NSIsEmptyRect(dirtyRect);
	CGFloat screenHeight = self.bounds.size.height;
	CGFloat screenWidth = self.bounds.size.width;

	for (GridCellItem *cellItem in self.gridCells) {
		NSString *label = cellItem.label;
		CGRect bounds = cellItem.bounds;
		BOOL isMatched = cellItem.isMatched;
		BOOL isSubgrid = cellItem.isSubgrid;

		if (self.hideUnmatched && !isMatched && !isSubgrid)
			continue;

		// Flip coordinates and skip if outside dirty region
		CGFloat flippedY = screenHeight - bounds.origin.y - bounds.size.height;
		NSRect cellRect = NSMakeRect(bounds.origin.x, flippedY, bounds.size.width, bounds.size.height);
		if (filterByRect && !NSIntersectsRect(cellRect, dirtyRect))
			continue;

		// Draw background
		NSColor *bgBase =
		    isMatched && self.gridMatchedBackgroundColor ? self.gridMatchedBackgroundColor : self.gridBackgroundColor;
		[bgBase setFill];
		NSRectFill(cellRect);

		// Draw border
		NSColor *borderColor =
		    isMatched && self.gridMatchedBorderColor ? self.gridMatchedBorderColor : self.gridBorderColor;
		[borderColor setStroke];
		NSRect borderRect = cellRect;
		if ((int)self.gridBorderWidth % 2 == 1) {
			borderRect = NSOffsetRect(cellRect, 0.5, -0.5);
		}
		if (NSMaxX(cellRect) >= screenWidth) {
			borderRect.size.width -= 1.0;
		}
		if (NSMinY(cellRect) <= 0) {
			borderRect.origin.y += ceil(self.gridBorderWidth / 2.0);
			borderRect.size.height -= ceil(self.gridBorderWidth / 2.0);
		}
		NSBezierPath *borderPath = [NSBezierPath bezierPathWithRect:borderRect];
		[borderPath setLineWidth:self.gridBorderWidth];
		[borderPath stroke];

		// Draw label
		if (label && [label length] > 0) {
			if (self.gridDrawSubKeyPreview)
				[self drawSubKeyPreviewInCellRect:cellRect];
			[self drawGridLabel:label
			             inCellRect:cellRect
			              isMatched:isMatched
			    matchedPrefixLength:cellItem.matchedPrefixLength
			                  alpha:1.0];
		}
	}
}

- (void)drawAnimatedGridCellsInRect:(NSRect)dirtyRect progress:(CGFloat)progress {
	NSArray<GridCellItem *> *fromCells = self.transitionFromGridCells ?: @[];
	NSArray<GridCellItem *> *toCells = self.transitionToGridCells ?: @[];
	NSUInteger count = MAX([fromCells count], [toCells count]);
	if (count == 0) {
		return;
	}

	BOOL filterByRect = !NSIsEmptyRect(dirtyRect);
	CGFloat screenHeight = self.bounds.size.height;
	CGRect fromBounds = CGRectNull;
	CGRect toBounds = CGRectNull;

	for (GridCellItem *cell in fromCells) {
		fromBounds = CGRectIsNull(fromBounds) ? cell.bounds : CGRectUnion(fromBounds, cell.bounds);
	}
	for (GridCellItem *cell in toCells) {
		toBounds = CGRectIsNull(toBounds) ? cell.bounds : CGRectUnion(toBounds, cell.bounds);
	}
	if (CGRectIsNull(fromBounds)) {
		fromBounds = CGRectIsNull(toBounds) ? CGRectZero : toBounds;
	}
	if (CGRectIsNull(toBounds)) {
		toBounds = fromBounds;
	}

	for (NSUInteger idx = 0; idx < count; idx++) {
		GridCellItem *fromCell = idx < [fromCells count] ? fromCells[idx] : nil;
		GridCellItem *toCell = idx < [toCells count] ? toCells[idx] : nil;
		CGRect startRect = fromCell ? fromCell.bounds : fromBounds;
		CGRect endRect = toCell ? toCell.bounds : toBounds;
		CGRect rect = CGRectMake(
		    startRect.origin.x + (endRect.origin.x - startRect.origin.x) * progress,
		    startRect.origin.y + (endRect.origin.y - startRect.origin.y) * progress,
		    startRect.size.width + (endRect.size.width - startRect.size.width) * progress,
		    startRect.size.height + (endRect.size.height - startRect.size.height) * progress);

		CGFloat flippedY = screenHeight - rect.origin.y - rect.size.height;
		NSRect cellRect = NSMakeRect(rect.origin.x, flippedY, rect.size.width, rect.size.height);
		if (filterByRect && !NSIntersectsRect(cellRect, dirtyRect)) {
			continue;
		}

		NSColor *bgBase = self.gridBackgroundColor;
		[bgBase setFill];
		NSRectFill(cellRect);

		[self.gridBorderColor setStroke];
		NSRect borderRect = cellRect;
		if ((int)self.gridBorderWidth % 2 == 1) {
			borderRect = NSOffsetRect(cellRect, 0.5, -0.5);
		}
		NSBezierPath *borderPath = [NSBezierPath bezierPathWithRect:borderRect];
		[borderPath setLineWidth:self.gridBorderWidth];
		[borderPath stroke];

		NSString *fromLabel = fromCell.label ?: @"";
		NSString *toLabel = toCell.label ?: fromLabel;
		BOOL labelsMatch = [fromLabel isEqualToString:toLabel];
		if (self.gridDrawSubKeyPreview) {
			[self drawSubKeyPreviewInCellRect:cellRect];
		}
		if (labelsMatch) {
			[self drawGridLabel:toLabel inCellRect:cellRect isMatched:NO matchedPrefixLength:0 alpha:1.0];
			continue;
		}

		if (fromLabel.length > 0 && progress < 1.0) {
			[self drawGridLabel:fromLabel
			             inCellRect:cellRect
			              isMatched:NO
			    matchedPrefixLength:0
			                  alpha:(1.0 - progress)];
		}
		if (toLabel.length > 0 && progress > 0.0) {
			[self drawGridLabel:toLabel inCellRect:cellRect isMatched:NO matchedPrefixLength:0 alpha:progress];
		}
	}
}

/// Draw a grid label centered in the cell, optionally with a rounded badge.
/// @param label Grid label
/// @param cellRect Cell rectangle in view coordinates
/// @param isMatched Whether the cell currently matches the typed prefix
/// @param matchedPrefixLength Number of leading characters to draw with matched styling
- (void)drawGridLabel:(NSString *)label
             inCellRect:(NSRect)cellRect
              isMatched:(BOOL)isMatched
    matchedPrefixLength:(int)matchedPrefixLength {
	[self drawGridLabel:label
	             inCellRect:cellRect
	              isMatched:isMatched
	    matchedPrefixLength:matchedPrefixLength
	                  alpha:1.0];
}

- (void)drawGridLabel:(NSString *)label
             inCellRect:(NSRect)cellRect
              isMatched:(BOOL)isMatched
    matchedPrefixLength:(int)matchedPrefixLength
                  alpha:(CGFloat)alpha {
	if (alpha <= 0.0) {
		return;
	}

	// Set up attributed string
	NSMutableAttributedString *attrString = self.cachedGridCellAttributedString;
	[[attrString mutableString] setString:label];
	NSRange fullRange = NSMakeRange(0, [label length]);
	[attrString setAttributes:@{NSFontAttributeName : self.gridFont} range:fullRange];
	[attrString addAttribute:NSForegroundColorAttributeName
	                   value:[self color:self.cachedGridTextColor withMultipliedAlpha:alpha]
	                   range:fullRange];
	if (isMatched && matchedPrefixLength > 0 && matchedPrefixLength <= [label length]) {
		[attrString addAttribute:NSForegroundColorAttributeName
		                   value:[self color:self.cachedGridMatchedTextColor withMultipliedAlpha:alpha]
		                   range:NSMakeRange(0, matchedPrefixLength)];
	}

	NSSize textSize = [attrString size];

	// Fast path: no badge background
	if (!self.gridDrawLabelBackground) {
		CGFloat textX = cellRect.origin.x + (cellRect.size.width - textSize.width) / 2.0;
		CGFloat textY = cellRect.origin.y + (cellRect.size.height - textSize.height) / 2.0;
		[attrString drawAtPoint:NSMakePoint(textX, textY)];
		return;
	}

	// Compute badge dimensions
	CGFloat horizontalPadding = self.gridLabelBackgroundPaddingX >= 0.0
	                                ? self.gridLabelBackgroundPaddingX
	                                : MAX(4.0, round(self.gridFont.pointSize * 0.4));
	CGFloat verticalPadding = self.gridLabelBackgroundPaddingY >= 0.0 ? self.gridLabelBackgroundPaddingY
	                                                                  : MAX(2.0, round(self.gridFont.pointSize * 0.2));
	CGFloat badgeWidth = MAX(textSize.width + (horizontalPadding * 2.0), textSize.height + (verticalPadding * 2.0));
	CGFloat badgeHeight = textSize.height + (verticalPadding * 2.0);

	// Clamp badge to cell bounds — fall back to plain text if cell is too small
	CGFloat maxBadgeWidth = MAX(0.0, cellRect.size.width - 4.0);
	CGFloat maxBadgeHeight = MAX(0.0, cellRect.size.height - 4.0);
	if (maxBadgeWidth <= 0.0 || maxBadgeHeight <= 0.0) {
		CGFloat textX = cellRect.origin.x + (cellRect.size.width - textSize.width) / 2.0;
		CGFloat textY = cellRect.origin.y + (cellRect.size.height - textSize.height) / 2.0;
		[attrString drawAtPoint:NSMakePoint(textX, textY)];
		return;
	}
	badgeWidth = MIN(badgeWidth, maxBadgeWidth);
	badgeHeight = MIN(badgeHeight, maxBadgeHeight);

	// Resolve badge colors
	NSColor *badgeFill = (self.gridLabelBackgroundColor ? self.gridLabelBackgroundColor : self.gridBackgroundColor);
	badgeFill = [self color:badgeFill withMultipliedAlpha:alpha];
	NSColor *badgeBorder =
	    isMatched && self.gridMatchedBorderColor ? self.gridMatchedBorderColor : self.gridBorderColor;
	badgeBorder = [self color:badgeBorder withMultipliedAlpha:alpha];

	// Draw badge fill
	NSRect badgeRect = NSMakeRect(
	    cellRect.origin.x + (cellRect.size.width - badgeWidth) / 2.0,
	    cellRect.origin.y + (cellRect.size.height - badgeHeight) / 2.0, badgeWidth, badgeHeight);
	CGFloat maxRadius = MIN(badgeRect.size.width, badgeRect.size.height) / 2.0;
	CGFloat radius = self.gridLabelBackgroundBorderRadius >= 0.0 ? MIN(self.gridLabelBackgroundBorderRadius, maxRadius)
	                                                             : MIN(badgeRect.size.height / 2.0, 6.0);
	NSBezierPath *badgePath = [NSBezierPath bezierPathWithRoundedRect:badgeRect xRadius:radius yRadius:radius];
	[badgeFill setFill];
	[badgePath fill];

	// Draw badge border
	// Inset the stroke path by half the border width so the stroke stays entirely
	// within the badge rect and does not bleed into adjacent cells.
	if (badgeBorder && self.gridLabelBackgroundBorderWidth > 0.0) {
		CGFloat inset = self.gridLabelBackgroundBorderWidth / 2.0;
		NSRect strokeRect = NSInsetRect(badgeRect, inset, inset);
		if (strokeRect.size.width > 0.0 && strokeRect.size.height > 0.0) {
			CGFloat strokeMaxRadius = MIN(strokeRect.size.width, strokeRect.size.height) / 2.0;
			CGFloat strokeRadius = MIN(MAX(radius - inset, 0.0), strokeMaxRadius);
			NSBezierPath *strokePath = [NSBezierPath bezierPathWithRoundedRect:strokeRect
			                                                           xRadius:strokeRadius
			                                                           yRadius:strokeRadius];
			[badgeBorder setStroke];
			[strokePath setLineWidth:self.gridLabelBackgroundBorderWidth];
			[strokePath stroke];
		}
	}

	// Draw text centered in badge
	CGFloat textX = badgeRect.origin.x + (badgeRect.size.width - textSize.width) / 2.0;
	CGFloat textY = badgeRect.origin.y + (badgeRect.size.height - textSize.height) / 2.0;
	[attrString drawAtPoint:NSMakePoint(textX, textY)];
}

/// Draw a miniature version of the key grid inside a cell.
/// Each sub-cell shows the corresponding key label at reduced size and opacity.
/// Uses gridSubKeyLabels (the *next* depth's keys) so each cell previews what
/// pressing that key will produce, rather than echoing the current depth's layout.
/// When the preview layout has a true center cell (odd cols and odd rows),
/// that center label is omitted so it does not sit directly beneath the
/// prominently drawn main cell label.
/// @param cellRect The cell rectangle in view coordinates (Y-up, already flipped)
- (void)drawSubKeyPreviewInCellRect:(NSRect)cellRect {
	int cols = self.gridSubKeyCols;
	int rows = self.gridSubKeyRows;
	NSArray<NSString *> *labels = self.gridSubKeyLabels;
	NSUInteger count = labels ? [labels count] : 0;

	if (cols <= 0 || rows <= 0 || count == 0)
		return;

	// Guard: labels must contain exactly cols*rows items so that
	// the positional index (row * cols + col) maps to the correct label.
	// If they are out of sync the preview would silently misalign.
	if (count != (NSUInteger)(cols * rows)) {
		NSLog(@"[Neru] sub-key preview skipped: label count %lu != cols*rows %d", (unsigned long)count, cols * rows);
		return;
	}

	NSFont *subFont = self.gridSubKeyFont;
	NSColor *subColor = self.gridSubKeyTextColor;
	if (!subFont || !subColor)
		return;

	// Skip sub-key preview when sub-cells are too small to render legibly.
	// Each sub-cell must be at least (multiplier × font size) in both dimensions.
	// A multiplier of 0 disables autohide.
	CGFloat subCellWidth = cellRect.size.width / cols;
	CGFloat subCellHeight = cellRect.size.height / rows;
	CGFloat minSubCell = subFont.pointSize * self.gridSubKeyAutohideMultiplier;
	if (self.gridSubKeyAutohideMultiplier > 0 && (subCellWidth < minSubCell || subCellHeight < minSubCell))
		return;

	// Determine center index to skip (only for odd cols × odd rows layouts)
	NSUInteger centerIdx = NSNotFound;
	if (cols % 2 == 1 && rows % 2 == 1) {
		centerIdx = (NSUInteger)((rows / 2) * cols + (cols / 2));
	}

	NSMutableAttributedString *str = self.cachedGridSubKeyAttributedString;
	for (int row = 0; row < rows; row++) {
		for (int col = 0; col < cols; col++) {
			NSUInteger idx = (NSUInteger)(row * cols + col);
			if (idx >= count)
				break;
			if (idx == centerIdx)
				continue;

			NSString *subLabel = labels[idx];
			if (!subLabel || subLabel.length == 0)
				continue;

			// Sub-cells: row 0 is top of the cell.
			// In NSView coordinates (Y increases upward), top = larger Y.
			CGFloat subOriginX = cellRect.origin.x + col * subCellWidth;
			CGFloat subOriginY = cellRect.origin.y + (rows - 1 - row) * subCellHeight;
			NSRect subRect = NSMakeRect(subOriginX, subOriginY, subCellWidth, subCellHeight);

			[[str mutableString] setString:subLabel];
			NSRange range = NSMakeRange(0, subLabel.length);
			[str setAttributes:@{NSFontAttributeName : subFont, NSForegroundColorAttributeName : subColor} range:range];

			NSSize textSize = [str size];
			CGFloat x = subRect.origin.x + (subCellWidth - textSize.width) / 2.0;
			CGFloat y = subRect.origin.y + (subCellHeight - textSize.height) / 2.0;
			[str drawAtPoint:NSMakePoint(x, y)];
		}
	}
}

@end

#pragma mark - Overlay Window Controller Interface

@interface OverlayWindowController : NSObject
@property(nonatomic, strong) NSPanel *window;           ///< Panel instance (non-activating overlay)
@property(nonatomic, strong) OverlayView *overlayView;  ///< Overlay view instance
@property(nonatomic, assign) NSInteger sharingType;     ///< Current window sharing type
@property(nonatomic, assign) BOOL sharingTypeExplicit;  ///< Whether sharingType was explicitly configured
@end

#pragma mark - Overlay Window Controller Implementation

@implementation OverlayWindowController

/// Initialize
/// @return Initialized instance
- (instancetype)init {
	self = [super init];
	if (self) {
		[self createWindow];
	}
	return self;
}

/// Create window
- (void)createWindow {
	NSScreen *mainScreen = [NSScreen mainScreen];
	NSRect screenFrame = [mainScreen frame];

	// Use NSPanel for better floating overlay behavior.
	// Non-activating panel won't steal focus from other apps.
	NSPanel *panel =
	    [[NSPanel alloc] initWithContentRect:screenFrame
	                               styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
	                                 backing:NSBackingStoreBuffered
	                                   defer:NO];
	[panel setHidesOnDeactivate:NO];
	[panel setReleasedWhenClosed:NO];

	self.window = panel;

	// Disable animations
	if ([self.window respondsToSelector:@selector(setAnimationBehavior:)]) {
		[self.window setAnimationBehavior:NSWindowAnimationBehaviorNone];
	}
	[self.window setAnimations:@{}];
	[self.window setAlphaValue:1.0];

	// Window appearance and behavior
	[self.window setLevel:NSScreenSaverWindowLevel];
	[self.window setOpaque:NO];
	[self.window setBackgroundColor:[NSColor clearColor]];
	[self.window setIgnoresMouseEvents:YES];
	[self.window setAcceptsMouseMovedEvents:NO];
	[self.window setHasShadow:NO];
	[self.window
	    setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary |
	                          NSWindowCollectionBehaviorFullScreenAuxiliary | NSWindowCollectionBehaviorIgnoresCycle];

	// Set sharing type — default to visible (NSWindowSharingReadOnly = 1) unless explicitly configured
	if (!self.sharingTypeExplicit) {
		self.sharingType = NSWindowSharingReadOnly;
	}
	[self.window setSharingType:self.sharingType];

	// Create and attach overlay view
	NSRect viewFrame = NSMakeRect(0, 0, screenFrame.size.width, screenFrame.size.height);
	self.overlayView = [[OverlayView alloc] initWithFrame:viewFrame];
	[self.window setContentView:self.overlayView];
}

@end

#pragma mark - C Interface Implementation

/// Create overlay window
/// @return Overlay window handle
OverlayWindow createOverlayWindow(void) {
	__block OverlayWindowController *controller = nil;
	if ([NSThread isMainThread]) {
		controller = [[OverlayWindowController alloc] init];
	} else {
		dispatch_sync(dispatch_get_main_queue(), ^{
			controller = [[OverlayWindowController alloc] init];
		});
	}

	return (__bridge_retained void *)controller;  // Transfer ownership to caller
}

/// Destroy overlay window
/// @param window Overlay window handle
void NeruDestroyOverlayWindow(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	if ([NSThread isMainThread]) {
		[controller.window close];
		CFRelease(window);  // Balance the CFBridgingRetain from createOverlayWindow
	} else {
		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.window close];
			CFRelease(window);  // Balance the CFBridgingRetain from createOverlayWindow
		});
	}
}

/// Show overlay window
/// @param window Overlay window handle
void NeruShowOverlayWindow(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	dispatch_async(dispatch_get_main_queue(), ^{
		[controller.window setLevel:kCGMaximumWindowLevel];
		[controller.window setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces |
		                                         NSWindowCollectionBehaviorStationary |
		                                         NSWindowCollectionBehaviorIgnoresCycle |
		                                         NSWindowCollectionBehaviorFullScreenAuxiliary];

		[controller.window setIsVisible:YES];
		[controller.window orderFrontRegardless];
		[controller.window display];

		[controller.overlayView setNeedsDisplay:YES];
	});
}

/// Hide overlay window
/// @param window Overlay window handle
void NeruHideOverlayWindow(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	if ([NSThread isMainThread]) {
		[controller.window orderOut:nil];
	} else {
		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.window orderOut:nil];
		});
	}
}

/// Clear overlay
/// @param window Overlay window handle
void NeruClearOverlay(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	if ([NSThread isMainThread]) {
		[controller.overlayView cancelGridTransition];
		[controller.overlayView cancelCursorIndicatorTransition];
		[controller.overlayView.hints removeAllObjects];
		[controller.overlayView.gridCells removeAllObjects];
		controller.overlayView.cursorIndicatorVisible = NO;
		[controller.overlayView setNeedsDisplay:YES];
	} else {
		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.overlayView cancelGridTransition];
			[controller.overlayView cancelCursorIndicatorTransition];
			[controller.overlayView.hints removeAllObjects];
			[controller.overlayView.gridCells removeAllObjects];
			controller.overlayView.cursorIndicatorVisible = NO;
			[controller.overlayView setNeedsDisplay:YES];
		});
	}
}

/// Resize overlay to main screen
/// @param window Overlay window handle
void NeruResizeOverlayToMainScreen(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	dispatch_async(dispatch_get_main_queue(), ^{
		NSScreen *mainScreen = [NSScreen mainScreen];
		if (!mainScreen)
			return;

		NSRect screenFrame = [mainScreen frame];
		[controller.window setFrame:screenFrame display:YES];

		NSRect viewFrame = NSMakeRect(0, 0, screenFrame.size.width, screenFrame.size.height);
		[controller.overlayView setFrame:viewFrame];
		[controller.overlayView setNeedsDisplay:YES];

		// Force reset window state to handle "stuck" windows after full-screen transitions
		[controller.window orderOut:nil];
		[controller.window setLevel:kCGMaximumWindowLevel];
		[controller.window setCollectionBehavior:NSWindowCollectionBehaviorDefault];

		// Use a separate dispatch to ensure the window server processes the orderOut and state reset
		// before we bring the window back. This helps break the association with the previous space.
		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.window setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces |
			                                         NSWindowCollectionBehaviorStationary |
			                                         NSWindowCollectionBehaviorIgnoresCycle |
			                                         NSWindowCollectionBehaviorFullScreenAuxiliary];
			[controller.window setIsVisible:YES];
			[controller.window orderFrontRegardless];
		});
	});
}

/// Resize overlay to active screen
/// @param window Overlay window handle
void NeruResizeOverlayToActiveScreen(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	dispatch_async(dispatch_get_main_queue(), ^{
		NSPoint mouseLoc = [NSEvent mouseLocation];

		// Find the screen containing the mouse cursor
		NSScreen *activeScreen = nil;
		for (NSScreen *screen in [NSScreen screens]) {
			if (NSPointInRect(mouseLoc, screen.frame)) {
				activeScreen = screen;
				break;
			}
		}
		if (!activeScreen)
			activeScreen = [NSScreen mainScreen];
		if (!activeScreen)
			return;

		NSRect screenFrame = [activeScreen frame];
		[controller.window setFrame:screenFrame display:YES];

		NSRect viewFrame = NSMakeRect(0, 0, screenFrame.size.width, screenFrame.size.height);
		[controller.overlayView setFrame:viewFrame];
		[controller.overlayView setNeedsDisplay:YES];

		// Force reset window state to handle "stuck" windows after full-screen transitions
		[controller.window orderOut:nil];
		[controller.window setLevel:kCGMaximumWindowLevel];
		[controller.window setCollectionBehavior:NSWindowCollectionBehaviorDefault];

		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.window setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces |
			                                         NSWindowCollectionBehaviorStationary |
			                                         NSWindowCollectionBehaviorIgnoresCycle |
			                                         NSWindowCollectionBehaviorFullScreenAuxiliary];
			[controller.window setIsVisible:YES];
			[controller.window orderFrontRegardless];
		});
	});
}

/// Resize overlay to active screen with callback
/// @param window Overlay window handle
/// @param callback Completion callback
/// @param context Callback context
void NeruResizeOverlayToActiveScreenWithCallback(
    OverlayWindow window, ResizeCompletionCallback callback, void *context) {
	if (!window) {
		if (callback)
			callback(context);
		return;
	}

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	dispatch_async(dispatch_get_main_queue(), ^{
		NSPoint mouseLoc = [NSEvent mouseLocation];

		// Find the screen containing the mouse cursor
		NSScreen *activeScreen = nil;
		for (NSScreen *screen in [NSScreen screens]) {
			if (NSPointInRect(mouseLoc, screen.frame)) {
				activeScreen = screen;
				break;
			}
		}
		if (!activeScreen)
			activeScreen = [NSScreen mainScreen];
		if (!activeScreen) {
			if (callback)
				callback(context);
			return;
		}

		NSRect screenFrame = [activeScreen frame];
		[controller.window setFrame:screenFrame display:YES];

		NSRect viewFrame = NSMakeRect(0, 0, screenFrame.size.width, screenFrame.size.height);
		[controller.overlayView setFrame:viewFrame];
		[controller.overlayView setNeedsDisplay:YES];

		// Force reset window state to handle "stuck" windows after full-screen transitions
		[controller.window orderOut:nil];
		[controller.window setLevel:kCGMaximumWindowLevel];
		[controller.window setCollectionBehavior:NSWindowCollectionBehaviorDefault];

		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.window setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces |
			                                         NSWindowCollectionBehaviorStationary |
			                                         NSWindowCollectionBehaviorIgnoresCycle |
			                                         NSWindowCollectionBehaviorFullScreenAuxiliary];
			[controller.window setIsVisible:YES];
			[controller.window orderFrontRegardless];

			if (callback)
				callback(context);
		});
	});
}

#pragma mark - Helper Functions

/// Helper function to copy style strings safely
/// @param str String to copy
/// @return Duplicated string
static inline char *safe_strdup(const char *str) { return str ? strdup(str) : NULL; }

/// Helper function to free style strings
/// @param style Hint style
static inline void free_hint_style_strings(const HintStyle *style) {
	if (style->fontFamily)
		free((void *)style->fontFamily);
	if (style->backgroundColor)
		free((void *)style->backgroundColor);
	if (style->textColor)
		free((void *)style->textColor);
	if (style->matchedTextColor)
		free((void *)style->matchedTextColor);
	if (style->borderColor)
		free((void *)style->borderColor);
}

/// Build GridCellItem array from C GridCell array.
/// Safe to call from any thread (only creates ObjC objects from C data).
/// @param cells Array of grid cell data
/// @param count Number of cells
/// @return Array of GridCellItem objects
static NSMutableArray<GridCellItem *> *buildGridCellItems(GridCell *cells, int count) {
	NSMutableArray<GridCellItem *> *cellItems = [NSMutableArray arrayWithCapacity:count];
	for (int i = 0; i < count; i++) {
		GridCell cell = cells[i];
		GridCellItem *cellItem = [[GridCellItem alloc] init];
		cellItem.label = cell.label ? @(cell.label) : @"";
		cellItem.bounds = cell.bounds;
		cellItem.isMatched = cell.isMatched ? YES : NO;
		cellItem.isSubgrid = cell.isSubgrid ? YES : NO;
		cellItem.matchedPrefixLength = cell.matchedPrefixLength;
		[cellItems addObject:cellItem];
	}
	return cellItems;
}

/// Build HintItem array from C HintData array.
/// Safe to call from any thread (only creates ObjC objects from C data).
/// @param hints Array of hint data
/// @param count Number of hints
/// @param showArrow Whether hints should show an arrow
/// @return Array of HintItem objects
static NSMutableArray<HintItem *> *buildHintItems(HintData *hints, int count, BOOL showArrow) {
	NSMutableArray<HintItem *> *hintItems = [NSMutableArray arrayWithCapacity:count];
	for (int i = 0; i < count; i++) {
		HintData hint = hints[i];
		HintItem *hintItem = [[HintItem alloc] init];
		hintItem.label = hint.label ? @(hint.label) : @"";
		hintItem.position = hint.position;
		hintItem.matchedPrefixLength = hint.matchedPrefixLength;
		hintItem.showArrow = showArrow;
		[hintItems addObject:hintItem];
	}
	return hintItems;
}

/// Draw hints
/// @param window Overlay window handle
/// @param hints Array of hint data
/// @param count Number of hints
/// @param style Hint style
void NeruDrawHints(OverlayWindow window, HintData *hints, int count, HintStyle style) {
	if (!window || !hints)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	// Build hint items upfront — safe from any thread
	NSMutableArray<HintItem *> *hintItems = buildHintItems(hints, count, style.showArrow ? YES : NO);

	if ([NSThread isMainThread]) {
		[controller.overlayView.hints removeAllObjects];
		[controller.overlayView applyStyle:style];
		[controller.overlayView.hints addObjectsFromArray:hintItems];
		[controller.overlayView setNeedsDisplay:YES];
	} else {
		// Copy style strings before crossing the thread boundary
		HintStyle styleCopy = {
		    .fontSize = style.fontSize,
		    .borderRadius = style.borderRadius,
		    .borderWidth = style.borderWidth,
		    .paddingX = style.paddingX,
		    .paddingY = style.paddingY,
		    .showArrow = style.showArrow,
		    .fontFamily = safe_strdup(style.fontFamily),
		    .backgroundColor = safe_strdup(style.backgroundColor),
		    .textColor = safe_strdup(style.textColor),
		    .matchedTextColor = safe_strdup(style.matchedTextColor),
		    .borderColor = safe_strdup(style.borderColor)};

		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.overlayView.hints removeAllObjects];
			[controller.overlayView applyStyle:styleCopy];
			[controller.overlayView.hints addObjectsFromArray:hintItems];
			[controller.overlayView setNeedsDisplay:YES];

			free_hint_style_strings(&styleCopy);
		});
	}
}

/// Update hint match prefix (incremental update for typing).
/// Only invalidates the bounding rects of hints whose matchedPrefixLength actually changed,
/// enabling partial redraw in drawLayer:inContext:.
/// @param window Overlay window handle
/// @param prefix Match prefix
void NeruUpdateHintMatchPrefix(OverlayWindow window, const char *prefix) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	NSString *prefixStr = prefix ? @(prefix) : @"";

	dispatch_async(dispatch_get_main_queue(), ^{
		BOOL anyInvalidated = NO;
		NSUInteger prefixLen = [prefixStr length];

		for (HintItem *hintItem in controller.overlayView.hints) {
			NSString *label = hintItem.label ?: @"";
			int newMatchedPrefixLength = 0;
			if (prefixLen > 0 && [label hasPrefix:prefixStr]) {
				newMatchedPrefixLength = (int)prefixLen;
			}

			// Only invalidate if the match state actually changed
			if (hintItem.matchedPrefixLength != newMatchedPrefixLength) {
				hintItem.matchedPrefixLength = newMatchedPrefixLength;
				NSRect dirtyRect = [controller.overlayView boundingRectForHint:hintItem];
				if (!NSIsEmptyRect(dirtyRect)) {
					[controller.overlayView setNeedsDisplayInRect:dirtyRect];
					anyInvalidated = YES;
				}
			}
		}

		if (anyInvalidated) {
			// Signal partial redraw mode so drawLayer:inContext: uses the clip box
			controller.overlayView.fullRedraw = NO;
		}
	});
}

/// Draw hints incrementally (add/update/remove specific hints without clearing entire overlay)
/// @param window Overlay window handle
/// @param hintsToAdd Array of hint data to add or update
/// @param addCount Number of hints to add/update
/// @param positionsToRemove Array of hint positions to remove (by matching position)
/// @param removeCount Number of hints to remove
/// @param style Hint style (used for new/updated hints)
void NeruDrawIncrementHints(
    OverlayWindow window, HintData *hintsToAdd, int addCount, CGPoint *positionsToRemove, int removeCount,
    HintStyle style) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	// Build hint data arrays upfront — safe from any thread
	NSMutableArray<HintItem *> *hintItemsToAdd = nil;
	if (hintsToAdd && addCount > 0) {
		hintItemsToAdd = buildHintItems(hintsToAdd, addCount, style.showArrow ? YES : NO);
	}

	// Build positions array for hints to remove
	NSMutableArray *positionsToRemoveArray = nil;
	if (positionsToRemove && removeCount > 0) {
		positionsToRemoveArray = [NSMutableArray arrayWithCapacity:removeCount];
		for (int i = 0; i < removeCount; i++) {
			NSValue *positionValue = [NSValue valueWithPoint:NSPointFromCGPoint(positionsToRemove[i])];
			[positionsToRemoveArray addObject:positionValue];
		}
	}

	// Copy all style properties NOW (before async block)
	CGFloat fontSize = style.fontSize > 0 ? style.fontSize : kDefaultHintFontSize;
	NSString *fontFamily = nil;
	if (style.fontFamily) {
		fontFamily = @(style.fontFamily);
		if (fontFamily.length == 0)
			fontFamily = nil;
	}
	NSString *bgHex = style.backgroundColor ? @(style.backgroundColor) : nil;
	NSString *textHex = style.textColor ? @(style.textColor) : nil;
	NSString *matchedTextHex = style.matchedTextColor ? @(style.matchedTextColor) : nil;
	NSString *borderHex = style.borderColor ? @(style.borderColor) : nil;
	int borderRadius = style.borderRadius;
	int borderWidth = style.borderWidth;
	int paddingX = style.paddingX;
	int paddingY = style.paddingY;

	dispatch_async(dispatch_get_main_queue(), ^{
		// Apply font — only re-create when family or size changed
		BOOL hintFamilyChanged =
		    (fontFamily != controller.overlayView.cachedHintFontFamily &&
		     ![fontFamily isEqualToString:controller.overlayView.cachedHintFontFamily]);
		if (hintFamilyChanged || fontSize != controller.overlayView.cachedHintFontSize) {
			NSFont *font = nil;
			if (fontFamily && [fontFamily length] > 0) {
				font = [controller.overlayView resolveFont:fontFamily size:fontSize bold:YES];
			}
			if (!font)
				font = [NSFont boldSystemFontOfSize:fontSize];
			controller.overlayView.hintFont = font;
			controller.overlayView.cachedHintFontFamily = fontFamily;
			controller.overlayView.cachedHintFontSize = fontSize;
		}

		// Apply colors
		if (bgHex) {
			NSColor *defaultBg = [[NSColor colorWithRed:1.0 green:0.84 blue:0.0
			                                      alpha:1.0] colorWithAlphaComponent:0.95];
			controller.overlayView.hintBackgroundColor = [controller.overlayView colorFromHex:bgHex
			                                                                     defaultColor:defaultBg];
		}
		if (textHex) {
			controller.overlayView.hintTextColor = [controller.overlayView colorFromHex:textHex
			                                                               defaultColor:[NSColor blackColor]];
		}
		if (matchedTextHex) {
			controller.overlayView.hintMatchedTextColor =
			    [controller.overlayView colorFromHex:matchedTextHex defaultColor:[NSColor systemBlueColor]];
		}
		if (borderHex) {
			controller.overlayView.hintBorderColor = [controller.overlayView colorFromHex:borderHex
			                                                                 defaultColor:[NSColor blackColor]];
		}

		// Apply geometry properties
		controller.overlayView.hintBorderRadius = borderRadius;
		controller.overlayView.hintBorderWidth = borderWidth >= 0 ? borderWidth : 1.0;
		controller.overlayView.hintPaddingX = paddingX;
		controller.overlayView.hintPaddingY = paddingY;

		// Remove hints matching the given positions
		if (positionsToRemoveArray && [positionsToRemoveArray count] > 0) {
			// Build a set of position keys for O(1) lookup
			NSMutableSet *positionsToRemoveSet = [NSMutableSet setWithCapacity:[positionsToRemoveArray count]];
			for (NSValue *removePositionValue in positionsToRemoveArray) {
				NSPoint removePosition = [removePositionValue pointValue];
				NSString *key = [NSString stringWithFormat:@"%.6f,%.6f", removePosition.x, removePosition.y];
				[positionsToRemoveSet addObject:key];
			}

			NSMutableArray<HintItem *> *hintsToKeep =
			    [NSMutableArray arrayWithCapacity:[controller.overlayView.hints count]];
			for (HintItem *hintItem in controller.overlayView.hints) {
				NSPoint hintPosition = hintItem.position;
				NSString *hintKey = [NSString stringWithFormat:@"%.6f,%.6f", hintPosition.x, hintPosition.y];
				if (![positionsToRemoveSet containsObject:hintKey]) {
					[hintsToKeep addObject:hintItem];
				}
			}
			controller.overlayView.hints = hintsToKeep;
		}

		// Add or update hints
		if (hintItemsToAdd && [hintItemsToAdd count] > 0) {
			// Build lookup map for existing hints by position for O(1) access
			NSMutableDictionary *hintsByPosition =
			    [NSMutableDictionary dictionaryWithCapacity:[controller.overlayView.hints count]];
			for (HintItem *hintItem in controller.overlayView.hints) {
				NSPoint pos = hintItem.position;
				NSString *key = [NSString stringWithFormat:@"%.6f,%.6f", pos.x, pos.y];
				hintsByPosition[key] = hintItem;
			}

			for (HintItem *newHintItem in hintItemsToAdd) {
				NSPoint newPosition = newHintItem.position;
				NSString *key = [NSString stringWithFormat:@"%.6f,%.6f", newPosition.x, newPosition.y];
				HintItem *existingHint = hintsByPosition[key];
				if (existingHint) {
					NSUInteger index = [controller.overlayView.hints indexOfObjectIdenticalTo:existingHint];
					if (index != NSNotFound) {
						controller.overlayView.hints[index] = newHintItem;
					}
				} else {
					[controller.overlayView.hints addObject:newHintItem];
				}
			}
		}

		[controller.overlayView setNeedsDisplay:YES];
	});
}

/// Replace overlay window
/// @param pwindow Pointer to overlay window handle
void NeruReplaceOverlayWindow(OverlayWindow *pwindow) {
	if (!pwindow)
		return;

	// Must use dispatch_sync (not dispatch_async) because pwindow points into
	// Go struct memory (&o.window).  The pointer is only guaranteed to remain
	// valid while the calling Go function is on the stack; an async dispatch
	// could dereference it after the Go side has moved on, causing a
	// use-after-free.
	void (^replaceBlock)(void) = ^{
		OverlayWindowController *oldController = (__bridge OverlayWindowController *)(*pwindow);

		NSInteger sharingType = NSWindowSharingReadOnly;  // Default to visible
		if (oldController) {
			sharingType = oldController.sharingType;
		}

		OverlayWindowController *newController = [[OverlayWindowController alloc] init];
		newController.sharingType = sharingType;
		newController.sharingTypeExplicit = YES;
		[newController.window setSharingType:sharingType];

		if (oldController) {
			[oldController.window close];
			CFRelease(*pwindow);  // Balance the CFBridgingRetain from createOverlayWindow
		}

		*pwindow = (__bridge_retained void *)newController;  // Transfer ownership to caller
	};

	if ([NSThread isMainThread]) {
		replaceBlock();
	} else {
		dispatch_sync(dispatch_get_main_queue(), replaceBlock);
	}
}

/// Draw grid cells
/// @param window Overlay window handle
/// @param cells Array of grid cells
/// @param count Number of cells
/// @param style Grid cell style
void NeruDrawGridCells(OverlayWindow window, GridCell *cells, int count, GridCellStyle style) {
	if (!window || !cells)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	// Build cell data array upfront — safe from any thread
	NSMutableArray<GridCellItem *> *cellItems = buildGridCellItems(cells, count);

	// Copy all style properties NOW (before async block)
	CGFloat fontSize = style.fontSize > 0 ? style.fontSize : kDefaultGridFontSize;
	NSString *fontFamily = nil;
	if (style.fontFamily) {
		fontFamily = @(style.fontFamily);
		if (fontFamily.length == 0)
			fontFamily = nil;
	}
	NSString *bgHex = style.backgroundColor ? @(style.backgroundColor) : nil;
	NSString *labelBgHex = style.labelBackgroundColor ? @(style.labelBackgroundColor) : nil;
	NSString *textHex = style.textColor ? @(style.textColor) : nil;
	NSString *matchedTextHex = style.matchedTextColor ? @(style.matchedTextColor) : nil;
	NSString *matchedBgHex = style.matchedBackgroundColor ? @(style.matchedBackgroundColor) : nil;
	NSString *matchedBorderHex = style.matchedBorderColor ? @(style.matchedBorderColor) : nil;
	NSString *borderHex = style.borderColor ? @(style.borderColor) : nil;
	int borderWidth = style.borderWidth;
	BOOL drawLabelBackground = style.drawLabelBackground ? YES : NO;
	CGFloat labelBackgroundPaddingX = style.labelBackgroundPaddingX;
	CGFloat labelBackgroundPaddingY = style.labelBackgroundPaddingY;
	CGFloat labelBackgroundBorderRadius = style.labelBackgroundBorderRadius;
	CGFloat labelBackgroundBorderWidth = style.labelBackgroundBorderWidth;
	BOOL drawSubKeyPreview = style.drawSubKeyPreview ? YES : NO;
	int subKeyGridCols = style.subKeyGridCols;
	int subKeyGridRows = style.subKeyGridRows;
	CGFloat subKeyFontSize = style.subKeyFontSize > 0 ? style.subKeyFontSize : 6.0;
	CGFloat subKeyAutohideMultiplier = style.subKeyAutohideMultiplier;
	NSString *subKeyTextHex = style.subKeyTextColor ? @(style.subKeyTextColor) : nil;

	// Build sub-key labels array from the next-depth key string.
	// Use composed-character enumeration so this stays correct even if
	// non-ASCII characters are ever allowed in the future.
	NSString *subKeyKeysStr = style.subKeyKeys ? @(style.subKeyKeys) : nil;
	NSMutableArray<NSString *> *subKeyLabels = nil;
	if (subKeyKeysStr && subKeyKeysStr.length > 0) {
		subKeyLabels = [NSMutableArray arrayWithCapacity:subKeyKeysStr.length];
		[subKeyKeysStr
		    enumerateSubstringsInRange:NSMakeRange(0, subKeyKeysStr.length)
		                       options:NSStringEnumerationByComposedCharacterSequences
		                    usingBlock:^(
		                        NSString *substring, NSRange substringRange, NSRange enclosingRange, BOOL *stop) {
			                    [subKeyLabels addObject:substring];
		                    }];
	}

	dispatch_async(dispatch_get_main_queue(), ^{
		// Apply font — only re-create when family or size changed
		BOOL gridFamilyChanged =
		    (fontFamily != controller.overlayView.cachedGridFontFamily &&
		     ![fontFamily isEqualToString:controller.overlayView.cachedGridFontFamily]);
		if (gridFamilyChanged || fontSize != controller.overlayView.cachedGridFontSize) {
			NSFont *font = nil;
			if (fontFamily && [fontFamily length] > 0) {
				font = [controller.overlayView resolveFont:fontFamily size:fontSize bold:NO];
			}
			if (!font)
				font = [NSFont systemFontOfSize:fontSize];
			controller.overlayView.gridFont = font;
			controller.overlayView.cachedGridFontFamily = fontFamily;
			controller.overlayView.cachedGridFontSize = fontSize;
		}

		// Apply colors
		controller.overlayView.gridBackgroundColor = [controller.overlayView colorFromHex:bgHex
		                                                                     defaultColor:[NSColor whiteColor]];
		controller.overlayView.gridLabelBackgroundColor = [controller.overlayView
		    colorFromHex:labelBgHex
		    defaultColor:[[NSColor colorWithRed:1.0 green:0.84 blue:0.0 alpha:1.0] colorWithAlphaComponent:0.8]];
		controller.overlayView.gridTextColor = [controller.overlayView colorFromHex:textHex
		                                                               defaultColor:[NSColor blackColor]];
		controller.overlayView.gridMatchedTextColor = [controller.overlayView colorFromHex:matchedTextHex
		                                                                      defaultColor:[NSColor blueColor]];
		controller.overlayView.gridMatchedBackgroundColor = [controller.overlayView colorFromHex:matchedBgHex
		                                                                            defaultColor:[NSColor blueColor]];
		controller.overlayView.gridMatchedBorderColor = [controller.overlayView colorFromHex:matchedBorderHex
		                                                                        defaultColor:[NSColor blueColor]];
		controller.overlayView.gridBorderColor = [controller.overlayView colorFromHex:borderHex
		                                                                 defaultColor:[NSColor grayColor]];

		// Apply geometry and layout properties
		controller.overlayView.gridBorderWidth = borderWidth > 0 ? borderWidth : 1.0;
		controller.overlayView.gridDrawLabelBackground = drawLabelBackground;
		controller.overlayView.gridLabelBackgroundPaddingX = labelBackgroundPaddingX;
		controller.overlayView.gridLabelBackgroundPaddingY = labelBackgroundPaddingY;
		controller.overlayView.gridLabelBackgroundBorderRadius = labelBackgroundBorderRadius;
		controller.overlayView.gridLabelBackgroundBorderWidth = labelBackgroundBorderWidth;

		// Apply sub-key preview settings
		controller.overlayView.gridDrawSubKeyPreview = drawSubKeyPreview;
		controller.overlayView.gridSubKeyCols = subKeyGridCols;
		controller.overlayView.gridSubKeyRows = subKeyGridRows;
		controller.overlayView.gridSubKeyLabels = subKeyLabels;
		if (drawSubKeyPreview) {
			if (subKeyFontSize != controller.overlayView.cachedGridSubKeyFontSize) {
				controller.overlayView.gridSubKeyFont = [NSFont systemFontOfSize:subKeyFontSize];
				controller.overlayView.cachedGridSubKeyFontSize = subKeyFontSize;
			}
			controller.overlayView.gridSubKeyAutohideMultiplier = subKeyAutohideMultiplier;
			controller.overlayView.gridSubKeyTextColor = [controller.overlayView colorFromHex:subKeyTextHex
			                                                                     defaultColor:[NSColor grayColor]];
		}

		// Sync cached color references
		controller.overlayView.cachedGridTextColor = controller.overlayView.gridTextColor;
		controller.overlayView.cachedGridMatchedTextColor = controller.overlayView.gridMatchedTextColor;

		// Replace cell data and redisplay
		[controller.overlayView cancelGridTransition];
		[controller.overlayView cancelCursorIndicatorTransition];
		[controller.overlayView.gridCells removeAllObjects];
		[controller.overlayView.gridCells addObjectsFromArray:cellItems];
		[controller.overlayView setNeedsDisplay:YES];
	});
}

/// Animate recursive-grid cells between the current and next depth state.
/// @param window Overlay window handle
/// @param cells Target grid cells
/// @param count Number of target cells
/// @param style Grid cell style
/// @param duration Animation duration in seconds
void NeruAnimateRecursiveGridTransition(
    OverlayWindow window, GridCell *cells, int count, GridCellStyle style, double duration) {
	if (!window || !cells) {
		return;
	}

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	NSMutableArray<GridCellItem *> *cellItems = buildGridCellItems(cells, count);

	CGFloat fontSize = style.fontSize > 0 ? style.fontSize : kDefaultGridFontSize;
	NSString *fontFamily = nil;
	if (style.fontFamily) {
		fontFamily = @(style.fontFamily);
		if (fontFamily.length == 0) {
			fontFamily = nil;
		}
	}
	NSString *bgHex = style.backgroundColor ? @(style.backgroundColor) : nil;
	NSString *labelBgHex = style.labelBackgroundColor ? @(style.labelBackgroundColor) : nil;
	NSString *textHex = style.textColor ? @(style.textColor) : nil;
	NSString *matchedTextHex = style.matchedTextColor ? @(style.matchedTextColor) : nil;
	NSString *matchedBgHex = style.matchedBackgroundColor ? @(style.matchedBackgroundColor) : nil;
	NSString *matchedBorderHex = style.matchedBorderColor ? @(style.matchedBorderColor) : nil;
	NSString *borderHex = style.borderColor ? @(style.borderColor) : nil;
	int borderWidth = style.borderWidth;
	BOOL drawLabelBackground = style.drawLabelBackground ? YES : NO;
	CGFloat labelBackgroundPaddingX = style.labelBackgroundPaddingX;
	CGFloat labelBackgroundPaddingY = style.labelBackgroundPaddingY;
	CGFloat labelBackgroundBorderRadius = style.labelBackgroundBorderRadius;
	CGFloat labelBackgroundBorderWidth = style.labelBackgroundBorderWidth;
	BOOL drawSubKeyPreview = style.drawSubKeyPreview ? YES : NO;
	int subKeyGridCols = style.subKeyGridCols;
	int subKeyGridRows = style.subKeyGridRows;
	CGFloat subKeyFontSize = style.subKeyFontSize > 0 ? style.subKeyFontSize : 6.0;
	CGFloat subKeyAutohideMultiplier = style.subKeyAutohideMultiplier;
	NSString *subKeyTextHex = style.subKeyTextColor ? @(style.subKeyTextColor) : nil;

	NSString *subKeyKeysStr = style.subKeyKeys ? @(style.subKeyKeys) : nil;
	NSMutableArray<NSString *> *subKeyLabels = nil;
	if (subKeyKeysStr && subKeyKeysStr.length > 0) {
		subKeyLabels = [NSMutableArray arrayWithCapacity:subKeyKeysStr.length];
		[subKeyKeysStr
		    enumerateSubstringsInRange:NSMakeRange(0, subKeyKeysStr.length)
		                       options:NSStringEnumerationByComposedCharacterSequences
		                    usingBlock:^(
		                        NSString *substring, NSRange substringRange, NSRange enclosingRange, BOOL *stop) {
			                    [subKeyLabels addObject:substring];
		                    }];
	}

	dispatch_async(dispatch_get_main_queue(), ^{
		BOOL gridFamilyChanged =
		    (fontFamily != controller.overlayView.cachedGridFontFamily &&
		     ![fontFamily isEqualToString:controller.overlayView.cachedGridFontFamily]);
		if (gridFamilyChanged || fontSize != controller.overlayView.cachedGridFontSize) {
			NSFont *font = nil;
			if (fontFamily && [fontFamily length] > 0) {
				font = [controller.overlayView resolveFont:fontFamily size:fontSize bold:NO];
			}
			if (!font) {
				font = [NSFont systemFontOfSize:fontSize];
			}
			controller.overlayView.gridFont = font;
			controller.overlayView.cachedGridFontFamily = fontFamily;
			controller.overlayView.cachedGridFontSize = fontSize;
		}

		controller.overlayView.gridBackgroundColor = [controller.overlayView colorFromHex:bgHex
		                                                                     defaultColor:[NSColor whiteColor]];
		controller.overlayView.gridLabelBackgroundColor = [controller.overlayView
		    colorFromHex:labelBgHex
		    defaultColor:[[NSColor colorWithRed:1.0 green:0.84 blue:0.0 alpha:1.0] colorWithAlphaComponent:0.8]];
		controller.overlayView.gridTextColor = [controller.overlayView colorFromHex:textHex
		                                                               defaultColor:[NSColor blackColor]];
		controller.overlayView.gridMatchedTextColor = [controller.overlayView colorFromHex:matchedTextHex
		                                                                      defaultColor:[NSColor blueColor]];
		controller.overlayView.gridMatchedBackgroundColor = [controller.overlayView colorFromHex:matchedBgHex
		                                                                            defaultColor:[NSColor blueColor]];
		controller.overlayView.gridMatchedBorderColor = [controller.overlayView colorFromHex:matchedBorderHex
		                                                                        defaultColor:[NSColor blueColor]];
		controller.overlayView.gridBorderColor = [controller.overlayView colorFromHex:borderHex
		                                                                 defaultColor:[NSColor grayColor]];

		controller.overlayView.gridBorderWidth = borderWidth > 0 ? borderWidth : 1.0;
		controller.overlayView.gridDrawLabelBackground = drawLabelBackground;
		controller.overlayView.gridLabelBackgroundPaddingX = labelBackgroundPaddingX;
		controller.overlayView.gridLabelBackgroundPaddingY = labelBackgroundPaddingY;
		controller.overlayView.gridLabelBackgroundBorderRadius = labelBackgroundBorderRadius;
		controller.overlayView.gridLabelBackgroundBorderWidth = labelBackgroundBorderWidth;
		controller.overlayView.gridDrawSubKeyPreview = drawSubKeyPreview;
		controller.overlayView.gridSubKeyCols = subKeyGridCols;
		controller.overlayView.gridSubKeyRows = subKeyGridRows;
		controller.overlayView.gridSubKeyLabels = subKeyLabels;
		if (drawSubKeyPreview) {
			if (subKeyFontSize != controller.overlayView.cachedGridSubKeyFontSize) {
				controller.overlayView.gridSubKeyFont = [NSFont systemFontOfSize:subKeyFontSize];
				controller.overlayView.cachedGridSubKeyFontSize = subKeyFontSize;
			}
			controller.overlayView.gridSubKeyAutohideMultiplier = subKeyAutohideMultiplier;
			controller.overlayView.gridSubKeyTextColor = [controller.overlayView colorFromHex:subKeyTextHex
			                                                                     defaultColor:[NSColor grayColor]];
		}

		controller.overlayView.cachedGridTextColor = controller.overlayView.gridTextColor;
		controller.overlayView.cachedGridMatchedTextColor = controller.overlayView.gridMatchedTextColor;
		[controller.overlayView startGridTransitionToCells:cellItems duration:duration];
	});
}

/// Update grid match prefix (incremental update for typing).
/// Only invalidates the bounding rects of cells whose match state actually changed,
/// enabling partial redraw in drawLayer:inContext:.
/// @param window Overlay window handle
/// @param prefix Match prefix
void NeruUpdateGridMatchPrefix(OverlayWindow window, const char *prefix) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	NSString *prefixStr = prefix ? @(prefix) : @"";

	dispatch_async(dispatch_get_main_queue(), ^{
		OverlayView *view = controller.overlayView;
		NSUInteger cellCount = [view.gridCells count];
		if (cellCount == 0)
			return;

		NSUInteger prefixLen = [prefixStr length];
		BOOL anyMatchStateChanged = NO;

		// First pass: update all cells and track which ones changed.
		// Use a stack-allocated array for small counts, heap for large.
		BOOL stackFlags[256];
		BOOL *changedFlags = cellCount <= 256 ? stackFlags : (BOOL *)calloc(cellCount, sizeof(BOOL));
		NSUInteger changedCount = 0;
		NSUInteger idx = 0;

		for (GridCellItem *cellItem in view.gridCells) {
			NSString *label = cellItem.label ?: @"";
			BOOL newIsMatched = (prefixLen > 0 && [label hasPrefix:prefixStr]);
			int newMatchedPrefixLength = newIsMatched ? (int)prefixLen : 0;
			BOOL changed =
			    (cellItem.isMatched != newIsMatched || cellItem.matchedPrefixLength != newMatchedPrefixLength);

			if (cellItem.isMatched != newIsMatched) {
				anyMatchStateChanged = YES;
			}
			if (changed) {
				cellItem.isMatched = newIsMatched;
				cellItem.matchedPrefixLength = newMatchedPrefixLength;
				changedFlags[idx] = YES;
				changedCount++;
			} else {
				changedFlags[idx] = NO;
			}
			idx++;
		}

		if (changedCount == 0) {
			if (changedFlags != stackFlags)
				free(changedFlags);
			return;
		}

		// If hideUnmatched is active and cells toggled visibility, a full redraw is needed
		if (view.hideUnmatched && anyMatchStateChanged) {
			if (changedFlags != stackFlags)
				free(changedFlags);
			view.fullRedraw = YES;
			[view setNeedsDisplay:YES];
			return;
		}

		// Second pass: partial redraw — only invalidate changed cells
		BOOL anyInvalidated = NO;
		idx = 0;
		for (GridCellItem *cellItem in view.gridCells) {
			if (changedFlags[idx]) {
				NSRect dirtyRect = [view screenRectForGridCell:cellItem];
				if (!NSIsEmptyRect(dirtyRect)) {
					[view setNeedsDisplayInRect:dirtyRect];
					anyInvalidated = YES;
				}
			}
			idx++;
		}

		if (changedFlags != stackFlags)
			free(changedFlags);

		if (anyInvalidated) {
			// Signal partial redraw mode so drawLayer:inContext: uses the clip box
			view.fullRedraw = NO;
		}
	});
}

/// Set overlay level
/// @param window Overlay window handle
/// @param level Overlay level
void NeruSetOverlayLevel(OverlayWindow window, int level) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	if ([NSThread isMainThread]) {
		[controller.window setLevel:level];
	} else {
		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.window setLevel:level];
		});
	}
}

/// Set hide unmatched cells
/// @param window Overlay window handle
/// @param hide Hide unmatched cells (1 = yes, 0 = no)
void NeruSetHideUnmatched(OverlayWindow window, int hide) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	dispatch_async(dispatch_get_main_queue(), ^{
		controller.overlayView.hideUnmatched = hide ? YES : NO;
		[controller.overlayView setNeedsDisplay:YES];
	});
}

/// Set overlay sharing type for screen sharing visibility
/// @param window Overlay window handle
/// @param sharingType Sharing type: 0 = NSWindowSharingNone (hidden), 1 = NSWindowSharingReadOnly (visible)
void NeruSetOverlaySharingType(OverlayWindow window, int sharingType) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	dispatch_async(dispatch_get_main_queue(), ^{
		controller.sharingType = sharingType;
		[controller.window setSharingType:sharingType];
	});
}

/// Draw grid cells incrementally (add/update/remove specific cells without clearing entire overlay)
/// @param window Overlay window handle
/// @param cellsToAdd Array of grid cells to add or update
/// @param addCount Number of cells to add/update
/// @param cellsToRemove Array of cell bounds to remove (by matching bounds)
/// @param removeCount Number of cells to remove
/// @param style Grid cell style (used for new/updated cells)
void NeruDrawIncrementGrid(
    OverlayWindow window, GridCell *cellsToAdd, int addCount, CGRect *cellsToRemove, int removeCount,
    GridCellStyle style) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	// Build cell data arrays upfront — safe from any thread
	NSMutableArray<GridCellItem *> *cellItemsToAdd = nil;
	if (cellsToAdd && addCount > 0) {
		cellItemsToAdd = buildGridCellItems(cellsToAdd, addCount);
	}

	// Build bounds array for cells to remove
	NSMutableArray *boundsToRemove = nil;
	if (cellsToRemove && removeCount > 0) {
		boundsToRemove = [NSMutableArray arrayWithCapacity:removeCount];
		for (int i = 0; i < removeCount; i++) {
			NSValue *boundsValue = [NSValue valueWithRect:NSRectFromCGRect(cellsToRemove[i])];
			[boundsToRemove addObject:boundsValue];
		}
	}

	// Copy all style properties NOW (before async block)
	CGFloat fontSize = style.fontSize > 0 ? style.fontSize : kDefaultGridFontSize;
	NSString *fontFamily = nil;
	if (style.fontFamily) {
		fontFamily = @(style.fontFamily);
		if (fontFamily.length == 0)
			fontFamily = nil;
	}
	NSString *bgHex = style.backgroundColor ? @(style.backgroundColor) : nil;
	NSString *labelBgHex = style.labelBackgroundColor ? @(style.labelBackgroundColor) : nil;
	NSString *textHex = style.textColor ? @(style.textColor) : nil;
	NSString *matchedTextHex = style.matchedTextColor ? @(style.matchedTextColor) : nil;
	NSString *matchedBgHex = style.matchedBackgroundColor ? @(style.matchedBackgroundColor) : nil;
	NSString *matchedBorderHex = style.matchedBorderColor ? @(style.matchedBorderColor) : nil;
	NSString *borderHex = style.borderColor ? @(style.borderColor) : nil;
	int borderWidth = style.borderWidth;
	BOOL drawLabelBackground = style.drawLabelBackground ? YES : NO;
	CGFloat labelBackgroundPaddingX = style.labelBackgroundPaddingX;
	CGFloat labelBackgroundPaddingY = style.labelBackgroundPaddingY;
	CGFloat labelBackgroundBorderRadius = style.labelBackgroundBorderRadius;
	CGFloat labelBackgroundBorderWidth = style.labelBackgroundBorderWidth;

	dispatch_async(dispatch_get_main_queue(), ^{
		// Apply font — only re-create when family or size changed
		BOOL gridFamilyChanged =
		    (fontFamily != controller.overlayView.cachedGridFontFamily &&
		     ![fontFamily isEqualToString:controller.overlayView.cachedGridFontFamily]);
		if (gridFamilyChanged || fontSize != controller.overlayView.cachedGridFontSize) {
			NSFont *font = nil;
			if (fontFamily && [fontFamily length] > 0) {
				font = [controller.overlayView resolveFont:fontFamily size:fontSize bold:NO];
			}
			if (!font)
				font = [NSFont systemFontOfSize:fontSize];
			controller.overlayView.gridFont = font;
			controller.overlayView.cachedGridFontFamily = fontFamily;
			controller.overlayView.cachedGridFontSize = fontSize;
		}

		// Apply color updates if provided
		if (bgHex) {
			controller.overlayView.gridBackgroundColor = [controller.overlayView colorFromHex:bgHex
			                                                                     defaultColor:[NSColor whiteColor]];
		}
		if (labelBgHex) {
			controller.overlayView.gridLabelBackgroundColor = [controller.overlayView
			    colorFromHex:labelBgHex
			    defaultColor:[[NSColor colorWithRed:1.0 green:0.84 blue:0.0 alpha:1.0] colorWithAlphaComponent:0.8]];
		}
		if (textHex) {
			controller.overlayView.gridTextColor = [controller.overlayView colorFromHex:textHex
			                                                               defaultColor:[NSColor blackColor]];
		}
		if (matchedTextHex) {
			controller.overlayView.gridMatchedTextColor = [controller.overlayView colorFromHex:matchedTextHex
			                                                                      defaultColor:[NSColor blueColor]];
		}
		if (matchedBgHex) {
			controller.overlayView.gridMatchedBackgroundColor =
			    [controller.overlayView colorFromHex:matchedBgHex defaultColor:[NSColor blueColor]];
		}
		if (matchedBorderHex) {
			controller.overlayView.gridMatchedBorderColor = [controller.overlayView colorFromHex:matchedBorderHex
			                                                                        defaultColor:[NSColor blueColor]];
		}
		if (borderHex) {
			controller.overlayView.gridBorderColor = [controller.overlayView colorFromHex:borderHex
			                                                                 defaultColor:[NSColor grayColor]];
		}

		// Apply geometry and layout properties unconditionally.
		// Previously borderWidth was gated behind the color guard and would be
		// skipped if only borderWidth changed without any color properties.
		if (borderWidth > 0) {
			controller.overlayView.gridBorderWidth = borderWidth;
		}
		controller.overlayView.gridDrawLabelBackground = drawLabelBackground;
		controller.overlayView.gridLabelBackgroundPaddingX = labelBackgroundPaddingX;
		controller.overlayView.gridLabelBackgroundPaddingY = labelBackgroundPaddingY;
		controller.overlayView.gridLabelBackgroundBorderRadius = labelBackgroundBorderRadius;
		controller.overlayView.gridLabelBackgroundBorderWidth = labelBackgroundBorderWidth;

		// Sync cached color references
		controller.overlayView.cachedGridTextColor = controller.overlayView.gridTextColor;
		controller.overlayView.cachedGridMatchedTextColor = controller.overlayView.gridMatchedTextColor;
		[controller.overlayView cancelGridTransition];
		[controller.overlayView cancelCursorIndicatorTransition];

		// Remove cells matching the given bounds
		if (boundsToRemove && [boundsToRemove count] > 0) {
			// Build a set of bounds keys for O(1) lookup
			NSMutableSet *boundsToRemoveSet = [NSMutableSet setWithCapacity:[boundsToRemove count]];
			for (NSValue *removeBoundsValue in boundsToRemove) {
				NSRect removeBounds = [removeBoundsValue rectValue];
				NSString *key =
				    [NSString stringWithFormat:@"%.6f,%.6f,%.6f,%.6f", removeBounds.origin.x, removeBounds.origin.y,
				                               removeBounds.size.width, removeBounds.size.height];
				[boundsToRemoveSet addObject:key];
			}

			NSMutableArray<GridCellItem *> *cellsToKeep =
			    [NSMutableArray arrayWithCapacity:[controller.overlayView.gridCells count]];
			for (GridCellItem *cellItem in controller.overlayView.gridCells) {
				NSRect cellBounds = cellItem.bounds;
				NSString *cellKey =
				    [NSString stringWithFormat:@"%.6f,%.6f,%.6f,%.6f", cellBounds.origin.x, cellBounds.origin.y,
				                               cellBounds.size.width, cellBounds.size.height];
				if (![boundsToRemoveSet containsObject:cellKey]) {
					[cellsToKeep addObject:cellItem];
				}
			}
			controller.overlayView.gridCells = cellsToKeep;
		}

		// Add or update cells
		if (cellItemsToAdd && [cellItemsToAdd count] > 0) {
			// Build lookup map for existing cells by bounds for O(1) access
			NSMutableDictionary *cellsByBounds =
			    [NSMutableDictionary dictionaryWithCapacity:[controller.overlayView.gridCells count]];
			for (GridCellItem *cellItem in controller.overlayView.gridCells) {
				NSRect bounds = cellItem.bounds;
				NSString *key = [NSString stringWithFormat:@"%.6f,%.6f,%.6f,%.6f", bounds.origin.x, bounds.origin.y,
				                                           bounds.size.width, bounds.size.height];
				cellsByBounds[key] = cellItem;
			}

			for (GridCellItem *newCellItem in cellItemsToAdd) {
				NSRect newBounds = newCellItem.bounds;
				NSString *key =
				    [NSString stringWithFormat:@"%.6f,%.6f,%.6f,%.6f", newBounds.origin.x, newBounds.origin.y,
				                               newBounds.size.width, newBounds.size.height];
				GridCellItem *existingCell = cellsByBounds[key];
				if (existingCell) {
					NSUInteger index = [controller.overlayView.gridCells indexOfObjectIdenticalTo:existingCell];
					if (index != NSNotFound) {
						controller.overlayView.gridCells[index] = newCellItem;
					}
				} else {
					[controller.overlayView.gridCells addObject:newCellItem];
				}
			}
		}

		[controller.overlayView setNeedsDisplay:YES];
	});
}

/// Show a virtual cursor indicator at the specified point.
/// @param window Overlay window handle
/// @param position Indicator center position in overlay coordinates
/// @param style Indicator style
void NeruShowCursorIndicator(OverlayWindow window, CGPoint position, CursorIndicatorStyle style) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	CGFloat radius = style.radius > 0 ? style.radius : 10.0;
	NSString *fillHex = style.fillColor ? @(style.fillColor) : nil;

	dispatch_async(dispatch_get_main_queue(), ^{
		NSPoint nextPosition = NSMakePoint(position.x, position.y);
		if (controller.overlayView.gridTransitionActive && controller.overlayView.cursorIndicatorVisible) {
			controller.overlayView.cursorIndicatorFromPosition =
			    [controller.overlayView currentCursorIndicatorPosition];
			controller.overlayView.cursorIndicatorToPosition = nextPosition;
			controller.overlayView.cursorIndicatorTransitionActive = YES;
		} else {
			[controller.overlayView cancelCursorIndicatorTransition];
		}

		controller.overlayView.cursorIndicatorVisible = YES;
		controller.overlayView.cursorIndicatorPosition = nextPosition;
		controller.overlayView.cursorIndicatorRadius = radius;
		controller.overlayView.cursorIndicatorFillColor = [controller.overlayView colorFromHex:fillHex
		                                                                          defaultColor:[NSColor whiteColor]];
		[controller.overlayView setNeedsDisplay:YES];
	});
}

/// Hide the virtual cursor indicator.
/// @param window Overlay window handle
void NeruHideCursorIndicator(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	dispatch_async(dispatch_get_main_queue(), ^{
		if (!controller.overlayView.cursorIndicatorVisible)
			return;

		NSRect dirtyRect = [controller.overlayView cursorIndicatorRect];
		[controller.overlayView cancelCursorIndicatorTransition];
		controller.overlayView.cursorIndicatorVisible = NO;
		if (NSIsEmptyRect(dirtyRect)) {
			[controller.overlayView setNeedsDisplay:YES];
			return;
		}

		controller.overlayView.fullRedraw = NO;
		[controller.overlayView setNeedsDisplayInRect:dirtyRect];
	});
}
