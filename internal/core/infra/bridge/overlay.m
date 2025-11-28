//
//  overlay.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "overlay.h"
#import <Cocoa/Cocoa.h>

#pragma mark - Overlay View Interface

@interface OverlayView : NSView
@property(nonatomic, strong) NSMutableArray *hints;               ///< Hints array
@property(nonatomic, strong) NSFont *hintFont;                    ///< Hint font
@property(nonatomic, strong) NSColor *hintTextColor;              ///< Hint text color
@property(nonatomic, strong) NSColor *hintMatchedTextColor;       ///< Hint matched text color
@property(nonatomic, strong) NSColor *hintBackgroundColor;        ///< Hint background color
@property(nonatomic, strong) NSColor *hintBorderColor;            ///< Hint border color
@property(nonatomic, assign) CGFloat hintBorderRadius;            ///< Hint border radius
@property(nonatomic, assign) CGFloat hintBorderWidth;             ///< Hint border width
@property(nonatomic, assign) CGFloat hintPadding;                 ///< Hint padding
@property(nonatomic, assign) CGRect scrollHighlight;              ///< Scroll highlight bounds
@property(nonatomic, strong) NSColor *scrollHighlightColor;       ///< Scroll highlight color
@property(nonatomic, assign) int scrollHighlightWidth;            ///< Scroll highlight width
@property(nonatomic, assign) BOOL showScrollHighlight;            ///< Show scroll highlight
@property(nonatomic, assign) BOOL showTargetDot;                  ///< Show target dot
@property(nonatomic, assign) CGPoint targetDotCenter;             ///< Target dot center
@property(nonatomic, assign) CGFloat targetDotRadius;             ///< Target dot radius
@property(nonatomic, strong) NSColor *targetDotBackgroundColor;   ///< Target dot background color
@property(nonatomic, strong) NSColor *targetDotBorderColor;       ///< Target dot border color
@property(nonatomic, assign) CGFloat targetDotBorderWidth;        ///< Target dot border width
@property(nonatomic, strong) NSMutableArray *gridCells;           ///< Grid cells array
@property(nonatomic, strong) NSMutableArray *gridLines;           ///< Grid lines array
@property(nonatomic, strong) NSFont *gridFont;                    ///< Grid font
@property(nonatomic, strong) NSColor *gridTextColor;              ///< Grid text color
@property(nonatomic, strong) NSColor *gridMatchedTextColor;       ///< Grid matched text color
@property(nonatomic, strong) NSColor *gridMatchedBackgroundColor; ///< Grid matched background color
@property(nonatomic, strong) NSColor *gridMatchedBorderColor;     ///< Grid matched border color
@property(nonatomic, strong) NSColor *gridBackgroundColor;        ///< Grid background color
@property(nonatomic, strong) NSColor *gridBorderColor;            ///< Grid border color
@property(nonatomic, assign) CGFloat gridBorderWidth;             ///< Grid border width
@property(nonatomic, assign) CGFloat gridBackgroundOpacity;       ///< Grid background opacity
@property(nonatomic, assign) CGFloat gridTextOpacity;             ///< Grid text opacity
@property(nonatomic, assign) BOOL hideUnmatched;                  ///< Hide unmatched cells

// Cached colors with opacity to reduce allocations during drawing
@property(nonatomic, strong) NSColor *cachedGridTextColorWithOpacity;        ///< Cached grid text color with opacity
@property(nonatomic, strong) NSColor *cachedGridMatchedTextColorWithOpacity; ///< Cached matched text color with opacity

// Cached string buffer to reduce allocations
@property(nonatomic, strong) NSMutableAttributedString *cachedAttributedString; ///< Cached attributed string buffer

- (void)applyStyle:(HintStyle)style;                                                  ///< Apply hint style
- (NSColor *)colorFromHex:(NSString *)hexString defaultColor:(NSColor *)defaultColor; ///< Color from hex string
@end

#pragma mark - Overlay View Implementation

@implementation OverlayView

/// Initialize with frame
/// @param frame View frame
/// @return Initialized instance
- (instancetype)initWithFrame:(NSRect)frame {
	self = [super initWithFrame:frame];
	if (self) {
		_hints = [NSMutableArray arrayWithCapacity:100];     // Pre-size for typical hint count
		_gridCells = [NSMutableArray arrayWithCapacity:100]; // Pre-size for typical grid size
		_gridLines = [NSMutableArray arrayWithCapacity:50];  // Pre-size for typical line count
		_showScrollHighlight = NO;
		_showTargetDot = NO;
		_targetDotRadius = 4.0;
		_targetDotBorderWidth = 1.0;
		_targetDotBorderColor = [NSColor blackColor];
		_targetDotBackgroundColor = [[NSColor colorWithRed:1.0 green:0.84 blue:0.0
		                                             alpha:1.0] colorWithAlphaComponent:0.95];
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
		_gridBackgroundOpacity = 0.85;
		_gridTextOpacity = 1.0;
		_hideUnmatched = NO;

		// Initialize cached colors with opacity
		_cachedGridTextColorWithOpacity = [_gridTextColor colorWithAlphaComponent:_gridTextOpacity];
		_cachedGridMatchedTextColorWithOpacity = [_gridMatchedTextColor colorWithAlphaComponent:_gridTextOpacity];

		// Initialize cached string buffer
		_cachedAttributedString = [[NSMutableAttributedString alloc] initWithString:@""];
	}
	return self;
}

/// Draw rectangle
/// @param dirtyRect Dirty rectangle
- (void)drawRect:(NSRect)dirtyRect {
	[super drawRect:dirtyRect];

	// Clear background
	[[NSColor clearColor] setFill];
	NSRectFill(dirtyRect);

	// Draw grid lines first (behind everything)
	[self drawGridLines];

	// Draw grid cells
	[self drawGridCells];

	// Draw scroll highlight if enabled
	if (self.showScrollHighlight) {
		[self drawScrollHighlight];
	}

	// Draw target dot if enabled (draw before hints so hints appear on top)
	if (self.showTargetDot) {
		[self drawTargetDot];
	}

	// Draw hints
	[self drawHints];
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

	NSColor *backgroundColor = [self colorFromHex:backgroundHex defaultColor:defaultBg];
	CGFloat opacity = style.opacity;
	if (opacity < 0.0 || opacity > 1.0) {
		opacity = 0.95;
	}
	self.hintBackgroundColor = [backgroundColor colorWithAlphaComponent:opacity];
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

	return [NSColor colorWithRed:red green:green blue:blue alpha:alpha];
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

/// Draw hints
- (void)drawHints {
	for (NSDictionary *hint in self.hints) {
		NSString *label = hint[@"label"];
		if (!label || [label length] == 0)
			continue;

		NSPoint position = [hint[@"position"] pointValue];
		NSNumber *matchedPrefixLengthNum = hint[@"matchedPrefixLength"];
		int matchedPrefixLength = matchedPrefixLengthNum ? [matchedPrefixLengthNum intValue] : 0;
		NSNumber *showArrowNum = hint[@"showArrow"];
		BOOL showArrow = showArrowNum ? [showArrowNum boolValue] : YES;

		// Create attributed string with matched prefix in different color
		// Reuse cached attributed string buffer
		NSMutableAttributedString *attrString = self.cachedAttributedString;
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
		CGFloat elementCenterX = position.x + (hint[@"size"] ? [hint[@"size"] sizeValue].width : 0) / 2.0;
		CGFloat elementCenterY = position.y + (hint[@"size"] ? [hint[@"size"] sizeValue].height : 0) / 2.0;

		// Position tooltip body above element (arrow points down)
		CGFloat gap = 3.0;
		CGFloat tooltipX = elementCenterX - boxWidth / 2.0;
		CGFloat tooltipY = elementCenterY + arrowHeight + gap;

		// Convert coordinates (macOS uses bottom-left origin, we need top-left)
		NSScreen *mainScreen = [NSScreen mainScreen];
		CGFloat screenHeight = [mainScreen frame].size.height;
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
		NSPoint textPosition = NSMakePoint(textX, textY);
		[attrString drawAtPoint:textPosition];
	}
}

/// Draw scroll highlight
- (void)drawScrollHighlight {
	if (CGRectIsEmpty(self.scrollHighlight)) {
		return;
	}

	NSGraphicsContext *context = [NSGraphicsContext currentContext];
	[context saveGraphicsState];

	// Convert coordinates (macOS uses bottom-left origin)
	NSScreen *mainScreen = [NSScreen mainScreen];
	CGFloat screenHeight = [mainScreen frame].size.height;
	CGFloat flippedY = screenHeight - self.scrollHighlight.origin.y - self.scrollHighlight.size.height;

	NSRect rect = NSMakeRect(self.scrollHighlight.origin.x, flippedY, self.scrollHighlight.size.width,
	                         self.scrollHighlight.size.height);

	NSBezierPath *path = [NSBezierPath bezierPathWithRect:rect];

	if (self.scrollHighlightColor) {
		[self.scrollHighlightColor setStroke];
	} else {
		[[NSColor redColor] setStroke];
	}
	[path setLineWidth:self.scrollHighlightWidth];
	[path stroke];

	[context restoreGraphicsState];
}

/// Draw target dot
- (void)drawTargetDot {
	NSGraphicsContext *context = [NSGraphicsContext currentContext];
	[context saveGraphicsState];

	// Convert coordinates (macOS uses bottom-left origin)
	NSScreen *mainScreen = [NSScreen mainScreen];
	CGFloat screenHeight = [mainScreen frame].size.height;
	CGFloat flippedY = screenHeight - self.targetDotCenter.y;

	CGFloat x = self.targetDotCenter.x - self.targetDotRadius;
	CGFloat y = flippedY - self.targetDotRadius;
	CGFloat diameter = self.targetDotRadius * 2;

	NSRect dotRect = NSMakeRect(x, y, diameter, diameter);
	NSBezierPath *circlePath = [NSBezierPath bezierPathWithOvalInRect:dotRect];

	// Fill the dot
	if (self.targetDotBackgroundColor) {
		[self.targetDotBackgroundColor setFill];
	} else {
		[[NSColor redColor] setFill];
	}
	[circlePath fill];

	if (self.targetDotBorderColor && self.targetDotBorderWidth > 0) {
		[self.targetDotBorderColor setStroke];
		[circlePath setLineWidth:self.targetDotBorderWidth];
		[circlePath stroke];
	}

	[context restoreGraphicsState];
}

/// Create color from hex string
/// @param hexString Hex color string
/// @return NSColor instance
- (NSColor *)colorFromHex:(NSString *)hexString {
	if (!hexString || [hexString length] == 0) {
		return [NSColor blackColor];
	}

	unsigned rgbValue = 0;
	NSScanner *scanner = [NSScanner scannerWithString:hexString];
	if ([hexString hasPrefix:@"#"]) {
		[scanner setScanLocation:1];
	}
	[scanner scanHexInt:&rgbValue];

	return [NSColor colorWithRed:((rgbValue & 0xFF0000) >> 16) / 255.0
	                       green:((rgbValue & 0xFF00) >> 8) / 255.0
	                        blue:(rgbValue & 0xFF) / 255.0
	                       alpha:1.0];
}

/// Draw grid cells
- (void)drawGridCells {
	if ([self.gridCells count] == 0)
		return;

	NSGraphicsContext *context = [NSGraphicsContext currentContext];
	[context saveGraphicsState];

	NSScreen *mainScreen = [NSScreen mainScreen];
	CGFloat screenHeight = [mainScreen frame].size.height;

	for (NSDictionary *cellDict in self.gridCells) {
		NSString *label = cellDict[@"label"];
		NSValue *boundsValue = cellDict[@"bounds"];
		BOOL isMatched = [cellDict[@"isMatched"] boolValue];
		BOOL isSubgrid = [cellDict[@"isSubgrid"] boolValue];

		// Skip drawing unmatched cells if hideUnmatched is enabled AND it's not a subgrid cell
		if (self.hideUnmatched && !isMatched && !isSubgrid) {
			continue;
		}

		CGRect bounds = [boundsValue rectValue];

		// Convert coordinates (macOS uses bottom-left origin)
		CGFloat flippedY = screenHeight - bounds.origin.y - bounds.size.height;
		NSRect cellRect = NSMakeRect(bounds.origin.x, flippedY, bounds.size.width, bounds.size.height);

		// Draw cell background with opacity
		NSColor *bgBase = self.gridBackgroundColor;
		if (isMatched && self.gridMatchedBackgroundColor) {
			bgBase = self.gridMatchedBackgroundColor;
		}
		NSColor *bgColor = [bgBase colorWithAlphaComponent:self.gridBackgroundOpacity];
		[bgColor setFill];
		NSRectFill(cellRect);

		// Draw cell border
		NSColor *borderColor = self.gridBorderColor;
		if (isMatched && self.gridMatchedBorderColor) {
			borderColor = self.gridMatchedBorderColor;
		}
		[borderColor setStroke];
		NSBezierPath *borderPath = [NSBezierPath bezierPathWithRect:cellRect];
		[borderPath setLineWidth:self.gridBorderWidth];
		[borderPath stroke];

		// Draw text label centered in cell
		if (label && [label length] > 0) {
			// Reuse cached attributed string buffer
			NSMutableAttributedString *attrString = self.cachedAttributedString;
			[[attrString mutableString] setString:label];

			// Clear previous attributes and set new ones
			NSRange fullRange = NSMakeRange(0, [label length]);
			[attrString setAttributes:@{NSFontAttributeName : self.gridFont} range:fullRange];

			// Use cached color with opacity to avoid repeated allocations
			[attrString addAttribute:NSForegroundColorAttributeName
			                   value:self.cachedGridTextColorWithOpacity
			                   range:fullRange];

			NSNumber *matchedPrefixLengthNum = cellDict[@"matchedPrefixLength"];
			int matchedPrefixLength = matchedPrefixLengthNum ? [matchedPrefixLengthNum intValue] : 0;
			if (isMatched && matchedPrefixLength > 0 && matchedPrefixLength <= [label length]) {
				// Use cached matched color with opacity
				[attrString addAttribute:NSForegroundColorAttributeName
				                   value:self.cachedGridMatchedTextColorWithOpacity
				                   range:NSMakeRange(0, matchedPrefixLength)];
			}

			NSSize textSize = [attrString size];
			CGFloat textX = cellRect.origin.x + (cellRect.size.width - textSize.width) / 2.0;
			CGFloat textY = cellRect.origin.y + (cellRect.size.height - textSize.height) / 2.0;

			[attrString drawAtPoint:NSMakePoint(textX, textY)];
		}
	}

	[context restoreGraphicsState];
}

/// Draw grid lines
- (void)drawGridLines {
	if ([self.gridLines count] == 0)
		return;

	NSGraphicsContext *context = [NSGraphicsContext currentContext];
	[context saveGraphicsState];

	for (NSDictionary *lineDict in self.gridLines) {
		NSValue *rectValue = lineDict[@"rect"];
		NSString *colorHex = lineDict[@"color"];
		NSNumber *widthNum = lineDict[@"width"];
		NSNumber *opacityNum = lineDict[@"opacity"];

		CGRect lineRect = [rectValue rectValue];
		int width = [widthNum intValue];
		double opacity = [opacityNum doubleValue];

		NSScreen *mainScreen = [NSScreen mainScreen];
		CGFloat screenHeight = [mainScreen frame].size.height;
		CGFloat flippedY = screenHeight - lineRect.origin.y - lineRect.size.height;
		NSRect rect = NSMakeRect(lineRect.origin.x, flippedY, lineRect.size.width, lineRect.size.height);

		NSColor *color = [self colorFromHex:colorHex];
		color = [color colorWithAlphaComponent:opacity];
		[color setFill];
		NSRectFill(rect);
	}

	[context restoreGraphicsState];
}

@end

#pragma mark - Overlay Window Controller Interface

@interface OverlayWindowController : NSObject
@property(nonatomic, strong) NSWindow *window;         ///< Window instance
@property(nonatomic, strong) OverlayView *overlayView; ///< Overlay view instance
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

	self.window = [[NSWindow alloc] initWithContentRect:screenFrame
	                                          styleMask:NSWindowStyleMaskBorderless
	                                            backing:NSBackingStoreBuffered
	                                              defer:NO];

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
		[controller retain];
	} else {
		dispatch_sync(dispatch_get_main_queue(), ^{
			controller = [[OverlayWindowController alloc] init];
			[controller retain];
		});
	}
	return (void *)controller;
}

/// Destroy overlay window
/// @param window Overlay window handle
void NeruDestroyOverlayWindow(OverlayWindow window) {
	if (!window) {
		return;
	}

	OverlayWindowController *controller = (OverlayWindowController *)window;
	if ([NSThread isMainThread]) {
		[controller.window close];
		[controller release];
	} else {
		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.window close];
			[controller release];
		});
	}
}

/// Show overlay window
/// @param window Overlay window handle
void NeruShowOverlayWindow(OverlayWindow window) {
	if (!window) {
		return;
	}

	OverlayWindowController *controller = (OverlayWindowController *)window;

	dispatch_async(dispatch_get_main_queue(), ^{
		[controller.window setLevel:kCGMaximumWindowLevel];

		[controller.window setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces |
		                                         NSWindowCollectionBehaviorStationary |
		                                         NSWindowCollectionBehaviorIgnoresCycle |
		                                         NSWindowCollectionBehaviorFullScreenAuxiliary];

		[controller.window setIsVisible:YES];
		[controller.window orderFrontRegardless];
		[controller.window makeKeyAndOrderFront:nil];

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

	OverlayWindowController *controller = (OverlayWindowController *)window;

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

	OverlayWindowController *controller = (OverlayWindowController *)window;

	if ([NSThread isMainThread]) {
		[controller.overlayView.hints removeAllObjects];
		[controller.overlayView.gridCells removeAllObjects];
		[controller.overlayView.gridLines removeAllObjects];
		controller.overlayView.showScrollHighlight = NO;
		controller.overlayView.showTargetDot = NO;
		[controller.overlayView setNeedsDisplay:YES];
	} else {
		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.overlayView.hints removeAllObjects];
			[controller.overlayView.gridCells removeAllObjects];
			[controller.overlayView.gridLines removeAllObjects];
			controller.overlayView.showScrollHighlight = NO;
			controller.overlayView.showTargetDot = NO;
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

	OverlayWindowController *controller = (OverlayWindowController *)window;
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
	});
}

/// Resize overlay to active screen
/// @param window Overlay window handle
void NeruResizeOverlayToActiveScreen(OverlayWindow window) {
	if (!window) {
		return;
	}

	OverlayWindowController *controller = (OverlayWindowController *)window;
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

	OverlayWindowController *controller = (OverlayWindowController *)window;
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

		if (callback) {
			callback(context);
		}
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

	OverlayWindowController *controller = (OverlayWindowController *)window;

	if ([NSThread isMainThread]) {
		[controller.overlayView.hints removeAllObjects];
		[controller.overlayView applyStyle:style];

		for (int i = 0; i < count; i++) {
			HintData hint = hints[i];
			NSDictionary *hintDict = @{
				@"label" : @(hint.label),
				@"position" : [NSValue valueWithPoint:NSPointFromCGPoint(hint.position)],
				@"matchedPrefixLength" : @(hint.matchedPrefixLength),
				@"showArrow" : @(style.showArrow)
			};
			[controller.overlayView.hints addObject:hintDict];
		}

		[controller.overlayView setNeedsDisplay:YES];
	} else {
		// Copy hint data
		NSMutableArray *hintDicts = [NSMutableArray arrayWithCapacity:count];
		for (int i = 0; i < count; i++) {
			HintData hint = hints[i];
			NSDictionary *hintDict = @{
				@"label" : @(hint.label),
				@"position" : [NSValue valueWithPoint:NSPointFromCGPoint(hint.position)],
				@"matchedPrefixLength" : @(hint.matchedPrefixLength),
				@"showArrow" : @(style.showArrow)
			};
			[hintDicts addObject:hintDict];
		}

		HintStyle styleCopy = {.fontSize = style.fontSize,
		                       .borderRadius = style.borderRadius,
		                       .borderWidth = style.borderWidth,
		                       .padding = style.padding,
		                       .opacity = style.opacity,
		                       .showArrow = style.showArrow,
		                       .fontFamily = safe_strdup(style.fontFamily),
		                       .backgroundColor = safe_strdup(style.backgroundColor),
		                       .textColor = safe_strdup(style.textColor),
		                       .matchedTextColor = safe_strdup(style.matchedTextColor),
		                       .borderColor = safe_strdup(style.borderColor)};

		dispatch_async(dispatch_get_main_queue(), ^{
			[controller.overlayView.hints removeAllObjects];
			[controller.overlayView applyStyle:styleCopy];
			[controller.overlayView.hints addObjectsFromArray:hintDicts];
			[controller.overlayView setNeedsDisplay:YES];

			free_hint_style_strings(&styleCopy);
		});
	}
}

/// Draw scroll highlight
/// @param window Overlay window handle
/// @param bounds Highlight bounds
/// @param color Highlight color
/// @param width Highlight width
void NeruDrawScrollHighlight(OverlayWindow window, CGRect bounds, char *color, int width) {
	if (!window)
		return;

	OverlayWindowController *controller = (OverlayWindowController *)window;

	if ([NSThread isMainThread]) {
		controller.overlayView.scrollHighlight = bounds;
		controller.overlayView.scrollHighlightWidth = width;
		controller.overlayView.showScrollHighlight = YES;

		if (color) {
			NSString *colorStr = @(color);
			unsigned rgbValue = 0;
			NSScanner *scanner = [NSScanner scannerWithString:colorStr];
			[scanner setScanLocation:1];
			[scanner scanHexInt:&rgbValue];

			controller.overlayView.scrollHighlightColor = [NSColor colorWithRed:((rgbValue & 0xFF0000) >> 16) / 255.0
			                                                              green:((rgbValue & 0xFF00) >> 8) / 255.0
			                                                               blue:(rgbValue & 0xFF) / 255.0
			                                                              alpha:1.0];
		}

		[controller.overlayView setNeedsDisplay:YES];
	} else {
		NSString *colorStr = color ? [NSString stringWithUTF8String:color] : nil;

		dispatch_async(dispatch_get_main_queue(), ^{
			controller.overlayView.scrollHighlight = bounds;
			controller.overlayView.scrollHighlightWidth = width;
			controller.overlayView.showScrollHighlight = YES;

			if (colorStr) {
				unsigned rgbValue = 0;
				NSScanner *scanner = [NSScanner scannerWithString:colorStr];
				[scanner setScanLocation:1];
				[scanner scanHexInt:&rgbValue];

				controller.overlayView.scrollHighlightColor =
				    [NSColor colorWithRed:((rgbValue & 0xFF0000) >> 16) / 255.0
				                    green:((rgbValue & 0xFF00) >> 8) / 255.0
				                     blue:(rgbValue & 0xFF) / 255.0
				                    alpha:1.0];
			}

			[controller.overlayView setNeedsDisplay:YES];
		});
	}
}

/// Draw target dot
/// @param window Overlay window handle
/// @param center Dot center
/// @param radius Dot radius
/// @param colorStr Dot color
/// @param borderColorStr Border color
/// @param borderWidth Border width
void NeruDrawTargetDot(OverlayWindow window, CGPoint center, double radius, const char *colorStr,
                       const char *borderColorStr, double borderWidth) {
	if (!window)
		return;

	OverlayWindowController *controller = (OverlayWindowController *)window;

	if ([NSThread isMainThread]) {
		controller.overlayView.targetDotCenter = center;
		controller.overlayView.targetDotRadius = radius;
		controller.overlayView.targetDotBorderWidth = borderWidth;
		controller.overlayView.showTargetDot = YES;

		if (colorStr) {
			NSString *colorString = @(colorStr);
			controller.overlayView.targetDotBackgroundColor = [controller.overlayView colorFromHex:colorString
			                                                                          defaultColor:[NSColor redColor]];
		} else {
			controller.overlayView.targetDotBackgroundColor = [NSColor redColor];
		}

		if (borderColorStr) {
			NSString *borderColorString = @(borderColorStr);
			controller.overlayView.targetDotBorderColor = [controller.overlayView colorFromHex:borderColorString
			                                                                      defaultColor:[NSColor blackColor]];
		} else {
			controller.overlayView.targetDotBorderColor = nil;
		}

		[controller.overlayView setNeedsDisplay:YES];
	} else {
		NSString *colorString = colorStr ? [NSString stringWithUTF8String:colorStr] : nil;
		NSString *borderColorString = borderColorStr ? [NSString stringWithUTF8String:borderColorStr] : nil;

		dispatch_async(dispatch_get_main_queue(), ^{
			controller.overlayView.targetDotCenter = center;
			controller.overlayView.targetDotRadius = radius;
			controller.overlayView.targetDotBorderWidth = borderWidth;
			controller.overlayView.showTargetDot = YES;

			if (colorString) {
				controller.overlayView.targetDotBackgroundColor =
				    [controller.overlayView colorFromHex:colorString defaultColor:[NSColor redColor]];
			} else {
				controller.overlayView.targetDotBackgroundColor = [NSColor redColor];
			}

			if (borderColorString) {
				controller.overlayView.targetDotBorderColor =
				    [controller.overlayView colorFromHex:borderColorString defaultColor:[NSColor blackColor]];
			} else {
				controller.overlayView.targetDotBorderColor = nil;
			}

			[controller.overlayView setNeedsDisplay:YES];
		});
	}
}

/// Replace overlay window
/// @param pwindow Pointer to overlay window handle
void NeruReplaceOverlayWindow(OverlayWindow *pwindow) {
	if (!pwindow)
		return;
	dispatch_async(dispatch_get_main_queue(), ^{
		OverlayWindowController *oldController = (OverlayWindowController *)(*pwindow);
		OverlayWindowController *newController = [[OverlayWindowController alloc] init];
		[newController retain];
		if (oldController) {
			[oldController.window close];
			[oldController release];
		}
		*pwindow = (void *)newController;
	});
}

/// Draw grid cells
/// @param window Overlay window handle
/// @param cells Array of grid cells
/// @param count Number of cells
/// @param style Grid cell style
void NeruDrawGridCells(OverlayWindow window, GridCell *cells, int count, GridCellStyle style) {
	if (!window || !cells)
		return;

	OverlayWindowController *controller = (OverlayWindowController *)window;

	// Build cell data array and copy all strings NOW
	NSMutableArray *cellDicts = [NSMutableArray arrayWithCapacity:count];
	for (int i = 0; i < count; i++) {
		GridCell cell = cells[i];
		NSDictionary *cellDict = @{
			@"label" : cell.label ? @(cell.label) : @"",
			@"bounds" : [NSValue valueWithRect:NSRectFromCGRect(cell.bounds)],
			@"isMatched" : @(cell.isMatched),
			@"isSubgrid" : @(cell.isSubgrid),
			@"matchedPrefixLength" : @(cell.matchedPrefixLength)
		};
		[cellDicts addObject:cellDict];
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
	double backgroundOpacity = style.backgroundOpacity;
	double textOpacity = style.textOpacity;

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
		controller.overlayView.gridBackgroundOpacity =
		    (backgroundOpacity >= 0.0 && backgroundOpacity <= 1.0) ? backgroundOpacity : 0.85;
		controller.overlayView.gridTextOpacity = (textOpacity >= 0.0 && textOpacity <= 1.0) ? textOpacity : 1.0;

		// Update cached colors after setting style properties
		controller.overlayView.cachedGridTextColorWithOpacity =
		    [controller.overlayView.gridTextColor colorWithAlphaComponent:controller.overlayView.gridTextOpacity];
		controller.overlayView.cachedGridMatchedTextColorWithOpacity = [controller.overlayView.gridMatchedTextColor
		    colorWithAlphaComponent:controller.overlayView.gridTextOpacity];

		[controller.overlayView.gridCells removeAllObjects];
		[controller.overlayView.gridCells addObjectsFromArray:cellDicts];
		[controller.overlayView setNeedsDisplay:YES];
	});
}

/// Draw window border lines
/// @param window Overlay window handle
/// @param lines Array of line rectangles
/// @param count Number of lines
/// @param color Line color
/// @param width Line width
/// @param opacity Line opacity
void NeruDrawWindowBorder(OverlayWindow window, CGRect *lines, int count, char *color, int width, double opacity) {
	if (!window || !lines)
		return;

	OverlayWindowController *controller = (OverlayWindowController *)window;
	NSString *colorHex = color ? @(color) : @"#333333";

	// Build line data array
	NSMutableArray *lineDicts = [NSMutableArray arrayWithCapacity:count];
	for (int i = 0; i < count; i++) {
		NSDictionary *lineDict = @{
			@"rect" : [NSValue valueWithRect:NSRectFromCGRect(lines[i])],
			@"color" : colorHex,
			@"width" : @(width),
			@"opacity" : @(opacity)
		};
		[lineDicts addObject:lineDict];
	}

	dispatch_async(dispatch_get_main_queue(), ^{
		[controller.overlayView.gridLines removeAllObjects];
		[controller.overlayView.gridLines addObjectsFromArray:lineDicts];
		[controller.overlayView setNeedsDisplay:YES];
	});
}

/// Update grid match prefix
/// @param window Overlay window handle
/// @param prefix Match prefix
void NeruUpdateGridMatchPrefix(OverlayWindow window, const char *prefix) {
	if (!window)
		return;

	OverlayWindowController *controller = (OverlayWindowController *)window;

	NSString *prefixStr = prefix ? @(prefix) : @"";

	dispatch_async(dispatch_get_main_queue(), ^{
		NSMutableArray *updated = [NSMutableArray arrayWithCapacity:[controller.overlayView.gridCells count]];
		for (NSDictionary *cellDict in controller.overlayView.gridCells) {
			NSString *label = cellDict[@"label"] ?: @"";
			BOOL isMatched = NO;
			int matchedPrefixLength = 0;
			if ([prefixStr length] > 0 && [label length] >= [prefixStr length]) {
				NSString *lblPrefix = [label substringToIndex:[prefixStr length]];
				isMatched = [lblPrefix isEqualToString:prefixStr];
				if (isMatched) {
					matchedPrefixLength = (int)[prefixStr length];
				}
			}
			BOOL isSubgrid = [cellDict[@"isSubgrid"] boolValue];
			NSDictionary *newDict = @{
				@"label" : label,
				@"bounds" : cellDict[@"bounds"],
				@"isMatched" : @(isMatched),
				@"isSubgrid" : @(isSubgrid),
				@"matchedPrefixLength" : @(matchedPrefixLength)
			};
			[updated addObject:newDict];
		}
		[controller.overlayView.gridCells removeAllObjects];
		[controller.overlayView.gridCells addObjectsFromArray:updated];
		[controller.overlayView setNeedsDisplay:YES];
	});
}

/// Set overlay level
/// @param window Overlay window handle
/// @param level Overlay level
void NeruSetOverlayLevel(OverlayWindow window, int level) {
	if (!window)
		return;

	OverlayWindowController *controller = (OverlayWindowController *)window;

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

	OverlayWindowController *controller = (OverlayWindowController *)window;

	dispatch_async(dispatch_get_main_queue(), ^{
		controller.overlayView.hideUnmatched = hide ? YES : NO;
		[controller.overlayView setNeedsDisplay:YES];
	});
}
