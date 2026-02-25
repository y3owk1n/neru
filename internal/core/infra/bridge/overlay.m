//
//  overlay.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "overlay.h"
#import <Cocoa/Cocoa.h>

#pragma mark - Helper Functions

/// Compare two rectangles with epsilon for floating point precision
static inline BOOL rectsEqual(NSRect a, NSRect b, CGFloat epsilon) {
	return fabs(a.origin.x - b.origin.x) < epsilon && fabs(a.origin.y - b.origin.y) < epsilon &&
	       fabs(a.size.width - b.size.width) < epsilon && fabs(a.size.height - b.size.height) < epsilon;
}

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

#pragma mark - Overlay View Interface

@interface OverlayView : NSView
@property(nonatomic, strong) NSMutableArray<HintItem *> *hints; ///< Hints array
@property(nonatomic, strong) NSFont *hintFont;                  ///< Hint font
@property(nonatomic, strong) NSColor *hintTextColor;            ///< Hint text color
@property(nonatomic, strong) NSColor *hintMatchedTextColor;     ///< Hint matched text color
@property(nonatomic, strong) NSColor *hintBackgroundColor;      ///< Hint background color
@property(nonatomic, strong) NSColor *hintBorderColor;          ///< Hint border color
@property(nonatomic, assign) CGFloat hintBorderRadius;          ///< Hint border radius
@property(nonatomic, assign) CGFloat hintBorderWidth;           ///< Hint border width
@property(nonatomic, assign) CGFloat hintPadding;               ///< Hint padding

@property(nonatomic, strong) NSMutableArray<GridCellItem *> *gridCells; ///< Grid cells array
@property(nonatomic, strong) NSFont *gridFont;                          ///< Grid font
@property(nonatomic, strong) NSColor *gridTextColor;                    ///< Grid text color
@property(nonatomic, strong) NSColor *gridMatchedTextColor;             ///< Grid matched text color
@property(nonatomic, strong) NSColor *gridMatchedBackgroundColor;       ///< Grid matched background color
@property(nonatomic, strong) NSColor *gridMatchedBorderColor;           ///< Grid matched border color
@property(nonatomic, strong) NSColor *gridBackgroundColor;              ///< Grid background color
@property(nonatomic, strong) NSColor *gridBorderColor;                  ///< Grid border color
@property(nonatomic, assign) CGFloat gridBorderWidth;                   ///< Grid border width
@property(nonatomic, assign) BOOL hideUnmatched;                        ///< Hide unmatched cells

// Cached grid text colors to reduce allocations during drawing
@property(nonatomic, strong) NSColor *cachedGridTextColor;
@property(nonatomic, strong) NSColor *cachedGridMatchedTextColor;

// Cached string buffers to reduce allocations (one per drawing method to avoid shared mutable state)
@property(nonatomic, strong)
    NSMutableAttributedString *cachedHintAttributedString; ///< Cached attributed string buffer for hints
@property(nonatomic, strong)
    NSMutableAttributedString *cachedGridCellAttributedString; ///< Cached attributed string buffer for grid cells

/// Cached parsed colors keyed by normalized hex string
@property(nonatomic, strong) NSCache *colorCache;

/// When YES, drawLayer:inContext: clears the full bounds and redraws everything.
/// When NO, only the dirty region (clip box) is cleared and items intersecting it are redrawn.
/// Defaults to YES; set to NO by match-prefix-only updates that use setNeedsDisplayInRect:.
@property(nonatomic, assign) BOOL fullRedraw;

- (void)applyStyle:(HintStyle)style;                                                  ///< Apply hint style
- (NSColor *)colorFromHex:(NSString *)hexString defaultColor:(NSColor *)defaultColor; ///< Color from hex string
- (CGFloat)currentBackingScaleFactor;                                                 ///< Current backing scale factor
- (NSRect)boundingRectForHint:(HintItem *)hint;           ///< Compute bounding rect for hint
- (NSRect)screenRectForGridCell:(GridCellItem *)cellItem; ///< Compute screen-space rect for grid cell
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

		_hints = [NSMutableArray arrayWithCapacity:100];     // Pre-size for typical hint count
		_gridCells = [NSMutableArray arrayWithCapacity:100]; // Pre-size for typical grid size

		_hintFont = [NSFont boldSystemFontOfSize:14.0];
		_hintTextColor = [NSColor blackColor];
		_hintMatchedTextColor = [NSColor systemBlueColor];
		_hintBackgroundColor = [[NSColor colorWithRed:1.0 green:0.84 blue:0.0 alpha:1.0] colorWithAlphaComponent:0.95];
		_hintBorderColor = [NSColor blackColor];
		_hintBorderRadius = 4.0;
		_hintBorderWidth = 1.0;
		_hintPadding = 4.0;

		// Grid defaults
		_gridFont = [NSFont fontWithName:@"Menlo" size:10.0];
		_gridTextColor = [NSColor colorWithWhite:0.2 alpha:1.0];
		_gridMatchedTextColor = [NSColor colorWithRed:0.0 green:0.4 blue:1.0 alpha:1.0];
		_gridBackgroundColor = [NSColor whiteColor];
		_gridBorderColor = [NSColor colorWithWhite:0.7 alpha:1.0];
		_gridBorderWidth = 1.0;
		_hideUnmatched = NO;

		// Initialize cached colors
		_cachedGridTextColor = _gridTextColor;
		_cachedGridMatchedTextColor = _gridMatchedTextColor;

		// Initialize cached string buffers
		_cachedHintAttributedString = [[NSMutableAttributedString alloc] initWithString:@""];
		_cachedGridCellAttributedString = [[NSMutableAttributedString alloc] initWithString:@""];

		// Initialize fullRedraw to YES for structural changes
		_fullRedraw = YES;
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
	if (self.fullRedraw) {
		// Full redraw: clear everything and draw all items
		CGContextClearRect(ctx, self.bounds);
		[self drawGridCells];
		[self drawHints];
	} else {
		// Partial redraw: only clear and redraw items intersecting the dirty region.
		// Core Animation sets the clip to the union of invalidated rects.
		CGRect clipBox = CGContextGetClipBoundingBox(ctx);
		NSRect dirtyRect = NSRectFromCGRect(clipBox);
		// If the clip box covers the full bounds, fall back to full redraw
		if (NSContainsRect(dirtyRect, self.bounds)) {
			CGContextClearRect(ctx, self.bounds);
			[self drawGridCells];
			[self drawHints];
		} else {
			// Clear only the dirty region
			CGContextClearRect(ctx, clipBox);
			// Redraw grid cells that intersect the dirty rect
			[self drawGridCellsInRect:dirtyRect];
			// Redraw hints that intersect the dirty rect
			[self drawHintsInRect:dirtyRect];
		}
	}
	// Reset to full redraw for next cycle; partial-redraw callers
	// set this to NO before calling setNeedsDisplayInRect:
	self.fullRedraw = YES;
	[NSGraphicsContext restoreGraphicsState];
}

/// Apply hint style
/// @param style Hint style
- (void)applyStyle:(HintStyle)style {
	CGFloat fontSize = style.fontSize > 0 ? style.fontSize : 14.0;
	NSString *fontFamily = nil;
	if (style.fontFamily) {
		fontFamily = [NSString stringWithUTF8String:style.fontFamily];
		if (fontFamily.length == 0) {
			fontFamily = nil;
		}
	}
	NSFont *font = nil;
	if (fontFamily.length > 0) {
		font = [NSFont fontWithName:fontFamily size:fontSize];
	}
	if (!font) {
		font = [NSFont fontWithName:@"Menlo-Bold" size:fontSize];
	}
	if (!font) {
		font = [NSFont boldSystemFontOfSize:fontSize];
	}
	self.hintFont = font;

	NSColor *defaultBg = [[NSColor colorWithRed:1.0 green:0.84 blue:0.0 alpha:1.0] colorWithAlphaComponent:0.95];
	NSColor *defaultText = [NSColor blackColor];
	NSColor *defaultMatchedText = [NSColor systemBlueColor];
	NSColor *defaultBorder = [NSColor blackColor];

	NSString *backgroundHex = style.backgroundColor ? [NSString stringWithUTF8String:style.backgroundColor] : nil;
	NSString *textHex = style.textColor ? [NSString stringWithUTF8String:style.textColor] : nil;
	NSString *matchedTextHex = style.matchedTextColor ? [NSString stringWithUTF8String:style.matchedTextColor] : nil;
	NSString *borderHex = style.borderColor ? [NSString stringWithUTF8String:style.borderColor] : nil;

	self.hintBackgroundColor = [self colorFromHex:backgroundHex defaultColor:defaultBg];
	self.hintTextColor = [self colorFromHex:textHex defaultColor:defaultText];
	self.hintMatchedTextColor = [self colorFromHex:matchedTextHex defaultColor:defaultMatchedText];
	self.hintBorderColor = [self colorFromHex:borderHex defaultColor:defaultBorder];

	self.hintBorderRadius = style.borderRadius > 0 ? style.borderRadius : 4.0;
	self.hintBorderWidth = style.borderWidth > 0 ? style.borderWidth : 1.0;
	self.hintPadding = style.padding >= 0 ? style.padding : 4.0;
}

/// Create color from hex string
/// @param hexString Hex color string
/// @param defaultColor Default color
/// @return NSColor instance
- (NSColor *)colorFromHex:(NSString *)hexString defaultColor:(NSColor *)defaultColor {
	if (!hexString || hexString.length == 0) {
		return defaultColor;
	}

	NSString *cleanString =
	    [hexString stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
	if ([cleanString hasPrefix:@"#"]) {
		cleanString = [cleanString substringFromIndex:1];
	}
	cleanString = [cleanString lowercaseString];

	// Expand 3-char hex to 6-char (e.g., f0a -> ff00aa) for consistent cache keys
	NSString *cacheKey = cleanString;
	if (cacheKey.length == 3) {
		cacheKey =
		    [NSString stringWithFormat:@"%c%c%c%c%c%c", [cacheKey characterAtIndex:0], [cacheKey characterAtIndex:0],
		                               [cacheKey characterAtIndex:1], [cacheKey characterAtIndex:1],
		                               [cacheKey characterAtIndex:2], [cacheKey characterAtIndex:2]];
	}

	NSColor *cachedColor = [self.colorCache objectForKey:cacheKey];
	if (cachedColor) {
		return cachedColor;
	}

	// Use expanded string for parsing
	cleanString = cacheKey;

	if (cleanString.length != 6 && cleanString.length != 8) {
		return defaultColor;
	}

	unsigned long long hexValue = 0;
	NSScanner *scanner = [NSScanner scannerWithString:cleanString];
	if (![scanner scanHexLongLong:&hexValue]) {
		return defaultColor;
	}

	CGFloat alpha = 1.0;
	if (cleanString.length == 8) {
		alpha = ((hexValue & 0xFF000000) >> 24) / 255.0;
	}
	CGFloat red = ((hexValue & 0x00FF0000) >> 16) / 255.0;
	CGFloat green = ((hexValue & 0x0000FF00) >> 8) / 255.0;
	CGFloat blue = (hexValue & 0x000000FF) / 255.0;

	NSColor *result = [NSColor colorWithRed:red green:green blue:blue alpha:alpha];
	[self.colorCache setObject:result forKey:cacheKey];
	return result;
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

	// Arrow dimensions
	CGFloat arrowTipX = elementCenterX;
	CGFloat arrowTipY = elementCenterY;
	CGFloat arrowBaseY = bodyRect.origin.y + bodyRect.size.height;
	CGFloat arrowWidth = arrowSize * 2.5;
	CGFloat arrowLeft = arrowTipX - arrowWidth / 2;
	CGFloat arrowRight = arrowTipX + arrowWidth / 2;

	// Clamp arrow to tooltip bounds
	CGFloat tooltipLeft = bodyRect.origin.x + self.hintBorderRadius;
	CGFloat tooltipRight = bodyRect.origin.x + bodyRect.size.width - self.hintBorderRadius;
	arrowLeft = MAX(arrowLeft, tooltipLeft);
	arrowRight = MIN(arrowRight, tooltipRight);
	arrowTipX = (arrowLeft + arrowRight) / 2;

	// Start from top-left corner
	[path moveToPoint:NSMakePoint(bodyRect.origin.x + self.hintBorderRadius, bodyRect.origin.y)];

	// Top edge
	[path lineToPoint:NSMakePoint(bodyRect.origin.x + bodyRect.size.width - self.hintBorderRadius, bodyRect.origin.y)];

	// Top-right corner
	[path appendBezierPathWithArcFromPoint:NSMakePoint(bodyRect.origin.x + bodyRect.size.width, bodyRect.origin.y)
	                               toPoint:NSMakePoint(bodyRect.origin.x + bodyRect.size.width,
	                                                   bodyRect.origin.y + self.hintBorderRadius)
	                                radius:self.hintBorderRadius];

	// Right edge
	[path lineToPoint:NSMakePoint(bodyRect.origin.x + bodyRect.size.width, arrowBaseY - self.hintBorderRadius)];

	// Bottom-right corner
	[path appendBezierPathWithArcFromPoint:NSMakePoint(bodyRect.origin.x + bodyRect.size.width, arrowBaseY)
	                               toPoint:NSMakePoint(bodyRect.origin.x + bodyRect.size.width - self.hintBorderRadius,
	                                                   arrowBaseY)
	                                radius:self.hintBorderRadius];

	// Bottom edge to arrow right side
	[path lineToPoint:NSMakePoint(arrowRight, arrowBaseY)];

	// Arrow right side to tip
	[path lineToPoint:NSMakePoint(arrowTipX, arrowTipY)];

	// Arrow left side
	[path lineToPoint:NSMakePoint(arrowLeft, arrowBaseY)];

	// Continue bottom edge to bottom-left corner
	[path lineToPoint:NSMakePoint(bodyRect.origin.x + self.hintBorderRadius, arrowBaseY)];

	// Bottom-left corner
	[path appendBezierPathWithArcFromPoint:NSMakePoint(bodyRect.origin.x, arrowBaseY)
	                               toPoint:NSMakePoint(bodyRect.origin.x, arrowBaseY - self.hintBorderRadius)
	                                radius:self.hintBorderRadius];

	// Left edge
	[path lineToPoint:NSMakePoint(bodyRect.origin.x, bodyRect.origin.y + self.hintBorderRadius)];

	// Top-left corner
	[path appendBezierPathWithArcFromPoint:NSMakePoint(bodyRect.origin.x, bodyRect.origin.y)
	                               toPoint:NSMakePoint(bodyRect.origin.x + self.hintBorderRadius, bodyRect.origin.y)
	                                radius:self.hintBorderRadius];

	[path closePath];
	return path;
}

/// Draw hint labels above target elements
- (void)drawHints {
	for (HintItem *hint in self.hints) {
		NSString *label = hint.label;
		if (!label || [label length] == 0)
			continue;
		NSPoint position = hint.position;
		int matchedPrefixLength = hint.matchedPrefixLength;
		BOOL showArrow = hint.showArrow;
		// Create attributed string with matched prefix in different color
		// Reuse cached hint attributed string buffer
		NSMutableAttributedString *attrString = self.cachedHintAttributedString;
		[[attrString mutableString] setString:label];
		// Clear previous attributes and set new ones
		NSRange fullRange = NSMakeRange(0, [label length]);
		[attrString
		    setAttributes:@{NSFontAttributeName : self.hintFont, NSForegroundColorAttributeName : self.hintTextColor}
		            range:fullRange];
		// Highlight matched prefix
		if (matchedPrefixLength > 0 && matchedPrefixLength <= [label length]) {
			[attrString addAttribute:NSForegroundColorAttributeName
			                   value:self.hintMatchedTextColor
			                   range:NSMakeRange(0, matchedPrefixLength)];
		}
		NSSize textSize = [attrString size];
		// Calculate hint box size (include arrow space if needed)
		CGFloat padding = self.hintPadding;
		CGFloat arrowHeight = showArrow ? 2.0 : 0.0;
		// Calculate dimensions - ensure box is at least square
		CGFloat contentWidth = textSize.width + (padding * 2);
		CGFloat contentHeight = textSize.height + (padding * 2);
		// Make box square if content is narrow, otherwise use content width
		CGFloat boxWidth = MAX(contentWidth, contentHeight);
		CGFloat boxHeight = contentHeight + arrowHeight;
		// Position tooltip above element with arrow pointing down to element center
		// position is already the element center (set via element.Center() in Go)
		CGFloat elementCenterX = position.x;
		CGFloat elementCenterY = position.y;
		// Position tooltip body above element (arrow points down)
		CGFloat gap = 3.0;
		CGFloat tooltipX = elementCenterX - boxWidth / 2.0;
		CGFloat tooltipY = elementCenterY + arrowHeight + gap;
		// Convert coordinates (macOS uses bottom-left origin, we need top-left)
		CGFloat screenHeight = self.bounds.size.height;
		CGFloat flippedY = screenHeight - tooltipY - boxHeight;
		CGFloat flippedElementCenterY = screenHeight - elementCenterY;
		NSRect hintRect = NSMakeRect(tooltipX, flippedY, boxWidth, boxHeight);
		// Draw tooltip background
		NSBezierPath *path;
		if (showArrow) {
			path = [self createTooltipPath:hintRect
			                     arrowSize:arrowHeight
			                elementCenterX:elementCenterX
			                elementCenterY:flippedElementCenterY];
		} else {
			path = [NSBezierPath bezierPathWithRoundedRect:hintRect
			                                       xRadius:self.hintBorderRadius
			                                       yRadius:self.hintBorderRadius];
		}
		[self.hintBackgroundColor setFill];
		[path fill];
		[self.hintBorderColor setStroke];
		[path setLineWidth:self.hintBorderWidth];
		[path stroke];
		// Draw text (centered in tooltip body)
		CGFloat textX = hintRect.origin.x + (boxWidth - textSize.width) / 2.0;
		CGFloat textY = hintRect.origin.y + padding;
		[attrString drawAtPoint:NSMakePoint(textX, textY)];
	}
}

/// Draw grid cells with labels and borders
- (void)drawGridCells {
	if ([self.gridCells count] == 0)
		return;
	CGFloat screenHeight = self.bounds.size.height;
	CGFloat screenWidth = self.bounds.size.width;
	for (GridCellItem *cellItem in self.gridCells) {
		NSString *label = cellItem.label;
		CGRect bounds = cellItem.bounds;
		BOOL isMatched = cellItem.isMatched;
		BOOL isSubgrid = cellItem.isSubgrid;
		// Skip drawing unmatched cells if hideUnmatched is enabled AND it's not a subgrid cell
		if (self.hideUnmatched && !isMatched && !isSubgrid) {
			continue;
		}
		// Convert coordinates (macOS uses bottom-left origin)
		CGFloat flippedY = screenHeight - bounds.origin.y - bounds.size.height;
		NSRect cellRect = NSMakeRect(bounds.origin.x, flippedY, bounds.size.width, bounds.size.height);
		// Draw cell background
		NSColor *bgBase = self.gridBackgroundColor;
		if (isMatched && self.gridMatchedBackgroundColor) {
			bgBase = self.gridMatchedBackgroundColor;
		}
		[bgBase setFill];
		NSRectFill(cellRect);
		// Draw cell border
		NSColor *borderColor = self.gridBorderColor;
		if (isMatched && self.gridMatchedBorderColor) {
			borderColor = self.gridMatchedBorderColor;
		}
		[borderColor setStroke];
		NSRect borderRect = cellRect;
		// For odd border widths (like 1.0), offset by 0.5 to ensure crisp lines
		// and proper overlap at shared edges.
		if ((int)self.gridBorderWidth % 2 == 1) {
			borderRect = NSOffsetRect(cellRect, 0.5, -0.5);
		}
		// Adjust for right screen edge to ensure border is visible
		if (NSMaxX(cellRect) >= screenWidth) {
			borderRect.size.width -= 1.0;
		}
		// Adjust for bottom screen edge to ensure border is visible
		if (NSMinY(cellRect) <= 0) {
			borderRect.origin.y += ceil(self.gridBorderWidth / 2.0);
			borderRect.size.height -= ceil(self.gridBorderWidth / 2.0);
		}
		NSBezierPath *borderPath = [NSBezierPath bezierPathWithRect:borderRect];
		[borderPath setLineWidth:self.gridBorderWidth];
		[borderPath stroke];
		// Draw text label centered in cell
		if (label && [label length] > 0) {
			// Reuse cached grid cell attributed string buffer
			NSMutableAttributedString *attrString = self.cachedGridCellAttributedString;
			[[attrString mutableString] setString:label];
			// Clear previous attributes and set new ones
			NSRange fullRange = NSMakeRange(0, [label length]);
			[attrString setAttributes:@{NSFontAttributeName : self.gridFont} range:fullRange];
			// Use cached color to avoid repeated allocations
			[attrString addAttribute:NSForegroundColorAttributeName value:self.cachedGridTextColor range:fullRange];
			int matchedPrefixLength = cellItem.matchedPrefixLength;
			if (isMatched && matchedPrefixLength > 0 && matchedPrefixLength <= [label length]) {
				// Use cached matched color
				[attrString addAttribute:NSForegroundColorAttributeName
				                   value:self.cachedGridMatchedTextColor
				                   range:NSMakeRange(0, matchedPrefixLength)];
			}
			NSSize textSize = [attrString size];
			CGFloat textX = cellRect.origin.x + (cellRect.size.width - textSize.width) / 2.0;
			CGFloat textY = cellRect.origin.y + (cellRect.size.height - textSize.height) / 2.0;
			[attrString drawAtPoint:NSMakePoint(textX, textY)];
		}
	}
}

/// Compute the screen-space bounding rect for a hint item (view coordinates, bottom-left origin).
/// Mirrors the geometry logic in drawHints so callers can determine dirty rects without drawing.
/// @param hint Hint item
/// @return Bounding rectangle including border and arrow
- (NSRect)boundingRectForHint:(HintItem *)hint {
	NSString *label = hint.label;
	if (!label || [label length] == 0)
		return NSZeroRect;
	// Measure text size using the same attributed string approach as drawHints/drawHintsInRect
	NSMutableAttributedString *attrString = self.cachedHintAttributedString;
	[[attrString mutableString] setString:label];
	[attrString
	    setAttributes:@{NSFontAttributeName : self.hintFont, NSForegroundColorAttributeName : self.hintTextColor}
	            range:NSMakeRange(0, [label length])];
	NSSize textSize = [attrString size];
	CGFloat padding = self.hintPadding;
	CGFloat arrowHeight = hint.showArrow ? 2.0 : 0.0;
	CGFloat contentWidth = textSize.width + (padding * 2);
	CGFloat contentHeight = textSize.height + (padding * 2);
	CGFloat boxWidth = MAX(contentWidth, contentHeight);
	CGFloat boxHeight = contentHeight + arrowHeight;
	NSPoint position = hint.position;
	CGFloat elementCenterX = position.x;
	CGFloat elementCenterY = position.y;
	CGFloat gap = 3.0;
	CGFloat tooltipX = elementCenterX - boxWidth / 2.0;
	CGFloat tooltipY = elementCenterY + arrowHeight + gap;
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
	CGFloat expand = ceil(self.gridBorderWidth / 2.0) + 1.0;
	return NSMakeRect(bounds.origin.x - expand, flippedY - expand, bounds.size.width + expand * 2,
	                  bounds.size.height + expand * 2);
}
/// Draw only hint labels whose bounding rects intersect the given dirty rect.
/// @param dirtyRect The dirty region to redraw
- (void)drawHintsInRect:(NSRect)dirtyRect {
	for (HintItem *hint in self.hints) {
		NSString *label = hint.label;
		if (!label || [label length] == 0)
			continue;
		// Skip hints outside the dirty region
		NSRect hintBounds = [self boundingRectForHint:hint];
		if (!NSIntersectsRect(hintBounds, dirtyRect))
			continue;
		// --- Identical drawing logic to drawHints ---
		NSPoint position = hint.position;
		int matchedPrefixLength = hint.matchedPrefixLength;
		BOOL showArrow = hint.showArrow;
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
		NSSize textSize = [attrString size];
		CGFloat padding = self.hintPadding;
		CGFloat arrowHeight = showArrow ? 2.0 : 0.0;
		CGFloat contentWidth = textSize.width + (padding * 2);
		CGFloat contentHeight = textSize.height + (padding * 2);
		CGFloat boxWidth = MAX(contentWidth, contentHeight);
		CGFloat boxHeight = contentHeight + arrowHeight;
		CGFloat elementCenterX = position.x;
		CGFloat elementCenterY = position.y;
		CGFloat gap = 3.0;
		CGFloat tooltipX = elementCenterX - boxWidth / 2.0;
		CGFloat tooltipY = elementCenterY + arrowHeight + gap;
		CGFloat screenHeight = self.bounds.size.height;
		CGFloat flippedY = screenHeight - tooltipY - boxHeight;
		CGFloat flippedElementCenterY = screenHeight - elementCenterY;
		NSRect hintRect = NSMakeRect(tooltipX, flippedY, boxWidth, boxHeight);
		NSBezierPath *path;
		if (showArrow) {
			path = [self createTooltipPath:hintRect
			                     arrowSize:arrowHeight
			                elementCenterX:elementCenterX
			                elementCenterY:flippedElementCenterY];
		} else {
			path = [NSBezierPath bezierPathWithRoundedRect:hintRect
			                                       xRadius:self.hintBorderRadius
			                                       yRadius:self.hintBorderRadius];
		}
		[self.hintBackgroundColor setFill];
		[path fill];
		[self.hintBorderColor setStroke];
		[path setLineWidth:self.hintBorderWidth];
		[path stroke];
		CGFloat textX = hintRect.origin.x + (boxWidth - textSize.width) / 2.0;
		CGFloat textY = hintRect.origin.y + padding;
		[attrString drawAtPoint:NSMakePoint(textX, textY)];
	}
}
/// Draw only grid cells whose bounding rects intersect the given dirty rect.
/// @param dirtyRect The dirty region to redraw
- (void)drawGridCellsInRect:(NSRect)dirtyRect {
	if ([self.gridCells count] == 0)
		return;
	CGFloat screenHeight = self.bounds.size.height;
	CGFloat screenWidth = self.bounds.size.width;
	for (GridCellItem *cellItem in self.gridCells) {
		NSString *label = cellItem.label;
		CGRect bounds = cellItem.bounds;
		BOOL isMatched = cellItem.isMatched;
		BOOL isSubgrid = cellItem.isSubgrid;
		if (self.hideUnmatched && !isMatched && !isSubgrid) {
			continue;
		}
		CGFloat flippedY = screenHeight - bounds.origin.y - bounds.size.height;
		NSRect cellRect = NSMakeRect(bounds.origin.x, flippedY, bounds.size.width, bounds.size.height);
		// Skip cells outside the dirty region
		if (!NSIntersectsRect(cellRect, dirtyRect))
			continue;
		// --- Identical drawing logic to drawGridCells ---
		NSColor *bgBase = self.gridBackgroundColor;
		if (isMatched && self.gridMatchedBackgroundColor) {
			bgBase = self.gridMatchedBackgroundColor;
		}
		[bgBase setFill];
		NSRectFill(cellRect);
		NSColor *borderColor = self.gridBorderColor;
		if (isMatched && self.gridMatchedBorderColor) {
			borderColor = self.gridMatchedBorderColor;
		}
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
		if (label && [label length] > 0) {
			NSMutableAttributedString *attrString = self.cachedGridCellAttributedString;
			[[attrString mutableString] setString:label];
			NSRange fullRange = NSMakeRange(0, [label length]);
			[attrString setAttributes:@{NSFontAttributeName : self.gridFont} range:fullRange];
			[attrString addAttribute:NSForegroundColorAttributeName value:self.cachedGridTextColor range:fullRange];
			int matchedPrefixLength = cellItem.matchedPrefixLength;
			if (isMatched && matchedPrefixLength > 0 && matchedPrefixLength <= [label length]) {
				[attrString addAttribute:NSForegroundColorAttributeName
				                   value:self.cachedGridMatchedTextColor
				                   range:NSMakeRange(0, matchedPrefixLength)];
			}
			NSSize textSize = [attrString size];
			CGFloat textX = cellRect.origin.x + (cellRect.size.width - textSize.width) / 2.0;
			CGFloat textY = cellRect.origin.y + (cellRect.size.height - textSize.height) / 2.0;
			[attrString drawAtPoint:NSMakePoint(textX, textY)];
		}
	}
}

@end

#pragma mark - Overlay Window Controller Interface

@interface OverlayWindowController : NSObject
@property(nonatomic, strong) NSPanel *window;          ///< Panel instance (non-activating overlay)
@property(nonatomic, strong) OverlayView *overlayView; ///< Overlay view instance
@property(nonatomic, assign) NSInteger sharingType;    ///< Current window sharing type
@property(nonatomic, assign) BOOL sharingTypeExplicit; ///< Whether sharingType was explicitly configured
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

	// Use NSPanel for better floating overlay behavior
	// Non-activating panel won't steal focus from other apps
	NSPanel *panel =
	    [[NSPanel alloc] initWithContentRect:screenFrame
	                               styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
	                                 backing:NSBackingStoreBuffered
	                                   defer:NO];

	[panel setHidesOnDeactivate:NO];
	[panel setReleasedWhenClosed:NO];

	self.window = panel;

	if ([self.window respondsToSelector:@selector(setAnimationBehavior:)]) {
		[self.window setAnimationBehavior:NSWindowAnimationBehaviorNone];
	}
	[self.window setAnimations:@{}];
	[self.window setAlphaValue:1.0];

	[self.window setLevel:NSScreenSaverWindowLevel];
	[self.window setOpaque:NO];
	[self.window setBackgroundColor:[NSColor clearColor]];
	[self.window setIgnoresMouseEvents:YES];
	[self.window setAcceptsMouseMovedEvents:NO];
	[self.window setHasShadow:NO];
	[self.window
	    setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary |
	                          NSWindowCollectionBehaviorFullScreenAuxiliary | NSWindowCollectionBehaviorIgnoresCycle];

	// Set sharing type - default to visible (NSWindowSharingReadOnly = 1) unless explicitly configured
	if (!self.sharingTypeExplicit) {
		self.sharingType = NSWindowSharingReadOnly;
	}
	[self.window setSharingType:self.sharingType];

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

	return (__bridge_retained void *)controller; // Transfer ownership to caller
}

/// Destroy overlay window
/// @param window Overlay window handle
void NeruDestroyOverlayWindow(OverlayWindow window) {
	if (!window) {
		return;
	}

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	if ([NSThread isMainThread]) {
		[controller.window close];
		CFRelease(window); // Balance the CFBridgingRetain from createOverlayWindow
	} else {
		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.window close];
			CFRelease(window); // Balance the CFBridgingRetain from createOverlayWindow
		});
	}
}

/// Show overlay window
/// @param window Overlay window handle
void NeruShowOverlayWindow(OverlayWindow window) {
	if (!window) {
		return;
	}

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
	if (!window) {
		return;
	}

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
	if (!window) {
		return;
	}

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	if ([NSThread isMainThread]) {
		[controller.overlayView.hints removeAllObjects];
		[controller.overlayView.gridCells removeAllObjects];

		[controller.overlayView setNeedsDisplay:YES];
	} else {
		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.overlayView.hints removeAllObjects];
			[controller.overlayView.gridCells removeAllObjects];

			[controller.overlayView setNeedsDisplay:YES];
		});
	}
}

/// Resize overlay to main screen
/// @param window Overlay window handle
void NeruResizeOverlayToMainScreen(OverlayWindow window) {
	if (!window) {
		return;
	}

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	dispatch_async(dispatch_get_main_queue(), ^{
		NSScreen *mainScreen = [NSScreen mainScreen];
		if (!mainScreen) {
			return;
		}
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
	if (!window) {
		return;
	}

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	dispatch_async(dispatch_get_main_queue(), ^{
		NSPoint mouseLoc = [NSEvent mouseLocation];

		NSScreen *activeScreen = nil;
		for (NSScreen *screen in [NSScreen screens]) {
			if (NSPointInRect(mouseLoc, screen.frame)) {
				activeScreen = screen;
				break;
			}
		}

		if (!activeScreen) {
			activeScreen = [NSScreen mainScreen];
		}

		if (!activeScreen) {
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
		});
	});
}

/// Resize overlay to active screen with callback
/// @param window Overlay window handle
/// @param callback Completion callback
/// @param context Callback context
void NeruResizeOverlayToActiveScreenWithCallback(OverlayWindow window, ResizeCompletionCallback callback,
                                                 void *context) {
	if (!window) {
		if (callback) {
			callback(context);
		}
		return;
	}

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	dispatch_async(dispatch_get_main_queue(), ^{
		NSPoint mouseLoc = [NSEvent mouseLocation];

		NSScreen *activeScreen = nil;
		for (NSScreen *screen in [NSScreen screens]) {
			if (NSPointInRect(mouseLoc, screen.frame)) {
				activeScreen = screen;
				break;
			}
		}

		if (!activeScreen) {
			activeScreen = [NSScreen mainScreen];
		}

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

			if (callback) {
				callback(context);
			}
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

/// Draw hints
/// @param window Overlay window handle
/// @param hints Array of hint data
/// @param count Number of hints
/// @param style Hint style
void NeruDrawHints(OverlayWindow window, HintData *hints, int count, HintStyle style) {
	if (!window || !hints)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	if ([NSThread isMainThread]) {
		[controller.overlayView.hints removeAllObjects];
		[controller.overlayView applyStyle:style];

		for (int i = 0; i < count; i++) {
			HintData hint = hints[i];
			HintItem *hintItem = [[HintItem alloc] init];
			hintItem.label = hint.label ? @(hint.label) : @"";
			hintItem.position = hint.position;
			hintItem.matchedPrefixLength = hint.matchedPrefixLength;
			hintItem.showArrow = style.showArrow ? YES : NO;
			[controller.overlayView.hints addObject:hintItem];
		}

		[controller.overlayView setNeedsDisplay:YES];
	} else {
		NSMutableArray<HintItem *> *hintItems = [NSMutableArray arrayWithCapacity:count];
		for (int i = 0; i < count; i++) {
			HintData hint = hints[i];
			HintItem *hintItem = [[HintItem alloc] init];
			hintItem.label = hint.label ? @(hint.label) : @"";
			hintItem.position = hint.position;
			hintItem.matchedPrefixLength = hint.matchedPrefixLength;
			hintItem.showArrow = style.showArrow ? YES : NO;
			[hintItems addObject:hintItem];
		}

		HintStyle styleCopy = {.fontSize = style.fontSize,
		                       .borderRadius = style.borderRadius,
		                       .borderWidth = style.borderWidth,
		                       .padding = style.padding,
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
		BOOL anyChanged = NO;
		for (HintItem *hintItem in controller.overlayView.hints) {
			NSString *label = hintItem.label ?: @"";
			int newMatchedPrefixLength = 0;
			if ([prefixStr length] > 0 && [label length] >= [prefixStr length]) {
				NSString *lblPrefix = [label substringToIndex:[prefixStr length]];
				if ([lblPrefix isEqualToString:prefixStr]) {
					newMatchedPrefixLength = (int)[prefixStr length];
				}
			}
			// Only invalidate if the match state actually changed
			if (hintItem.matchedPrefixLength != newMatchedPrefixLength) {
				hintItem.matchedPrefixLength = newMatchedPrefixLength;
				NSRect dirtyRect = [controller.overlayView boundingRectForHint:hintItem];
				if (!NSIsEmptyRect(dirtyRect)) {
					[controller.overlayView setNeedsDisplayInRect:dirtyRect];
				}
				anyChanged = YES;
			}
		}
		if (anyChanged) {
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
void NeruDrawIncrementHints(OverlayWindow window, HintData *hintsToAdd, int addCount, CGPoint *positionsToRemove,
                            int removeCount, HintStyle style) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	// Build hint data arrays for hints to add/update
	NSMutableArray<HintItem *> *hintItemsToAdd = nil;
	if (hintsToAdd && addCount > 0) {
		hintItemsToAdd = [NSMutableArray arrayWithCapacity:addCount];
		for (int i = 0; i < addCount; i++) {
			HintData hint = hintsToAdd[i];
			HintItem *hintItem = [[HintItem alloc] init];
			hintItem.label = hint.label ? @(hint.label) : @"";
			hintItem.position = hint.position;
			hintItem.matchedPrefixLength = hint.matchedPrefixLength;
			hintItem.showArrow = style.showArrow ? YES : NO;
			[hintItemsToAdd addObject:hintItem];
		}
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
	CGFloat fontSize = style.fontSize > 0 ? style.fontSize : 14.0;
	NSString *fontFamily = style.fontFamily ? @(style.fontFamily) : nil;
	NSString *bgHex = style.backgroundColor ? @(style.backgroundColor) : nil;
	NSString *textHex = style.textColor ? @(style.textColor) : nil;
	NSString *matchedTextHex = style.matchedTextColor ? @(style.matchedTextColor) : nil;
	NSString *borderHex = style.borderColor ? @(style.borderColor) : nil;
	int borderRadius = style.borderRadius;
	int borderWidth = style.borderWidth;
	int padding = style.padding;

	dispatch_async(dispatch_get_main_queue(), ^{
		// Apply style updates
		NSFont *font = controller.overlayView.hintFont;
		if (fontFamily && [fontFamily length] > 0) {
			font = [NSFont fontWithName:fontFamily size:fontSize];
		}
		if (!font) {
			font = [NSFont fontWithName:@"Menlo-Bold" size:fontSize];
		}
		if (!font) {
			font = [NSFont boldSystemFontOfSize:fontSize];
		}
		controller.overlayView.hintFont = font;

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
		if (borderRadius > 0) {
			controller.overlayView.hintBorderRadius = borderRadius;
		}
		if (borderWidth > 0) {
			controller.overlayView.hintBorderWidth = borderWidth;
		}
		if (padding >= 0) {
			controller.overlayView.hintPadding = padding;
		}

		// Remove hints that match the positions to remove
		if (positionsToRemoveArray && [positionsToRemoveArray count] > 0) {
			// Create a set of position keys for O(1) lookup
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
				BOOL shouldRemove = [positionsToRemoveSet containsObject:hintKey];

				if (!shouldRemove) {
					[hintsToKeep addObject:hintItem];
				}
			}
			controller.overlayView.hints = hintsToKeep;
		}

		// Add or update hints
		if (hintItemsToAdd && [hintItemsToAdd count] > 0) {
			// Build lookup map for existing hints by position
			NSMutableDictionary *hintsByPosition =
			    [NSMutableDictionary dictionaryWithCapacity:[controller.overlayView.hints count]];
			for (HintItem *hintItem in controller.overlayView.hints) {
				NSPoint pos = hintItem.position;
				NSString *key = [NSString stringWithFormat:@"%.6f,%.6f", pos.x, pos.y];
				hintsByPosition[key] = hintItem;
			}

			// For each hint to add/update, check if it already exists (by position) and replace it, otherwise add it
			for (HintItem *newHintItem in hintItemsToAdd) {
				NSPoint newPosition = newHintItem.position;
				NSString *key = [NSString stringWithFormat:@"%.6f,%.6f", newPosition.x, newPosition.y];

				HintItem *existingHint = hintsByPosition[key];
				if (existingHint) {
					// Replace existing hint (use identity lookup since the map stores the same pointers)
					NSUInteger index = [controller.overlayView.hints indexOfObjectIdenticalTo:existingHint];
					if (index != NSNotFound) {
						controller.overlayView.hints[index] = newHintItem;
					}
				} else {
					// Add as new hint
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
		NSInteger sharingType = NSWindowSharingReadOnly; // Default to visible
		if (oldController) {
			sharingType = oldController.sharingType;
		}
		OverlayWindowController *newController = [[OverlayWindowController alloc] init];
		newController.sharingType = sharingType;
		newController.sharingTypeExplicit = YES;
		[newController.window setSharingType:sharingType];
		if (oldController) {
			[oldController.window close];
			CFRelease(*pwindow); // Balance the CFBridgingRetain from createOverlayWindow
		}
		*pwindow = (__bridge_retained void *)newController; // Transfer ownership to caller
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

	// Build cell data array and copy all strings NOW
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

	// Copy all style properties NOW (before async block)
	CGFloat fontSize = style.fontSize > 0 ? style.fontSize : 10.0;
	NSString *fontFamily = style.fontFamily ? @(style.fontFamily) : nil;
	NSString *bgHex = style.backgroundColor ? @(style.backgroundColor) : nil;
	NSString *textHex = style.textColor ? @(style.textColor) : nil;
	NSString *matchedTextHex = style.matchedTextColor ? @(style.matchedTextColor) : nil;
	NSString *matchedBgHex = style.matchedBackgroundColor ? @(style.matchedBackgroundColor) : nil;
	NSString *matchedBorderHex = style.matchedBorderColor ? @(style.matchedBorderColor) : nil;
	NSString *borderHex = style.borderColor ? @(style.borderColor) : nil;
	int borderWidth = style.borderWidth;
	dispatch_async(dispatch_get_main_queue(), ^{
		// Apply style
		NSFont *font = nil;
		if (fontFamily && [fontFamily length] > 0) {
			font = [NSFont fontWithName:fontFamily size:fontSize];
		}
		if (!font) {
			font = [NSFont fontWithName:@"Menlo" size:fontSize];
		}
		if (!font) {
			font = [NSFont systemFontOfSize:fontSize];
		}
		controller.overlayView.gridFont = font;

		controller.overlayView.gridBackgroundColor = [controller.overlayView colorFromHex:bgHex
		                                                                     defaultColor:[NSColor whiteColor]];
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

		controller.overlayView.cachedGridTextColor = controller.overlayView.gridTextColor;
		controller.overlayView.cachedGridMatchedTextColor = controller.overlayView.gridMatchedTextColor;

		[controller.overlayView.gridCells removeAllObjects];
		[controller.overlayView.gridCells addObjectsFromArray:cellItems];
		[controller.overlayView setNeedsDisplay:YES];
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
		BOOL anyMatchStateChanged = NO;
		// First pass: update all cells and track which changed
		// Use a stack-allocated array for small counts, heap for large
		BOOL stackFlags[256];
		BOOL *changedFlags = cellCount <= 256 ? stackFlags : (BOOL *)calloc(cellCount, sizeof(BOOL));
		NSUInteger changedCount = 0;
		NSUInteger idx = 0;
		for (GridCellItem *cellItem in view.gridCells) {
			NSString *label = cellItem.label ?: @"";
			BOOL newIsMatched = NO;
			int newMatchedPrefixLength = 0;
			if ([prefixStr length] > 0 && [label length] >= [prefixStr length]) {
				NSString *lblPrefix = [label substringToIndex:[prefixStr length]];
				newIsMatched = [lblPrefix isEqualToString:prefixStr];
				if (newIsMatched) {
					newMatchedPrefixLength = (int)[prefixStr length];
				}
			}
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
		// If hideUnmatched is active and cells toggled visibility, full redraw needed
		if (view.hideUnmatched && anyMatchStateChanged) {
			if (changedFlags != stackFlags)
				free(changedFlags);
			[view setNeedsDisplay:YES];
			return;
		}
		// Partial redraw: only invalidate changed cells
		view.fullRedraw = NO;
		idx = 0;
		for (GridCellItem *cellItem in view.gridCells) {
			if (changedFlags[idx]) {
				NSRect dirtyRect = [view screenRectForGridCell:cellItem];
				if (!NSIsEmptyRect(dirtyRect)) {
					[view setNeedsDisplayInRect:dirtyRect];
				}
			}
			idx++;
		}
		if (changedFlags != stackFlags)
			free(changedFlags);
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
void NeruDrawIncrementGrid(OverlayWindow window, GridCell *cellsToAdd, int addCount, CGRect *cellsToRemove,
                           int removeCount, GridCellStyle style) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	// Build cell data arrays for cells to add/update
	NSMutableArray<GridCellItem *> *cellItemsToAdd = nil;
	if (cellsToAdd && addCount > 0) {
		cellItemsToAdd = [NSMutableArray arrayWithCapacity:addCount];
		for (int i = 0; i < addCount; i++) {
			GridCell cell = cellsToAdd[i];
			GridCellItem *cellItem = [[GridCellItem alloc] init];
			cellItem.label = cell.label ? @(cell.label) : @"";
			cellItem.bounds = cell.bounds;
			cellItem.isMatched = cell.isMatched ? YES : NO;
			cellItem.isSubgrid = cell.isSubgrid ? YES : NO;
			cellItem.matchedPrefixLength = cell.matchedPrefixLength;
			[cellItemsToAdd addObject:cellItem];
		}
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
	CGFloat fontSize = style.fontSize > 0 ? style.fontSize : 10.0;
	NSString *fontFamily = style.fontFamily ? @(style.fontFamily) : nil;
	NSString *bgHex = style.backgroundColor ? @(style.backgroundColor) : nil;
	NSString *textHex = style.textColor ? @(style.textColor) : nil;
	NSString *matchedTextHex = style.matchedTextColor ? @(style.matchedTextColor) : nil;
	NSString *matchedBgHex = style.matchedBackgroundColor ? @(style.matchedBackgroundColor) : nil;
	NSString *matchedBorderHex = style.matchedBorderColor ? @(style.matchedBorderColor) : nil;
	NSString *borderHex = style.borderColor ? @(style.borderColor) : nil;
	int borderWidth = style.borderWidth;
	dispatch_async(dispatch_get_main_queue(), ^{
		// Apply style if provided (only update if style properties are non-null)
		if (fontFamily || bgHex || textHex || matchedTextHex || matchedBgHex || matchedBorderHex || borderHex) {
			NSFont *font = controller.overlayView.gridFont;
			if (fontFamily && [fontFamily length] > 0) {
				font = [NSFont fontWithName:fontFamily size:fontSize];
			}
			if (!font) {
				font = [NSFont fontWithName:@"Menlo" size:fontSize];
			}
			if (!font) {
				font = [NSFont systemFontOfSize:fontSize];
			}
			controller.overlayView.gridFont = font;

			if (bgHex) {
				controller.overlayView.gridBackgroundColor = [controller.overlayView colorFromHex:bgHex
				                                                                     defaultColor:[NSColor whiteColor]];
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
				controller.overlayView.gridMatchedBorderColor =
				    [controller.overlayView colorFromHex:matchedBorderHex defaultColor:[NSColor blueColor]];
			}
			if (borderHex) {
				controller.overlayView.gridBorderColor = [controller.overlayView colorFromHex:borderHex
				                                                                 defaultColor:[NSColor grayColor]];
			}
			if (borderWidth > 0) {
				controller.overlayView.gridBorderWidth = borderWidth;
			}

			controller.overlayView.cachedGridTextColor = controller.overlayView.gridTextColor;
			controller.overlayView.cachedGridMatchedTextColor = controller.overlayView.gridMatchedTextColor;
		}

		// Remove cells that match the bounds to remove
		if (boundsToRemove && [boundsToRemove count] > 0) {
			NSMutableArray<GridCellItem *> *cellsToKeep =
			    [NSMutableArray arrayWithCapacity:[controller.overlayView.gridCells count]];
			for (GridCellItem *cellItem in controller.overlayView.gridCells) {
				NSRect cellBounds = cellItem.bounds;
				BOOL shouldRemove = NO;

				// Check if this cell's bounds match any of the bounds to remove
				for (NSValue *removeBoundsValue in boundsToRemove) {
					NSRect removeBounds = [removeBoundsValue rectValue];
					// Use rectsEqual for floating point comparison
					if (rectsEqual(cellBounds, removeBounds, 0.1)) {
						shouldRemove = YES;
						break;
					}
				}

				if (!shouldRemove) {
					[cellsToKeep addObject:cellItem];
				}
			}
			controller.overlayView.gridCells = cellsToKeep;
		}

		// Add or update cells
		if (cellItemsToAdd && [cellItemsToAdd count] > 0) {
			// Build lookup map for existing cells by bounds
			NSMutableDictionary *cellsByBounds =
			    [NSMutableDictionary dictionaryWithCapacity:[controller.overlayView.gridCells count]];
			for (GridCellItem *cellItem in controller.overlayView.gridCells) {
				NSRect bounds = cellItem.bounds;
				NSString *key = [NSString stringWithFormat:@"%.1f,%.1f,%.1f,%.1f", bounds.origin.x, bounds.origin.y,
				                                           bounds.size.width, bounds.size.height];
				cellsByBounds[key] = cellItem;
			}

			// For each cell to add/update, check if it already exists (by bounds) and replace it, otherwise add it
			for (GridCellItem *newCellItem in cellItemsToAdd) {
				NSRect newBounds = newCellItem.bounds;
				NSString *key =
				    [NSString stringWithFormat:@"%.1f,%.1f,%.1f,%.1f", newBounds.origin.x, newBounds.origin.y,
				                               newBounds.size.width, newBounds.size.height];

				GridCellItem *existingCell = cellsByBounds[key];
				if (existingCell) {
					// Replace existing cell (use identity lookup since the map stores the same pointers)
					NSUInteger index = [controller.overlayView.gridCells indexOfObjectIdenticalTo:existingCell];
					if (index != NSNotFound) {
						controller.overlayView.gridCells[index] = newCellItem;
					}
				} else {
					// Add as new cell
					[controller.overlayView.gridCells addObject:newCellItem];
				}
			}
		}

		[controller.overlayView setNeedsDisplay:YES];
	});
}
