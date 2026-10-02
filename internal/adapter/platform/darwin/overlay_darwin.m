//
//  overlay.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "overlay.h"

#import <Cocoa/Cocoa.h>
#import <CoreText/CoreText.h>
#import <QuartzCore/QuartzCore.h>
#import <stdatomic.h>

#pragma mark - HintItem Class

@interface HintItem : NSObject
@property(nonatomic, copy) NSString *label;
@property(nonatomic, assign) CGPoint position;
@property(nonatomic, assign) CGSize size;
@property(nonatomic, assign) int matchedPrefixLength;
@property(nonatomic, assign) BOOL showArrow;
@property(nonatomic, assign) int placement;
@end

@implementation HintItem

- (instancetype)init {
	self = [super init];
	if (self) {
		_showArrow = YES;
		_placement = 3;  // Bottom, default placement, overwritten by buildHintItems
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

@interface SearchInputItem : NSObject
@property(nonatomic, copy) NSString *query;
@property(nonatomic, assign) NSInteger resultCount;
@property(nonatomic, assign) CGPoint position;
@property(nonatomic, assign) CGFloat width;
@end

@implementation SearchInputItem
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

/// Where a hint badge sits relative to its element. These values must equal the
/// HINT_PLACEMENT_* macros in overlay.h, which is what Go passes in; the pin is
/// internal/architecture/hint_placement_vocabulary_test.go.
typedef NS_ENUM(NSInteger, HintPlacement) {
	HintPlacementTop = 1,
	HintPlacementCenter = 2,
	HintPlacementBottom = 3,
};

#pragma mark - Layer Tree Classes

@class OverlayView;

/// What an item layer draws. OverlayView draws every kind in view
/// coordinates, and the layer only sets which pixels it covers.
typedef NS_ENUM(NSInteger, NeruItemKind) {
	NeruItemKindHint,
	NeruItemKindText,
	NeruItemKindSearchInput,
};

/// A layer sized to one drawn item. Its backing store covers that item alone,
/// so overlay memory grows with what is on screen, not with the screen's size.
@interface NeruItemLayer : CALayer
@property(nonatomic, assign) NeruItemKind kind;
@property(nonatomic, strong) id item;                     ///< HintItem or SearchInputItem drawn
@property(nonatomic, copy) NSString *text;                ///< Plain text drawn
@property(nonatomic, strong) NSFont *font;                ///< Font of text
@property(nonatomic, strong) NSColor *color;              ///< Color of plain text
@property(nonatomic, assign) NSSize textSize;             ///< text measured in font
@property(nonatomic, assign) int matchedPrefixLength;     ///< Characters drawn as matched
@property(nonatomic, assign) NSUInteger styleGeneration;  ///< Style the contents were drawn with
@property(nonatomic, assign) CGRect itemRect;             ///< What is drawn (view coordinates)
@property(nonatomic, assign) CGPoint drawOrigin;          ///< View point at the layer's origin
@property(nonatomic, assign) CGRect drawnRect;            ///< itemRect relative to the layer when last drawn
@property(nonatomic, strong) CALayer *boundary;           ///< Hint target highlight, below the badge
@end

@implementation NeruItemLayer
@end

/// One character in one font and color at one scale, drawn once and shared
/// by every label that shows it.
@interface NeruGlyph : NSObject
@property(nonatomic, strong) id image;     ///< CGImageRef
@property(nonatomic, assign) CGSize size;  ///< Image size in points, padding included
@end

@implementation NeruGlyph
@end

/// How a text lays out in one font: its characters and where each starts.
@interface NeruTextLayout : NSObject
@property(nonatomic, assign) NSSize size;
@property(nonatomic, strong) NSArray<NSString *> *characters;  ///< The text in pieces the font draws apart
@property(nonatomic, strong) NSArray<NSNumber *> *locations;   ///< Each piece's index in the text
@property(nonatomic, strong) NSArray<NSNumber *> *offsets;     ///< Each piece's x from the line start
@end

@implementation NeruTextLayout
@end

/// The glyph images of one font and color at one scale, and the layouts of
/// texts in that font. Building a label from a text seen before costs only
/// lookups here.
@interface NeruGlyphSet : NSObject
@property(nonatomic, strong) NSFont *font;
@property(nonatomic, strong) NSColor *color;
@property(nonatomic, assign) CGFloat scale;
@property(nonatomic, strong) NSMutableDictionary<NSString *, NeruGlyph *> *glyphs;
@property(nonatomic, strong) NSCache<NSString *, NeruTextLayout *> *layouts;
@end

@implementation NeruGlyphSet
@end

/// A line of text built from shared glyph images, one sublayer per character.
/// A dense grid shows thousands of labels. A bitmap for each would cost as much
/// as one the size of the screen.
@interface NeruLabelLayer : CALayer
@property(nonatomic, strong) NeruTextLayout *layout;
@property(nonatomic, strong) NeruGlyphSet *glyphSet;
@property(nonatomic, strong) NeruGlyphSet *matchedGlyphSet;
@property(nonatomic, assign) int matchedPrefixLength;
@property(nonatomic, strong) NSMutableArray<CALayer *> *glyphLayers;
@property(nonatomic, strong) CALayer *badge;  ///< Rounded background behind the text, when configured
@end

@implementation NeruLabelLayer
@end

/// One grid cell. The layer's own background is the cell's, and its border and
/// labels are sublayers, so cells stack in the order they always drew in.
@interface NeruCellLayer : CALayer
@property(nonatomic, strong) CALayer *border;
@property(nonatomic, strong) NeruLabelLayer *label;
@property(nonatomic, strong) NeruLabelLayer *fadeLabel;  ///< Outgoing label while a transition cross-fades
@property(nonatomic, strong) NSMutableArray<NeruLabelLayer *> *subKeys;
// What the cell was last placed with. Rendering skips a cell when none of it changed.
@property(nonatomic, assign) NSUInteger placedGeneration;
@property(nonatomic, assign) CGRect placedRect;
@property(nonatomic, assign) BOOL placedMatched;
@property(nonatomic, assign) int placedMatchedPrefixLength;
@property(nonatomic, copy) NSString *placedLabel;
@end

@implementation NeruCellLayer
@end

/// Draws item layers and keeps every overlay layer from animating implicitly.
@interface NeruLayerDrawer : NSObject <CALayerDelegate>
@property(nonatomic, weak) OverlayView *view;
@end

#pragma mark - Overlay View Interface

/// The overlay is a tree of small layers, one per drawn item, rather than one
/// screen-sized bitmap. A screen-sized bitmap costs 32MB at 4K for as long as
/// it lives, and the system keeps it after the overlay hides.
@interface OverlayView : NSView
@property(nonatomic, strong) NSMutableArray<HintItem *> *hints;     ///< Hints array
@property(nonatomic, strong) NSFont *hintFont;                      ///< Hint font
@property(nonatomic, strong) NSColor *hintTextColor;                ///< Hint text color
@property(nonatomic, strong) NSColor *hintMatchedTextColor;         ///< Hint matched text color
@property(nonatomic, strong) NSColor *hintBackgroundColor;          ///< Hint background color
@property(nonatomic, strong) NSColor *hintBorderColor;              ///< Hint border color
@property(nonatomic, strong) NSColor *hintBoundaryBackgroundColor;  ///< Target boundary fill color
@property(nonatomic, strong) NSColor *hintBoundaryBorderColor;      ///< Target boundary stroke color
@property(nonatomic, assign) CGFloat hintBorderRadius;              ///< Hint border radius
@property(nonatomic, assign) CGFloat hintBorderWidth;               ///< Hint border width
@property(nonatomic, assign) CGFloat hintPaddingX;                  ///< Hint horizontal padding
@property(nonatomic, assign) CGFloat hintPaddingY;                  ///< Hint vertical padding
@property(nonatomic, assign) BOOL hintBoundaryHighlightEnabled;     ///< Draw target boundary highlight
@property(nonatomic, assign) CGFloat hintBoundaryBorderWidth;       ///< Target boundary stroke width
@property(nonatomic, assign) CGFloat hintBoundaryBorderRadius;      ///< Target boundary corner radius
@property(nonatomic, strong) SearchInputItem *searchInput;          ///< Active hints search input
@property(nonatomic, strong) NSFont *searchInputFont;               ///< Search input font
@property(nonatomic, strong) NSColor *searchInputTextColor;         ///< Search input text color
@property(nonatomic, strong) NSColor *searchInputBackgroundColor;   ///< Search input background color
@property(nonatomic, strong) NSColor *searchInputBorderColor;       ///< Search input border color
@property(nonatomic, assign) CGFloat searchInputBorderRadius;       ///< Search input border radius
@property(nonatomic, assign) CGFloat searchInputBorderWidth;        ///< Search input border width
@property(nonatomic, assign) CGFloat searchInputPaddingX;           ///< Search input horizontal padding
@property(nonatomic, assign) CGFloat searchInputPaddingY;           ///< Search input vertical padding

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
@property(nonatomic, assign) BOOL gridHideLabel;                ///< Cells are too small for a label at any allowed size
@property(nonatomic, strong) NSFont *gridTransitionFont;        ///< Label font held while a transition runs
@property(nonatomic, assign) BOOL gridTransitionHideLabel;      ///< Hide labels while a transition runs
@property(nonatomic, strong) NSFont *gridTransitionSubKeyFont;  ///< Preview font held while a transition runs
@property(nonatomic, assign) BOOL gridTransitionHideSubKeyPreview;  ///< Hide the preview while a transition runs
@property(nonatomic, strong)
    NSMutableDictionary<NSString *, NSFont *> *gridTransitionFontCache;  ///< Transition fonts by family and size

// Sub-key preview: draws a miniature key grid inside each cell
@property(nonatomic, assign) BOOL gridDrawSubKeyPreview;             ///< Draw sub-key preview mini-grid
@property(nonatomic, assign) int gridSubKeyCols;                     ///< Sub-key preview grid columns
@property(nonatomic, assign) int gridSubKeyRows;                     ///< Sub-key preview grid rows
@property(nonatomic, strong) NSFont *gridSubKeyFont;                 ///< Sub-key preview font
@property(nonatomic, strong) NSColor *gridSubKeyTextColor;           ///< Sub-key preview text color
@property(nonatomic, assign) CGFloat cachedGridSubKeyFontSize;       ///< Cached sub-key font size
@property(nonatomic, copy) NSString *cachedGridSubKeyFontFamily;     ///< Cached sub-key font family
@property(nonatomic, strong) NSArray<NSString *> *gridSubKeyLabels;  ///< Labels for sub-key preview (next depth's keys)
@property(nonatomic, assign) BOOL cursorIndicatorVisible;            ///< Draw virtual cursor indicator
@property(nonatomic, assign) NSPoint cursorIndicatorPosition;        ///< Virtual cursor indicator center
@property(nonatomic, strong) NSColor *cursorIndicatorFillColor;      ///< Virtual cursor indicator fill
@property(nonatomic, copy) NSString *cursorIndicatorLabel;           ///< Virtual cursor indicator label (char)
@property(nonatomic, strong) NSFont *cursorIndicatorFont;            ///< Virtual cursor indicator font
@property(nonatomic, strong) NSColor *cursorIndicatorTextColor;      ///< Virtual cursor indicator text color
@property(nonatomic, assign)
    BOOL cursorIndicatorTransitionActive;  ///< Animate virtual pointer with recursive-grid transitions
@property(nonatomic, assign) NSPoint cursorIndicatorFromPosition;  ///< Previous virtual pointer position
@property(nonatomic, assign) NSPoint cursorIndicatorToPosition;    ///< Target virtual pointer position

// Cached grid text colors to reduce allocations during drawing
@property(nonatomic, strong) NSColor *cachedGridTextColor;
@property(nonatomic, strong) NSColor *cachedGridMatchedTextColor;

// Cached string buffers to reduce allocations during drawing.
// Each buffer is exclusively used by its corresponding method to avoid shared mutable state.
@property(nonatomic, strong) NSMutableAttributedString *cachedHintAttributedString;  ///< Buffer for drawHint:
@property(nonatomic, strong) NSMutableAttributedString *cachedHintMeasureString;     ///< Buffer for badgeRectForHint:
@property(nonatomic, strong)
    NSMutableAttributedString *cachedSearchInputAttributedString;  ///< Cached string buffer for search input drawing

// Cached font keys: only re-create NSFont when family or size actually changes.
@property(nonatomic, copy) NSString *cachedHintFontFamily;  ///< Last resolved hint font family
@property(nonatomic, assign) CGFloat cachedHintFontSize;    ///< Last resolved hint font size
@property(nonatomic, copy) NSString *cachedGridFontFamily;  ///< Last resolved grid font family
@property(nonatomic, assign) CGFloat cachedGridFontSize;    ///< Last resolved grid font size

/// Cached parsed colors keyed by normalized hex string
@property(nonatomic, strong) NSCache *colorCache;

// Layer tree, back to front: grid, hints, then the search input and cursor.
@property(nonatomic, strong) NeruLayerDrawer *layerDrawer;
@property(nonatomic, strong) CALayer *gridRoot;
@property(nonatomic, strong) CALayer *hintRoot;
@property(nonatomic, strong) CALayer *topRoot;
@property(nonatomic, strong) NSMutableArray<NeruCellLayer *> *cellLayers;
@property(nonatomic, strong) NSMutableArray<NeruItemLayer *> *hintLayers;
@property(nonatomic, strong) NeruItemLayer *searchInputLayer;
@property(nonatomic, strong) NeruItemLayer *cursorIndicatorLayer;
@property(nonatomic, strong) NSMutableDictionary<NSString *, NeruGlyphSet *> *glyphSets;  ///< By font, color and scale

// Each signature records the style item contents were drawn with. A change
// bumps the generation, and every item drawn with the old style redraws.
@property(nonatomic, copy) NSArray *hintStyleSignature;
@property(nonatomic, assign) NSUInteger hintStyleGeneration;
@property(nonatomic, copy) NSArray *gridStyleSignature;
@property(nonatomic, assign) NSUInteger gridStyleGeneration;
@property(nonatomic, copy) NSArray *searchInputStyleSignature;
@property(nonatomic, assign) NSUInteger searchInputStyleGeneration;

- (void)clearContent;

- (void)applyStyle:(HintStyle)style;                                                   ///< Apply hint style
- (NSColor *)colorFromHex:(NSString *)hexString defaultColor:(NSColor *)defaultColor;  ///< Color from hex string
- (CGFloat)currentBackingScaleFactor;                                                  ///< Current backing scale factor
- (void)drawItemLayer:(NeruItemLayer *)layer inContext:(CGContextRef)ctx;              ///< Draw one item layer
- (void)cancelGridTransition;             ///< Stop recursive-grid animation
- (void)cancelCursorIndicatorTransition;  ///< Stop virtual pointer animation
- (void)startGridTransitionToCells:(NSArray<GridCellItem *> *)cells
                          duration:(CFTimeInterval)duration;  ///< Animate recursive-grid between states
- (NSArray<GridCellItem *> *)interpolatedGridCellsForProgress:(CGFloat)progress;  ///< Snapshot animated cells
- (CGFloat)currentGridTransitionProgress;   ///< Shared progress for grid/pointer animation
- (NSPoint)currentCursorIndicatorPosition;  ///< Current virtual pointer position for drawing

/// Resolve a font by name (accepts both PostScript names and family names).
/// Tries [NSFont fontWithName:] first, then NSFontManager family lookup.
/// Returns nil if the name cannot be resolved.
- (NSFont *)resolveFont:(NSString *)name size:(CGFloat)size bold:(BOOL)bold;
- (NSFont *)transitionFontForFamily:(NSString *)family size:(CGFloat)size;

/// Resolve horizontal hint padding (-1 = auto based on font size).
- (CGFloat)resolvedHintPaddingX;

/// Resolve vertical hint padding (-1 = auto based on font size).
- (CGFloat)resolvedHintPaddingY;
@end

@implementation NeruLayerDrawer

- (id<CAAction>)actionForLayer:(CALayer *)layer forKey:(NSString *)event {
	return (id<CAAction>)[NSNull null];
}

- (void)drawLayer:(CALayer *)layer inContext:(CGContextRef)ctx {
	if (![layer isKindOfClass:[NeruItemLayer class]])
		return;
	[self.view drawItemLayer:(NeruItemLayer *)layer inContext:ctx];
}

@end

#pragma mark - Overlay View Implementation

@implementation OverlayView

/// Initialize with frame
/// @param frame View frame
/// @return Initialized instance
- (instancetype)initWithFrame:(NSRect)frame {
	self = [super initWithFrame:frame];
	if (self) {
		// The view's own layer draws nothing. The items are its sublayers.
		[self setWantsLayer:YES];
		self.layer.opaque = NO;
		self.layer.backgroundColor = [[NSColor clearColor] CGColor];

		_layerDrawer = [[NeruLayerDrawer alloc] init];
		_layerDrawer.view = self;
		_gridRoot = [self makeContainerLayer];
		_hintRoot = [self makeContainerLayer];
		_topRoot = [self makeContainerLayer];
		_cellLayers = [NSMutableArray array];
		_hintLayers = [NSMutableArray array];
		_glyphSets = [NSMutableDictionary dictionary];

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
		_hintBoundaryBackgroundColor = [[NSColor systemBlueColor] colorWithAlphaComponent:0.08];
		_hintBoundaryBorderColor = [[NSColor systemBlueColor] colorWithAlphaComponent:0.45];
		_hintBorderRadius = -1.0;
		_hintBorderWidth = 1.0;
		_hintBoundaryHighlightEnabled = NO;
		_hintBoundaryBorderWidth = 1.0;
		_hintBoundaryBorderRadius = 4.0;
		_hintPaddingX = -1.0;
		_hintPaddingY = -1.0;
		_searchInput = nil;
		_searchInputFont = [NSFont systemFontOfSize:kDefaultHintFontSize];
		_searchInputTextColor = [NSColor blackColor];
		_searchInputBackgroundColor = [[NSColor colorWithRed:1.0 green:1.0 blue:1.0
		                                               alpha:1.0] colorWithAlphaComponent:0.95];
		_searchInputBorderColor = [NSColor blackColor];
		_searchInputBorderRadius = -1.0;
		_searchInputBorderWidth = 1.0;
		_searchInputPaddingX = -1.0;
		_searchInputPaddingY = -1.0;

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
		_gridHideLabel = NO;
		_gridTransitionHideLabel = NO;
		_gridTransitionHideSubKeyPreview = NO;
		_gridTransitionFontCache = [NSMutableDictionary dictionary];
		_hideUnmatched = NO;
		_cursorIndicatorVisible = NO;
		_cursorIndicatorFillColor = [NSColor colorWithWhite:1.0 alpha:1.0];
		_cursorIndicatorLabel = nil;
		_cursorIndicatorFont = nil;
		_cursorIndicatorTextColor = nil;
		_cursorIndicatorTransitionActive = NO;
		_cursorIndicatorFromPosition = NSZeroPoint;
		_cursorIndicatorToPosition = NSZeroPoint;

		// Initialize cached colors
		_cachedGridTextColor = _gridTextColor;
		_cachedGridMatchedTextColor = _gridMatchedTextColor;

		// Initialize cached string buffers
		_cachedHintAttributedString = [[NSMutableAttributedString alloc] initWithString:@""];
		_cachedHintMeasureString = [[NSMutableAttributedString alloc] initWithString:@""];
		_cachedSearchInputAttributedString = [[NSMutableAttributedString alloc] initWithString:@""];

		// Initialize cached font keys (match defaults above)
		_cachedHintFontFamily = nil;
		_cachedHintFontSize = kDefaultHintFontSize;
		_cachedGridFontFamily = nil;
		_cachedGridFontSize = kDefaultGridFontSize;

		_gridTransitionDuration = 0.18;
		_gridTransitionStartTime = 0;
		_gridTransitionActive = NO;
	}
	return self;
}

- (void)dealloc {
	[self.gridTransitionTimer invalidate];
	self.gridTransitionTimer = nil;
}

/// A sublayer of the view's layer that only groups items. It has no contents.
- (CALayer *)makeContainerLayer {
	CALayer *container = [CALayer layer];
	container.delegate = self.layerDrawer;
	[self.layer addSublayer:container];
	return container;
}

- (void)clearContent {
	BOOL hadContent =
	    [self.hints count] > 0 || [self.gridCells count] > 0 || self.searchInput || self.cursorIndicatorVisible;
	[self cancelGridTransition];
	[self cancelCursorIndicatorTransition];
	[self.hints removeAllObjects];
	[self.gridCells removeAllObjects];
	self.searchInput = nil;
	self.cursorIndicatorVisible = NO;
	self.cursorIndicatorLabel = nil;
	self.cursorIndicatorFont = nil;
	self.cursorIndicatorTextColor = nil;
	[self.colorCache removeAllObjects];
	[[self.cachedHintAttributedString mutableString] setString:@""];
	[[self.cachedHintMeasureString mutableString] setString:@""];
	[[self.cachedSearchInputAttributedString mutableString] setString:@""];
	// Render now. AppKit does not display a hidden window, so its item layers
	// would keep their bitmaps until the next show.
	if (hadContent)
		[self renderLayers];
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

/// Redraw item layers at the new scale when the view moves between screens
/// with different backing properties (e.g. Retina to non-Retina).
- (void)viewDidChangeBackingProperties {
	[super viewDidChangeBackingProperties];
	[self setNeedsDisplay:YES];
}

/// AppKit answers setNeedsDisplay: with updateLayer instead of drawRect:, so
/// the view's own layer never gets a backing store.
- (BOOL)wantsUpdateLayer {
	return YES;
}

- (void)updateLayer {
	[self renderLayers];
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

	__weak typeof(self) weakSelf = self;
	self.gridTransitionTimer = [NSTimer timerWithTimeInterval:(1.0 / 120.0)
	                                                  repeats:YES
	                                                    block:^(__unused NSTimer *timer) {
		                                                    OverlayView *strongSelf = weakSelf;
		                                                    if (!strongSelf)
			                                                    return;
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
	NSColor *defaultBoundaryBackground = [[NSColor systemBlueColor] colorWithAlphaComponent:0.08];
	NSColor *defaultBoundaryBorder = [[NSColor systemBlueColor] colorWithAlphaComponent:0.45];

	// Parse hex color strings
	NSString *backgroundHex = style.backgroundColor ? [NSString stringWithUTF8String:style.backgroundColor] : nil;
	NSString *textHex = style.textColor ? [NSString stringWithUTF8String:style.textColor] : nil;
	NSString *matchedTextHex = style.matchedTextColor ? [NSString stringWithUTF8String:style.matchedTextColor] : nil;
	NSString *borderHex = style.borderColor ? [NSString stringWithUTF8String:style.borderColor] : nil;
	NSString *boundaryBackgroundHex =
	    style.boundaryBackgroundColor ? [NSString stringWithUTF8String:style.boundaryBackgroundColor] : nil;
	NSString *boundaryBorderHex =
	    style.boundaryBorderColor ? [NSString stringWithUTF8String:style.boundaryBorderColor] : nil;

	// Apply colors
	self.hintBackgroundColor = [self colorFromHex:backgroundHex defaultColor:defaultBg];
	self.hintTextColor = [self colorFromHex:textHex defaultColor:defaultText];
	self.hintMatchedTextColor = [self colorFromHex:matchedTextHex defaultColor:defaultMatchedText];
	self.hintBorderColor = [self colorFromHex:borderHex defaultColor:defaultBorder];
	self.hintBoundaryBackgroundColor = [self colorFromHex:boundaryBackgroundHex defaultColor:defaultBoundaryBackground];
	self.hintBoundaryBorderColor = [self colorFromHex:boundaryBorderHex defaultColor:defaultBoundaryBorder];

	// Apply geometry properties
	self.hintBorderRadius = style.borderRadius;
	self.hintBorderWidth = style.borderWidth >= 0 ? style.borderWidth : 1.0;
	self.hintPaddingX = style.paddingX;
	self.hintPaddingY = style.paddingY;
	self.hintBoundaryHighlightEnabled = style.boundaryHighlightEnabled ? YES : NO;
	self.hintBoundaryBorderWidth = style.boundaryBorderWidth >= 0 ? style.boundaryBorderWidth : 1.0;
	self.hintBoundaryBorderRadius = style.boundaryBorderRadius >= 0 ? style.boundaryBorderRadius : 4.0;
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

/// The font a recursive-grid transition holds, by family and size. A depth's
/// held size recurs every time that depth is reached, so the lookup behind
/// resolveFont:size:bold: is paid once per size rather than once per keypress.
/// Sizes are whole numbers no larger than the configured one, which bounds it.
- (NSFont *)transitionFontForFamily:(NSString *)family size:(CGFloat)size {
	NSString *key = [NSString stringWithFormat:@"%@|%.0f", family ?: @"", size];
	NSFont *font = self.gridTransitionFontCache[key];
	if (font)
		return font;

	if (family.length > 0)
		font = [self resolveFont:family size:size bold:NO];
	if (!font)
		font = [NSFont systemFontOfSize:size];
	self.gridTransitionFontCache[key] = font;
	return font;
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

/// Whether a placement sits in the top row.
- (BOOL)isTopHintPlacement:(HintPlacement)placement {
	return placement == HintPlacementTop;
}

/// Whether a placement sits in the bottom row.
- (BOOL)isBottomHintPlacement:(HintPlacement)placement {
	return placement == HintPlacementBottom;
}

/// Whether this placement should draw an arrow.
- (BOOL)shouldDrawArrowForPlacement:(HintPlacement)placement showArrow:(BOOL)showArrow {
	return showArrow && ([self isTopHintPlacement:placement] || [self isBottomHintPlacement:placement]);
}

/// Compute the hint label frame in view coordinates.
- (NSRect)hintRectForPlacement:(HintPlacement)placement
                      position:(NSPoint)position
                      boxWidth:(CGFloat)boxWidth
                     boxHeight:(CGFloat)boxHeight
                   arrowHeight:(CGFloat)arrowHeight
                  screenHeight:(CGFloat)screenHeight {
	CGFloat targetX = position.x;
	CGFloat targetY = screenHeight - position.y;
	CGFloat x = targetX - boxWidth / 2.0;
	CGFloat y = targetY - boxHeight / 2.0;

	switch (placement) {
	case HintPlacementTop:
		x = targetX - boxWidth / 2.0;
		y = targetY + kHintArrowGap;
		break;
	case HintPlacementCenter:
		x = targetX - boxWidth / 2.0;
		y = targetY - boxHeight / 2.0;
		break;
	case HintPlacementBottom:
		x = targetX - boxWidth / 2.0;
		y = targetY - kHintArrowGap - arrowHeight - boxHeight;
		break;
	}

	return NSMakeRect(x, y, boxWidth, boxHeight);
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
                     elementCenterY:(CGFloat)elementCenterY
                          placement:(HintPlacement)placement {
	// Tooltip body rectangle (excluding arrow space)
	NSRect bodyRect = rect;
	BOOL topPlacement = [self isTopHintPlacement:placement];
	BOOL bottomPlacement = [self isBottomHintPlacement:placement];
	if (bottomPlacement) {
		bodyRect = NSMakeRect(rect.origin.x, rect.origin.y, rect.size.width, rect.size.height - arrowSize);
	} else if (topPlacement) {
		bodyRect = NSMakeRect(rect.origin.x, rect.origin.y + arrowSize, rect.size.width, rect.size.height - arrowSize);
	}

	// Resolve border radius (-1 = auto pill)
	CGFloat radius = self.hintBorderRadius >= 0.0 ? self.hintBorderRadius : MIN(bodyRect.size.height / 2.0, 6.0);

	// Arrow dimensions
	CGFloat arrowTipX = elementCenterX;
	CGFloat arrowTipY = elementCenterY;
	CGFloat arrowBaseY = bottomPlacement ? bodyRect.origin.y + bodyRect.size.height : bodyRect.origin.y;
	CGFloat arrowWidth = arrowSize * kHintArrowWidthMultiplier;
	CGFloat arrowLeft = arrowTipX - arrowWidth / 2;
	CGFloat arrowRight = arrowTipX + arrowWidth / 2;

	// Clamp arrow to tooltip bounds
	CGFloat tooltipLeft = bodyRect.origin.x + radius;
	CGFloat tooltipRight = bodyRect.origin.x + bodyRect.size.width - radius;
	arrowLeft = MAX(arrowLeft, tooltipLeft);
	arrowRight = MIN(arrowRight, tooltipRight);
	arrowTipX = (arrowLeft + arrowRight) / 2;

	NSBezierPath *path = [NSBezierPath bezierPath];
	CGFloat minX = NSMinX(bodyRect);
	CGFloat maxX = NSMaxX(bodyRect);
	CGFloat minY = NSMinY(bodyRect);
	CGFloat maxY = NSMaxY(bodyRect);

	if (bottomPlacement) {
		[path moveToPoint:NSMakePoint(minX + radius, minY)];
		[path lineToPoint:NSMakePoint(maxX - radius, minY)];
		[path appendBezierPathWithArcWithCenter:NSMakePoint(maxX - radius, minY + radius)
		                                 radius:radius
		                             startAngle:270.0
		                               endAngle:360.0];
		[path lineToPoint:NSMakePoint(maxX, maxY - radius)];
		[path appendBezierPathWithArcWithCenter:NSMakePoint(maxX - radius, maxY - radius)
		                                 radius:radius
		                             startAngle:0.0
		                               endAngle:90.0];
		[path lineToPoint:NSMakePoint(arrowRight, maxY)];
		[path lineToPoint:NSMakePoint(arrowTipX, arrowTipY)];
		[path lineToPoint:NSMakePoint(arrowLeft, maxY)];
		[path lineToPoint:NSMakePoint(minX + radius, maxY)];
		[path appendBezierPathWithArcWithCenter:NSMakePoint(minX + radius, maxY - radius)
		                                 radius:radius
		                             startAngle:90.0
		                               endAngle:180.0];
		[path lineToPoint:NSMakePoint(minX, minY + radius)];
		[path appendBezierPathWithArcWithCenter:NSMakePoint(minX + radius, minY + radius)
		                                 radius:radius
		                             startAngle:180.0
		                               endAngle:270.0];
	} else {
		[path moveToPoint:NSMakePoint(minX + radius, minY)];
		[path lineToPoint:NSMakePoint(arrowLeft, minY)];
		[path lineToPoint:NSMakePoint(arrowTipX, arrowTipY)];
		[path lineToPoint:NSMakePoint(arrowRight, minY)];
		[path lineToPoint:NSMakePoint(maxX - radius, minY)];
		[path appendBezierPathWithArcWithCenter:NSMakePoint(maxX - radius, minY + radius)
		                                 radius:radius
		                             startAngle:270.0
		                               endAngle:360.0];
		[path lineToPoint:NSMakePoint(maxX, maxY - radius)];
		[path appendBezierPathWithArcWithCenter:NSMakePoint(maxX - radius, maxY - radius)
		                                 radius:radius
		                             startAngle:0.0
		                               endAngle:90.0];
		[path lineToPoint:NSMakePoint(minX + radius, maxY)];
		[path appendBezierPathWithArcWithCenter:NSMakePoint(minX + radius, maxY - radius)
		                                 radius:radius
		                             startAngle:90.0
		                               endAngle:180.0];
		[path lineToPoint:NSMakePoint(minX, minY + radius)];
		[path appendBezierPathWithArcWithCenter:NSMakePoint(minX + radius, minY + radius)
		                                 radius:radius
		                             startAngle:180.0
		                               endAngle:270.0];
	}
	[path closePath];
	return path;
}

#pragma mark - Layer Tree Rendering

/// rect grown outward to whole device pixels, so a layer placed there is not
/// resampled and its contents land on the pixels they were drawn for.
static CGRect NeruPixelAlignedRect(CGRect rect, CGFloat scale) {
	CGFloat minX = floor(CGRectGetMinX(rect) * scale) / scale;
	CGFloat minY = floor(CGRectGetMinY(rect) * scale) / scale;
	CGFloat maxX = ceil(CGRectGetMaxX(rect) * scale) / scale;
	CGFloat maxY = ceil(CGRectGetMaxY(rect) * scale) / scale;
	return CGRectMake(minX, minY, maxX - minX, maxY - minY);
}

/// Stroke rect's edge with a layer border. Core Animation draws a border inside
/// the layer's bounds, so growing the layer by half the width centers the
/// border on the edge, where a path stroke sits.
static void NeruPlaceStrokeLayer(CALayer *stroke, CGRect rect, CGFloat width, CGFloat radius, NSColor *color) {
	if (width <= 0.0 || !color) {
		stroke.hidden = YES;
		return;
	}
	stroke.hidden = NO;
	stroke.frame = CGRectInset(rect, -width / 2.0, -width / 2.0);
	stroke.borderWidth = width;
	stroke.borderColor = color.CGColor;
	stroke.cornerRadius = radius > 0.0 ? radius + width / 2.0 : 0.0;
}

/// Bump a style generation when a style's signature changes, so every item
/// drawn with the old style redraws.
static NSUInteger NeruStyleGeneration(NSArray *signature, NSArray *__strong *last, NSUInteger generation) {
	if ([signature isEqualToArray:*last])
		return generation;
	*last = [signature copy];
	return generation + 1;
}

static id NeruOrNull(id object) { return object ?: [NSNull null]; }

- (NeruItemLayer *)makeItemLayer:(NeruItemKind)kind scale:(CGFloat)scale {
	NeruItemLayer *layer = [NeruItemLayer layer];
	layer.kind = kind;
	layer.delegate = self.layerDrawer;
	layer.contentsScale = scale;
	layer.anchorPoint = CGPointZero;
	return layer;
}

- (CALayer *)makePlainLayer {
	CALayer *layer = [CALayer layer];
	layer.delegate = self.layerDrawer;
	layer.anchorPoint = CGPointZero;
	return layer;
}

/// Place an item layer over rect (view coordinates) inside a parent at
/// parentOrigin, and redraw it when its contents or its pixels changed.
/// Leave snap off for items in motion. Resampling a moving layer is invisible,
/// and redrawing it every frame costs time.
- (void)placeItemLayer:(NeruItemLayer *)layer
                  rect:(CGRect)rect
          parentOrigin:(CGPoint)parentOrigin
                 scale:(CGFloat)scale
                  snap:(BOOL)snap
               changed:(BOOL)changed {
	CGRect frame = snap ? NeruPixelAlignedRect(rect, scale) : rect;
	CGRect drawnRect = CGRectOffset(rect, -frame.origin.x, -frame.origin.y);
	if (layer.contentsScale != scale) {
		layer.contentsScale = scale;
		changed = YES;
	}
	if (changed || !CGRectEqualToRect(drawnRect, layer.drawnRect) ||
	    !CGSizeEqualToSize(frame.size, layer.bounds.size)) {
		layer.drawnRect = drawnRect;
		[layer setNeedsDisplay];
	}
	layer.drawOrigin = frame.origin;
	layer.frame = CGRectOffset(frame, -parentOrigin.x, -parentOrigin.y);
	layer.hidden = NO;
}

/// Bring the layer tree in line with the view's state. AppKit calls this after
/// setNeedsDisplay:, at most once per frame.
- (void)renderLayers {
	[CATransaction begin];
	[CATransaction setDisableActions:YES];

	if (self.gridTransitionActive) {
		CGFloat duration = self.gridTransitionDuration > 0 ? self.gridTransitionDuration : 0.18;
		CFTimeInterval elapsed = CACurrentMediaTime() - self.gridTransitionStartTime;
		if (elapsed / duration >= 1.0) {
			// The last frame of a transition is the settled one. It shows only the
			// target grid's cells, with labels at the size that fits them.
			[self cancelGridTransition];
			[self cancelCursorIndicatorTransition];
		}
	}

	CGFloat scale = [self currentBackingScaleFactor];
	[self renderGridAtScale:scale];
	[self renderHintsAtScale:scale];
	[self renderSearchInputAtScale:scale];
	[self renderCursorIndicatorAtScale:scale];

	[CATransaction commit];
}

- (void)drawItemLayer:(NeruItemLayer *)layer inContext:(CGContextRef)ctx {
	[NSGraphicsContext saveGraphicsState];
	NSGraphicsContext *nsContext = [NSGraphicsContext graphicsContextWithCGContext:ctx flipped:NO];
	[NSGraphicsContext setCurrentContext:nsContext];
	CGContextTranslateCTM(ctx, -layer.drawOrigin.x, -layer.drawOrigin.y);

	switch (layer.kind) {
	case NeruItemKindHint:
		[self drawHint:layer.item matchedPrefixLength:layer.matchedPrefixLength];
		break;
	case NeruItemKindText:
		[layer.text drawAtPoint:layer.itemRect.origin
		         withAttributes:@{NSFontAttributeName : layer.font, NSForegroundColorAttributeName : layer.color}];
		break;
	case NeruItemKindSearchInput:
		[self drawSearchInput:layer.item];
		break;
	}

	[NSGraphicsContext restoreGraphicsState];
}

#pragma mark - Hints

/// Place hint layers, one badge and one target highlight per hint. A typed
/// prefix redraws only the badges whose match changed.
- (void)renderHintsAtScale:(CGFloat)scale {
	NSArray *signature = @[
		NeruOrNull(self.hintFont), NeruOrNull(self.hintTextColor), NeruOrNull(self.hintMatchedTextColor),
		NeruOrNull(self.hintBackgroundColor), NeruOrNull(self.hintBorderColor), @(self.hintBorderRadius),
		@(self.hintBorderWidth), @(self.hintPaddingX), @(self.hintPaddingY), @(self.bounds.size.height)
	];
	NSArray *last = self.hintStyleSignature;
	self.hintStyleGeneration = NeruStyleGeneration(signature, &last, self.hintStyleGeneration);
	self.hintStyleSignature = last;
	NSUInteger generation = self.hintStyleGeneration;

	NSUInteger count = [self.hints count];
	while ([self.hintLayers count] > count) {
		NeruItemLayer *layer = [self.hintLayers lastObject];
		[layer.boundary removeFromSuperlayer];
		[layer removeFromSuperlayer];
		[self.hintLayers removeLastObject];
	}

	for (NSUInteger i = 0; i < count; i++) {
		HintItem *hint = self.hints[i];
		NeruItemLayer *layer = i < [self.hintLayers count] ? self.hintLayers[i] : [self addHintLayerAtScale:scale];
		if ([hint.label length] == 0) {
			layer.hidden = YES;
			layer.boundary.hidden = YES;
			continue;
		}

		BOOL changed = layer.item != hint || layer.matchedPrefixLength != hint.matchedPrefixLength ||
		               layer.styleGeneration != generation;
		if (changed) {
			layer.item = hint;
			layer.matchedPrefixLength = hint.matchedPrefixLength;
			layer.styleGeneration = generation;
			layer.itemRect = [self badgeRectForHint:hint];
		}
		[self placeItemLayer:layer rect:layer.itemRect parentOrigin:CGPointZero scale:scale snap:YES changed:changed];
		[self placeBoundaryLayer:layer.boundary forHint:hint];
	}
}

- (NeruItemLayer *)addHintLayerAtScale:(CGFloat)scale {
	CALayer *boundary = [self makePlainLayer];
	[boundary addSublayer:[self makePlainLayer]];
	[self.hintRoot addSublayer:boundary];

	NeruItemLayer *layer = [self makeItemLayer:NeruItemKindHint scale:scale];
	layer.boundary = boundary;
	[self.hintRoot addSublayer:layer];
	[self.hintLayers addObject:layer];
	return layer;
}

/// The target highlight is a filled, stroked rounded rect, which layer
/// properties draw without a bitmap however large the target is.
- (void)placeBoundaryLayer:(CALayer *)boundary forHint:(HintItem *)hint {
	if (!self.hintBoundaryHighlightEnabled || hint.size.width <= 0.0 || hint.size.height <= 0.0) {
		boundary.hidden = YES;
		return;
	}

	CGRect rect = CGRectMake(
	    hint.position.x - hint.size.width / 2.0, self.bounds.size.height - hint.position.y - hint.size.height / 2.0,
	    hint.size.width, hint.size.height);
	CGFloat radius = MIN(self.hintBoundaryBorderRadius, MIN(rect.size.width, rect.size.height) / 2.0);
	boundary.hidden = NO;
	boundary.frame = rect;
	boundary.cornerRadius = radius;
	boundary.backgroundColor = self.hintBoundaryBackgroundColor.CGColor;
	NeruPlaceStrokeLayer(
	    boundary.sublayers.firstObject, boundary.bounds, self.hintBoundaryBorderWidth, radius,
	    self.hintBoundaryBorderColor);
}

/// The hint's label set in the hint font, colored for its matched prefix.
- (NSMutableAttributedString *)hintStringForLabel:(NSString *)label
                              matchedPrefixLength:(int)matchedPrefixLength
                                           buffer:(NSMutableAttributedString *)buffer {
	[[buffer mutableString] setString:label];
	NSRange fullRange = NSMakeRange(0, [label length]);
	[buffer setAttributes:@{NSFontAttributeName : self.hintFont, NSForegroundColorAttributeName : self.hintTextColor}
	                range:fullRange];
	if (matchedPrefixLength > 0 && matchedPrefixLength <= [label length]) {
		[buffer addAttribute:NSForegroundColorAttributeName
		               value:self.hintMatchedTextColor
		               range:NSMakeRange(0, matchedPrefixLength)];
	}
	return buffer;
}

/// Box and arrow geometry for a hint whose text measures textSize.
- (NSRect)hintRectForHint:(HintItem *)hint
                 textSize:(NSSize)textSize
              arrowHeight:(CGFloat *)outArrowHeight
                 boxWidth:(CGFloat *)outBoxWidth {
	HintPlacement placement = (HintPlacement)hint.placement;
	CGFloat paddingX = [self resolvedHintPaddingX];
	CGFloat paddingY = [self resolvedHintPaddingY];
	CGFloat arrowHeight =
	    [self shouldDrawArrowForPlacement:placement showArrow:hint.showArrow] ? kHintArrowHeight : 0.0;
	CGFloat contentWidth = textSize.width + (paddingX * 2);
	CGFloat contentHeight = textSize.height + (paddingY * 2);
	CGFloat boxWidth = MAX(contentWidth, contentHeight);
	CGFloat boxHeight = contentHeight + arrowHeight;
	if (outArrowHeight)
		*outArrowHeight = arrowHeight;
	if (outBoxWidth)
		*outBoxWidth = boxWidth;
	return [self hintRectForPlacement:placement
	                         position:hint.position
	                         boxWidth:boxWidth
	                        boxHeight:boxHeight
	                      arrowHeight:arrowHeight
	                     screenHeight:self.bounds.size.height];
}

/// The badge's extent in view coordinates, stroke and arrow tip included.
- (NSRect)badgeRectForHint:(HintItem *)hint {
	NSMutableAttributedString *measureString = [self hintStringForLabel:hint.label
	                                                matchedPrefixLength:0
	                                                             buffer:self.cachedHintMeasureString];
	CGFloat arrowHeight = 0.0;
	NSRect hintRect = [self hintRectForHint:hint textSize:[measureString size] arrowHeight:&arrowHeight boxWidth:NULL];

	// Expand by border width + 1pt to cover anti-aliased stroke edges
	CGFloat expand = ceil(self.hintBorderWidth / 2.0) + 1.0;
	NSRect badgeRect = NSInsetRect(hintRect, -expand, -expand);
	if (arrowHeight > 0.0) {
		CGFloat targetY = self.bounds.size.height - hint.position.y;
		badgeRect = NSUnionRect(badgeRect, NSMakeRect(hint.position.x - 1.0, targetY - 1.0, 2.0, 2.0));
	}
	return badgeRect;
}

/// Draw one hint's badge and label in view coordinates.
- (void)drawHint:(HintItem *)hint matchedPrefixLength:(int)matchedPrefixLength {
	NSMutableAttributedString *attrString = [self hintStringForLabel:hint.label
	                                             matchedPrefixLength:matchedPrefixLength
	                                                          buffer:self.cachedHintAttributedString];
	NSSize textSize = [attrString size];
	CGFloat arrowHeight = 0.0;
	CGFloat boxWidth = 0.0;
	NSRect hintRect = [self hintRectForHint:hint textSize:textSize arrowHeight:&arrowHeight boxWidth:&boxWidth];
	HintPlacement placement = (HintPlacement)hint.placement;

	// Draw background and border
	CGFloat resolvedBorderRadius =
	    self.hintBorderRadius >= 0.0 ? self.hintBorderRadius : MIN(hintRect.size.height / 2.0, 6.0);
	NSBezierPath *path;
	if (arrowHeight > 0.0) {
		path = [self createTooltipPath:hintRect
		                     arrowSize:arrowHeight
		                elementCenterX:hint.position.x
		                elementCenterY:self.bounds.size.height - hint.position.y
		                     placement:placement];
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
	CGFloat textY = hintRect.origin.y + [self resolvedHintPaddingY];
	if (arrowHeight > 0.0 && [self isTopHintPlacement:placement])
		textY += arrowHeight;
	[attrString drawAtPoint:NSMakePoint(textX, textY)];
}

#pragma mark - Search Input

- (NSMutableAttributedString *)searchInputString:(SearchInputItem *)input {
	NSMutableAttributedString *attrString = self.cachedSearchInputAttributedString;
	NSString *query = input.query ?: @"";
	NSString *display = [query length] > 0 ? [NSString stringWithFormat:@"/ %@", query] : @"/ Search hints";
	if ([query length] > 0) {
		display = [display stringByAppendingFormat:@"  %ld", (long)input.resultCount];
	}
	[[attrString mutableString] setString:display];
	[attrString setAttributes:@{
		NSFontAttributeName : self.searchInputFont,
		NSForegroundColorAttributeName : self.searchInputTextColor
	}
	                    range:NSMakeRange(0, [display length])];
	return attrString;
}

- (CGFloat)searchInputPaddingXResolved {
	return self.searchInputPaddingX >= 0.0 ? self.searchInputPaddingX : MAX(8.0, self.searchInputFont.pointSize * 0.9);
}

- (CGFloat)searchInputPaddingYResolved {
	return self.searchInputPaddingY >= 0.0 ? self.searchInputPaddingY : MAX(5.0, self.searchInputFont.pointSize * 0.5);
}

/// The search box for a text of textSize, in view coordinates.
- (NSRect)searchInputBoxRect:(SearchInputItem *)input textSize:(NSSize)textSize {
	CGFloat width = MAX(input.width, textSize.width + [self searchInputPaddingXResolved] * 2.0);
	CGFloat height = textSize.height + [self searchInputPaddingYResolved] * 2.0;
	return NSMakeRect(input.position.x, self.bounds.size.height - input.position.y - height, width, height);
}

- (void)drawSearchInput:(SearchInputItem *)input {
	NSMutableAttributedString *attrString = [self searchInputString:input];
	NSRect boxRect = [self searchInputBoxRect:input textSize:[attrString size]];
	CGFloat radius =
	    self.searchInputBorderRadius >= 0.0 ? self.searchInputBorderRadius : MIN(boxRect.size.height / 2.0, 8.0);
	NSBezierPath *path = [NSBezierPath bezierPathWithRoundedRect:boxRect xRadius:radius yRadius:radius];

	[self.searchInputBackgroundColor setFill];
	[path fill];

	if (self.searchInputBorderWidth > 0) {
		[self.searchInputBorderColor setStroke];
		[path setLineWidth:self.searchInputBorderWidth];
		[path stroke];
	}

	[attrString drawAtPoint:NSMakePoint(
	                            boxRect.origin.x + [self searchInputPaddingXResolved],
	                            boxRect.origin.y + [self searchInputPaddingYResolved])];
}

- (void)renderSearchInputAtScale:(CGFloat)scale {
	SearchInputItem *input = self.searchInput;
	if (!input) {
		[self.searchInputLayer removeFromSuperlayer];
		self.searchInputLayer = nil;
		return;
	}

	NSArray *signature = @[
		NeruOrNull(self.searchInputFont), NeruOrNull(self.searchInputTextColor),
		NeruOrNull(self.searchInputBackgroundColor), NeruOrNull(self.searchInputBorderColor),
		@(self.searchInputBorderRadius), @(self.searchInputBorderWidth), @(self.searchInputPaddingX),
		@(self.searchInputPaddingY), @(self.bounds.size.height)
	];
	NSArray *last = self.searchInputStyleSignature;
	self.searchInputStyleGeneration = NeruStyleGeneration(signature, &last, self.searchInputStyleGeneration);
	self.searchInputStyleSignature = last;

	if (!self.searchInputLayer) {
		self.searchInputLayer = [self makeItemLayer:NeruItemKindSearchInput scale:scale];
		[self.topRoot insertSublayer:self.searchInputLayer atIndex:0];
	}
	NeruItemLayer *layer = self.searchInputLayer;
	BOOL changed = layer.item != input || layer.styleGeneration != self.searchInputStyleGeneration;
	if (changed) {
		layer.item = input;
		layer.styleGeneration = self.searchInputStyleGeneration;
		NSRect boxRect = [self searchInputBoxRect:input textSize:[[self searchInputString:input] size]];
		CGFloat expand = ceil(self.searchInputBorderWidth / 2.0) + 1.0;
		layer.itemRect = NSInsetRect(boxRect, -expand, -expand);
	}
	[self placeItemLayer:layer rect:layer.itemRect parentOrigin:CGPointZero scale:scale snap:YES changed:changed];
}

#pragma mark - Cursor Indicator

/// Place the virtual cursor indicator. It is the configured character in the
/// configured font and color, centered on the pointer position.
- (void)renderCursorIndicatorAtScale:(CGFloat)scale {
	if (!self.cursorIndicatorVisible || [self.cursorIndicatorLabel length] == 0) {
		[self.cursorIndicatorLayer removeFromSuperlayer];
		self.cursorIndicatorLayer = nil;
		return;
	}

	if (!self.cursorIndicatorLayer) {
		self.cursorIndicatorLayer = [self makeItemLayer:NeruItemKindText scale:scale];
		[self.topRoot addSublayer:self.cursorIndicatorLayer];
	}
	NeruItemLayer *layer = self.cursorIndicatorLayer;
	NSFont *font = self.cursorIndicatorFont ?: [NSFont systemFontOfSize:8.0];
	NSColor *color = self.cursorIndicatorTextColor ?: [NSColor whiteColor];
	BOOL changed = ![layer.text isEqualToString:self.cursorIndicatorLabel] || ![layer.font isEqual:font] ||
	               ![layer.color isEqual:color];
	if (changed) {
		layer.text = self.cursorIndicatorLabel;
		layer.font = font;
		layer.color = color;
		layer.textSize = [layer.text sizeWithAttributes:@{NSFontAttributeName : font}];
	}

	NSPoint position = [self currentCursorIndicatorPosition];
	CGFloat centerY = self.bounds.size.height - position.y;
	NSSize textSize = layer.textSize;
	layer.itemRect =
	    CGRectMake(position.x - textSize.width / 2.0, centerY - textSize.height / 2.0, textSize.width, textSize.height);
	[self placeItemLayer:layer
	                rect:layer.itemRect
	        parentOrigin:CGPointZero
	               scale:scale
	                snap:!self.cursorIndicatorTransitionActive
	             changed:changed];
}

#pragma mark - Glyph Labels

/// Room around a glyph image for strokes and antialiasing that reach past the
/// character's line box.
static const CGFloat kNeruGlyphPadding = 2.0;

/// How many text layouts each glyph set keeps. A dense grid has about 2,300 labels.
static const NSUInteger kNeruTextLayoutCacheLimit = 4096;

/// How many glyph sets to keep. A render uses at most three.
static const NSUInteger kNeruGlyphSetLimit = 16;

/// A cache key for color. Equal colors get the same key even as different objects.
static NSString *NeruColorKey(NSColor *color) {
	NSColor *rgb = [color colorUsingColorSpace:[NSColorSpace sRGBColorSpace]];
	if (!rgb)
		return [color description];
	return [NSString stringWithFormat:@"%.4f,%.4f,%.4f,%.4f", rgb.redComponent, rgb.greenComponent, rgb.blueComponent,
	                                  rgb.alphaComponent];
}

/// The glyph set for font and color at scale, shared by every label drawn in
/// them. A render looks it up once per style, not once per character.
- (NeruGlyphSet *)glyphSetForFont:(NSFont *)font color:(NSColor *)color scale:(CGFloat)scale {
	if (!font || !color)
		return nil;
	NSString *key =
	    [NSString stringWithFormat:@"%@|%.3f|%@|%.2f", font.fontName, font.pointSize, NeruColorKey(color), scale];
	NeruGlyphSet *set = self.glyphSets[key];
	if (!set) {
		// Fonts, colors and scales change with config reloads and displays;
		// dropping every set past the limit keeps the old ones from piling up.
		if ([self.glyphSets count] >= kNeruGlyphSetLimit)
			[self.glyphSets removeAllObjects];
		set = [[NeruGlyphSet alloc] init];
		set.font = font;
		set.color = color;
		set.scale = scale;
		set.glyphs = [NSMutableDictionary dictionary];
		set.layouts = [[NSCache alloc] init];
		set.layouts.countLimit = kNeruTextLayoutCacheLimit;
		self.glyphSets[key] = set;
	}
	return set;
}

/// The image of one character from set, drawn the first time any label needs it.
- (NeruGlyph *)glyph:(NSString *)character inSet:(NeruGlyphSet *)set {
	NeruGlyph *glyph = set.glyphs[character];
	if (glyph)
		return glyph;

	CGFloat scale = set.scale;
	NSDictionary *attrs = @{NSFontAttributeName : set.font, NSForegroundColorAttributeName : set.color};
	NSSize textSize = [character sizeWithAttributes:attrs];
	size_t width = (size_t)ceil((textSize.width + kNeruGlyphPadding * 2.0) * scale);
	size_t height = (size_t)ceil((textSize.height + kNeruGlyphPadding * 2.0) * scale);
	CGColorSpaceRef space = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
	CGContextRef ctx = CGBitmapContextCreate(
	    NULL, width, height, 8, 0, space, kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Host);
	CGColorSpaceRelease(space);
	if (!ctx)
		return nil;

	// Text drawn straight into a layer was font-smoothed, so these glyphs are too.
	CGContextSetAllowsFontSmoothing(ctx, true);
	CGContextSetShouldSmoothFonts(ctx, true);
	CGContextScaleCTM(ctx, scale, scale);
	[NSGraphicsContext saveGraphicsState];
	[NSGraphicsContext setCurrentContext:[NSGraphicsContext graphicsContextWithCGContext:ctx flipped:NO]];
	[character drawAtPoint:NSMakePoint(kNeruGlyphPadding, kNeruGlyphPadding) withAttributes:attrs];
	[NSGraphicsContext restoreGraphicsState];
	CGImageRef image = CGBitmapContextCreateImage(ctx);
	CGContextRelease(ctx);
	if (!image)
		return nil;

	glyph = [[NeruGlyph alloc] init];
	glyph.image = (__bridge_transfer id)image;
	glyph.size = CGSizeMake(width / scale, height / scale);
	set.glyphs[character] = glyph;
	return glyph;
}

/// Fill xs with the x of the glyph starting at each UTF-16 index of line, or
/// NAN where no glyph starts.
static void NeruGlyphStartsInLine(CTLineRef line, CGFloat *xs, NSUInteger length) {
	for (NSUInteger i = 0; i < length; i++)
		xs[i] = NAN;
	for (id run in (__bridge NSArray *)CTLineGetGlyphRuns(line)) {
		CTRunRef ctRun = (__bridge CTRunRef)run;
		CFIndex count = CTRunGetGlyphCount(ctRun);
		if (count <= 0)
			continue;
		CFIndex *indices = malloc(sizeof(CFIndex) * (size_t)count);
		CGPoint *positions = malloc(sizeof(CGPoint) * (size_t)count);
		CTRunGetStringIndices(ctRun, CFRangeMake(0, 0), indices);
		CTRunGetPositions(ctRun, CFRangeMake(0, 0), positions);
		for (CFIndex g = 0; g < count; g++) {
			NSUInteger i = (NSUInteger)indices[g];
			if (i < length && isnan(xs[i]))
				xs[i] = positions[g].x;
		}
		free(indices);
		free(positions);
	}
}

/// The layout of text whose glyph starts are xs, indexed from text's first
/// character. A piece starts where a glyph starts on a character boundary. A
/// ligature covers several characters with one glyph, so they stay one piece
/// and draw as the font shapes them.
static NeruTextLayout *NeruLayoutFromGlyphStarts(NSString *text, const CGFloat *xs, NSSize size) {
	CGFloat base = isnan(xs[0]) ? 0.0 : xs[0];
	NSMutableArray<NSString *> *characters = [NSMutableArray arrayWithCapacity:[text length]];
	NSMutableArray<NSNumber *> *locations = [NSMutableArray arrayWithCapacity:[text length]];
	NSMutableArray<NSNumber *> *offsets = [NSMutableArray arrayWithCapacity:[text length]];
	[text enumerateSubstringsInRange:NSMakeRange(0, [text length])
	                         options:NSStringEnumerationByComposedCharacterSequences
	                      usingBlock:^(NSString *substring, NSRange range, NSRange enclosingRange, BOOL *stop) {
		                      CGFloat x = xs[range.location];
		                      if ([characters count] > 0 && isnan(x)) {
			                      NSUInteger last = [characters count] - 1;
			                      characters[last] = [characters[last] stringByAppendingString:substring];
			                      return;
		                      }
		                      [characters addObject:substring];
		                      [locations addObject:@(range.location)];
		                      [offsets addObject:@(isnan(x) ? 0.0 : x - base)];
	                      }];

	NeruTextLayout *layout = [[NeruTextLayout alloc] init];
	layout.size = size;
	layout.characters = characters;
	layout.locations = locations;
	layout.offsets = offsets;
	return layout;
}

/// How text lays out in set's font, measured the first time it is seen.
- (NeruTextLayout *)layoutOfText:(NSString *)text inSet:(NeruGlyphSet *)set {
	NeruTextLayout *layout = [set.layouts objectForKey:text];
	if (layout)
		return layout;

	NSAttributedString *line = [[NSAttributedString alloc] initWithString:text
	                                                           attributes:@{NSFontAttributeName : set.font}];
	CTLineRef ctLine = CTLineCreateWithAttributedString((__bridge CFAttributedStringRef)line);
	NSUInteger length = [text length];
	NSUInteger slots = MAX(length, (NSUInteger)1);
	CGFloat *xs = malloc(sizeof(CGFloat) * slots);
	NeruGlyphStartsInLine(ctLine, xs, slots);
	layout = NeruLayoutFromGlyphStarts(text, xs, [line size]);
	free(xs);
	CFRelease(ctLine);

	[set.layouts setObject:layout forKey:text];
	return layout;
}

/// Lay out every uncached text in one line rather than a line each. A grid has
/// thousands of labels, and laying out one line per label took most of its
/// first render. A text outside ASCII keeps a line of its own, because a
/// fallback font could change its line height.
- (void)prepareLayoutsOfTexts:(NSArray<NSString *> *)texts inSet:(NeruGlyphSet *)set {
	NSMutableArray<NSString *> *pending = [NSMutableArray arrayWithCapacity:[texts count]];
	NSMutableSet<NSString *> *seen = [NSMutableSet setWithCapacity:[texts count]];
	for (NSString *text in texts) {
		if ([text length] == 0 || [seen containsObject:text] || [set.layouts objectForKey:text] ||
		    ![text canBeConvertedToEncoding:NSASCIIStringEncoding])
			continue;
		[seen addObject:text];
		[pending addObject:text];
	}
	if ([pending count] < 2)
		return;

	// A zero-width space between texts joins no kerning pair or ligature, so
	// each text lays out exactly as it would on a line of its own.
	NSString *joined = [pending componentsJoinedByString:@"\u200B"];
	NSAttributedString *line = [[NSAttributedString alloc] initWithString:joined
	                                                           attributes:@{NSFontAttributeName : set.font}];
	CTLineRef ctLine = CTLineCreateWithAttributedString((__bridge CFAttributedStringRef)line);
	NSUInteger length = [joined length];
	CGFloat *xs = malloc(sizeof(CGFloat) * (length + 1));
	NeruGlyphStartsInLine(ctLine, xs, length);
	xs[length] = CTLineGetTypographicBounds(ctLine, NULL, NULL, NULL);
	// Every ASCII text in one font has the same line height, so measuring one of
	// them the way layoutOfText:inSet: does gives every text's height.
	CGFloat height = [[[NSAttributedString alloc] initWithString:[pending firstObject]
	                                                  attributes:@{NSFontAttributeName : set.font}] size]
	                     .height;

	NSUInteger start = 0;
	for (NSString *text in pending) {
		NSUInteger end = start + [text length];
		if (!isnan(xs[start]) && !isnan(xs[end])) {
			NSSize size = NSMakeSize(xs[end] - xs[start], height);
			[set.layouts setObject:NeruLayoutFromGlyphStarts(text, xs + start, size) forKey:text];
		}
		start = end + 1;
	}
	free(xs);
	CFRelease(ctLine);
}

- (NeruLabelLayer *)makeLabelLayer {
	NeruLabelLayer *label = [NeruLabelLayer layer];
	label.delegate = self.layerDrawer;
	label.anchorPoint = CGPointZero;
	label.glyphLayers = [NSMutableArray array];
	return label;
}

/// Give a label its text, its first matchedPrefixLength characters from
/// matchedGlyphs and the rest from glyphs. It swaps glyphs only when something
/// they show changes.
- (void)setLabel:(NeruLabelLayer *)label
                   text:(NSString *)text
                 glyphs:(NeruGlyphSet *)glyphs
          matchedGlyphs:(NeruGlyphSet *)matchedGlyphs
    matchedPrefixLength:(int)matchedPrefixLength {
	NeruTextLayout *layout = [self layoutOfText:text inSet:glyphs];
	if (label.layout == layout && label.glyphSet == glyphs && label.matchedGlyphSet == matchedGlyphs &&
	    label.matchedPrefixLength == matchedPrefixLength)
		return;
	label.layout = layout;
	label.glyphSet = glyphs;
	label.matchedGlyphSet = matchedGlyphs;
	label.matchedPrefixLength = matchedPrefixLength;

	NSUInteger count = [layout.characters count];
	while ([label.glyphLayers count] > count) {
		[[label.glyphLayers lastObject] removeFromSuperlayer];
		[label.glyphLayers removeLastObject];
	}
	while ([label.glyphLayers count] < count) {
		CALayer *glyphLayer = [self makePlainLayer];
		[label addSublayer:glyphLayer];
		[label.glyphLayers addObject:glyphLayer];
	}

	BOOL prefixInRange = matchedGlyphs && matchedPrefixLength > 0 && matchedPrefixLength <= [text length];
	for (NSUInteger k = 0; k < count; k++) {
		BOOL matched = prefixInRange && [layout.locations[k] intValue] < matchedPrefixLength;
		NeruGlyphSet *set = matched ? matchedGlyphs : glyphs;
		NeruGlyph *glyph = [self glyph:layout.characters[k] inSet:set];
		CALayer *glyphLayer = label.glyphLayers[k];
		glyphLayer.contents = glyph.image;
		glyphLayer.contentsScale = set.scale;
		glyphLayer.bounds = CGRectMake(0, 0, glyph.size.width, glyph.size.height);
	}
}

/// Place a label so its text's line box starts at textOrigin (view
/// coordinates), over a rounded badge at badgeRect when it has one.
- (void)placeLabel:(NeruLabelLayer *)label
          textOrigin:(CGPoint)textOrigin
           badgeRect:(NSRect)badgeRect
           badgeFill:(NSColor *)badgeFill
         badgeBorder:(NSColor *)badgeBorder
    badgeBorderWidth:(CGFloat)badgeBorderWidth
         badgeRadius:(CGFloat)badgeRadius
        parentOrigin:(CGPoint)parentOrigin
               scale:(CGFloat)scale
                snap:(BOOL)snap {
	NSSize textSize = label.layout.size;
	NSRect textRect = NSMakeRect(textOrigin.x, textOrigin.y, textSize.width, textSize.height);
	NSRect rect = NSIsEmptyRect(badgeRect) ? textRect : NSUnionRect(textRect, badgeRect);
	CGRect frame = snap ? NeruPixelAlignedRect(rect, scale) : rect;
	label.frame = CGRectOffset(frame, -parentOrigin.x, -parentOrigin.y);
	label.hidden = NO;

	if (NSIsEmptyRect(badgeRect)) {
		label.badge.hidden = YES;
	} else {
		if (!label.badge) {
			label.badge = [self makePlainLayer];
			[label insertSublayer:label.badge atIndex:0];
		}
		// The badge stroke sits inside the badge, as a layer border does.
		label.badge.hidden = NO;
		label.badge.frame = CGRectOffset(badgeRect, -frame.origin.x, -frame.origin.y);
		label.badge.backgroundColor = badgeFill.CGColor;
		label.badge.cornerRadius = badgeRadius;
		label.badge.borderWidth = badgeBorder ? MAX(badgeBorderWidth, 0.0) : 0.0;
		label.badge.borderColor = badgeBorder.CGColor;
	}

	NSUInteger count = [label.glyphLayers count];
	for (NSUInteger k = 0; k < count; k++) {
		CGPoint origin = CGPointMake(
		    textOrigin.x + [label.layout.offsets[k] doubleValue] - kNeruGlyphPadding, textOrigin.y - kNeruGlyphPadding);
		if (snap)
			origin = CGPointMake(round(origin.x * scale) / scale, round(origin.y * scale) / scale);
		CALayer *glyphLayer = label.glyphLayers[k];
		glyphLayer.position = CGPointMake(origin.x - frame.origin.x, origin.y - frame.origin.y);
	}
}

#pragma mark - Grid

/// The rect a cell border strokes, in view coordinates. Odd widths shift half
/// a point so the stroke covers whole pixels, and the edges at the right and
/// bottom of the screen pull in so their stroke is not cut off.
- (NSRect)gridBorderRectForCellRect:(NSRect)cellRect screenWidth:(CGFloat)screenWidth {
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
	return borderRect;
}

/// The badge rect a label draws in cellRect, or NSZeroRect when it draws as
/// plain text: no badge configured, or a cell too small to hold one.
- (NSRect)gridBadgeRectInCellRect:(NSRect)cellRect textSize:(NSSize)textSize font:(NSFont *)font {
	if (!self.gridDrawLabelBackground)
		return NSZeroRect;

	// Clamp the badge to the cell. A cell too small for one gets plain text.
	CGFloat maxBadgeWidth = MAX(0.0, cellRect.size.width - 4.0);
	CGFloat maxBadgeHeight = MAX(0.0, cellRect.size.height - 4.0);
	if (maxBadgeWidth <= 0.0 || maxBadgeHeight <= 0.0)
		return NSZeroRect;

	CGFloat horizontalPadding = self.gridLabelBackgroundPaddingX >= 0.0 ? self.gridLabelBackgroundPaddingX
	                                                                    : MAX(4.0, round(font.pointSize * 0.4));
	CGFloat verticalPadding = self.gridLabelBackgroundPaddingY >= 0.0 ? self.gridLabelBackgroundPaddingY
	                                                                  : MAX(2.0, round(font.pointSize * 0.2));
	CGFloat badgeWidth = MAX(textSize.width + (horizontalPadding * 2.0), textSize.height + (verticalPadding * 2.0));
	CGFloat badgeHeight = textSize.height + (verticalPadding * 2.0);
	badgeWidth = MIN(badgeWidth, maxBadgeWidth);
	badgeHeight = MIN(badgeHeight, maxBadgeHeight);
	return NSMakeRect(
	    cellRect.origin.x + (cellRect.size.width - badgeWidth) / 2.0,
	    cellRect.origin.y + (cellRect.size.height - badgeHeight) / 2.0, badgeWidth, badgeHeight);
}

- (NeruCellLayer *)addCellLayer {
	NeruCellLayer *cell = [NeruCellLayer layer];
	cell.delegate = self.layerDrawer;
	cell.anchorPoint = CGPointZero;
	cell.border = [self makePlainLayer];
	[cell addSublayer:cell.border];
	cell.subKeys = [NSMutableArray array];
	[self.gridRoot addSublayer:cell];
	[self.cellLayers addObject:cell];
	return cell;
}

/// Place a grid label centered in cellRect (view coordinates), on a badge
/// when configured, or hide it when there is nothing to draw.
- (NeruLabelLayer *)placeGridLabel:(NeruLabelLayer *)label
                              text:(NSString *)text
                            inCell:(NeruCellLayer *)cell
                          cellRect:(NSRect)cellRect
                      parentOrigin:(CGPoint)parentOrigin
                            glyphs:(NeruGlyphSet *)glyphs
                     matchedGlyphs:(NeruGlyphSet *)matchedGlyphs
                         isMatched:(BOOL)isMatched
               matchedPrefixLength:(int)matchedPrefixLength
                           opacity:(CGFloat)opacity
                             scale:(CGFloat)scale
                              snap:(BOOL)snap {
	if ([text length] == 0 || !glyphs || opacity <= 0.0) {
		label.hidden = YES;
		return label;
	}
	if (!label) {
		label = [self makeLabelLayer];
		label.zPosition = 2;
		[cell addSublayer:label];
	}

	[self setLabel:label
	                   text:text
	                 glyphs:glyphs
	          matchedGlyphs:matchedGlyphs
	    matchedPrefixLength:(isMatched ? matchedPrefixLength : 0)];

	NSSize textSize = label.layout.size;
	NSRect badgeRect = [self gridBadgeRectInCellRect:cellRect textSize:textSize font:glyphs.font];
	NSRect centerIn = NSIsEmptyRect(badgeRect) ? cellRect : badgeRect;
	CGPoint textOrigin = CGPointMake(
	    centerIn.origin.x + (centerIn.size.width - textSize.width) / 2.0,
	    centerIn.origin.y + (centerIn.size.height - textSize.height) / 2.0);
	CGFloat maxRadius = MIN(badgeRect.size.width, badgeRect.size.height) / 2.0;
	CGFloat radius = self.gridLabelBackgroundBorderRadius >= 0.0 ? MIN(self.gridLabelBackgroundBorderRadius, maxRadius)
	                                                             : MIN(badgeRect.size.height / 2.0, 6.0);
	[self placeLabel:label
	          textOrigin:textOrigin
	           badgeRect:badgeRect
	           badgeFill:(self.gridLabelBackgroundColor ?: self.gridBackgroundColor)badgeBorder
	                    :(isMatched && self.gridMatchedBorderColor ? self.gridMatchedBorderColor : self.gridBorderColor)
	    badgeBorderWidth:self.gridLabelBackgroundBorderWidth
	         badgeRadius:radius
	        parentOrigin:parentOrigin
	               scale:scale
	                snap:snap];
	label.opacity = opacity;
	return label;
}

/// Place the miniature next-depth key grid inside a cell. When the preview
/// layout has a true center cell (odd cols and odd rows), that center label is
/// omitted so it does not sit directly beneath the cell's own label.
- (void)placeSubKeyPreviewInCell:(NeruCellLayer *)cell
                        cellRect:(NSRect)cellRect
                    parentOrigin:(CGPoint)parentOrigin
                          glyphs:(NeruGlyphSet *)glyphs
                           scale:(CGFloat)scale
                            snap:(BOOL)snap {
	int cols = self.gridSubKeyCols;
	int rows = self.gridSubKeyRows;
	NSArray<NSString *> *labels = self.gridSubKeyLabels;
	NSUInteger used = 0;

	// Labels must hold exactly cols*rows keys for row * cols + col to index them.
	if (glyphs && cols > 0 && rows > 0 && [labels count] == (NSUInteger)(cols * rows)) {
		CGFloat subCellWidth = cellRect.size.width / cols;
		CGFloat subCellHeight = cellRect.size.height / rows;
		NSUInteger centerIdx =
		    (cols % 2 == 1 && rows % 2 == 1) ? (NSUInteger)((rows / 2) * cols + (cols / 2)) : NSNotFound;

		for (int row = 0; row < rows; row++) {
			for (int col = 0; col < cols; col++) {
				NSUInteger idx = (NSUInteger)(row * cols + col);
				NSString *subLabel = labels[idx];
				if (idx == centerIdx || [subLabel length] == 0)
					continue;

				NeruLabelLayer *label = used < [cell.subKeys count] ? cell.subKeys[used] : nil;
				if (!label) {
					label = [self makeLabelLayer];
					label.zPosition = 1;
					[cell addSublayer:label];
					[cell.subKeys addObject:label];
				}
				used++;

				[self setLabel:label text:subLabel glyphs:glyphs matchedGlyphs:nil matchedPrefixLength:0];

				// Sub-cells: row 0 is the top of the cell, which is the larger Y here.
				CGFloat subOriginX = cellRect.origin.x + col * subCellWidth;
				CGFloat subOriginY = cellRect.origin.y + (rows - 1 - row) * subCellHeight;
				NSSize textSize = label.layout.size;
				[self placeLabel:label
				          textOrigin:CGPointMake(
				                         subOriginX + (subCellWidth - textSize.width) / 2.0,
				                         subOriginY + (subCellHeight - textSize.height) / 2.0)
				           badgeRect:NSZeroRect
				           badgeFill:nil
				         badgeBorder:nil
				    badgeBorderWidth:0.0
				         badgeRadius:0.0
				        parentOrigin:parentOrigin
				               scale:scale
				                snap:snap];
			}
		}
	}

	while ([cell.subKeys count] > used) {
		[[cell.subKeys lastObject] removeFromSuperlayer];
		[cell.subKeys removeLastObject];
	}
}

/// Place one cell layer per grid cell. Mid-transition the cells are the
/// interpolated ones, unmatched, cross-fading from their old labels to their
/// new. Otherwise a cell nothing changed for is left alone.
- (void)renderGridAtScale:(CGFloat)scale {
	// Whether a label fits its cell, and at what size, is decided by
	// recursivegrid.Style.LabelFontSizeIn and handed over settled, including
	// the size a transition holds from its first frame to its last. Nothing is
	// measured against the interpolated rect here.
	BOOL inTransition = self.gridTransitionActive;
	BOOL hideLabel = inTransition ? self.gridTransitionHideLabel : self.gridHideLabel;
	NSFont *labelFont = (inTransition && self.gridTransitionFont) ? self.gridTransitionFont : self.gridFont;
	BOOL drawSubKeys = self.gridDrawSubKeyPreview && !(inTransition && self.gridTransitionHideSubKeyPreview);
	NSFont *subKeyFont =
	    (inTransition && self.gridTransitionSubKeyFont) ? self.gridTransitionSubKeyFont : self.gridSubKeyFont;

	NSArray *signature = @[
		NeruOrNull(self.cachedGridTextColor),
		NeruOrNull(self.cachedGridMatchedTextColor),
		NeruOrNull(self.gridLabelBackgroundColor),
		NeruOrNull(self.gridBackgroundColor),
		NeruOrNull(self.gridMatchedBackgroundColor),
		NeruOrNull(self.gridMatchedBorderColor),
		NeruOrNull(self.gridBorderColor),
		@(self.gridBorderWidth),
		@(self.gridDrawLabelBackground),
		@(self.gridLabelBackgroundPaddingX),
		@(self.gridLabelBackgroundPaddingY),
		@(self.gridLabelBackgroundBorderRadius),
		@(self.gridLabelBackgroundBorderWidth),
		NeruOrNull(labelFont),
		@(hideLabel),
		@(drawSubKeys),
		NeruOrNull(subKeyFont),
		NeruOrNull(self.gridSubKeyTextColor),
		NeruOrNull(self.gridSubKeyLabels),
		@(self.gridSubKeyCols),
		@(self.gridSubKeyRows),
		@(scale),
		@(self.bounds.size.width),
		@(self.bounds.size.height)
	];
	NSArray *last = self.gridStyleSignature;
	self.gridStyleGeneration = NeruStyleGeneration(signature, &last, self.gridStyleGeneration);
	self.gridStyleSignature = last;
	NSUInteger generation = self.gridStyleGeneration;
	NeruGlyphSet *glyphs = [self glyphSetForFont:labelFont color:self.cachedGridTextColor scale:scale];
	NeruGlyphSet *matchedGlyphs = [self glyphSetForFont:labelFont color:self.cachedGridMatchedTextColor scale:scale];
	NeruGlyphSet *subKeyGlyphs =
	    drawSubKeys ? [self glyphSetForFont:subKeyFont color:self.gridSubKeyTextColor scale:scale] : nil;

	// At the first label with no cached layout, the render lays out that label
	// and every one after it in one line. A transition has few labels, so it
	// lays them out one at a time.
	BOOL layoutsPrepared = inTransition || !glyphs;
	NSArray<GridCellItem *> *fromCells = inTransition ? (self.transitionFromGridCells ?: @[]) : nil;
	NSArray<GridCellItem *> *toCells = inTransition ? (self.transitionToGridCells ?: @[]) : self.gridCells;
	NSUInteger count = inTransition ? MAX([fromCells count], [toCells count]) : [toCells count];
	CGFloat progress = inTransition ? [self currentGridTransitionProgress] : 1.0;
	CGRect fromBounds = CGRectNull;
	CGRect toBounds = CGRectNull;
	if (inTransition) {
		for (GridCellItem *cell in fromCells)
			fromBounds = CGRectIsNull(fromBounds) ? cell.bounds : CGRectUnion(fromBounds, cell.bounds);
		for (GridCellItem *cell in toCells)
			toBounds = CGRectIsNull(toBounds) ? cell.bounds : CGRectUnion(toBounds, cell.bounds);
		if (CGRectIsNull(fromBounds))
			fromBounds = CGRectIsNull(toBounds) ? CGRectZero : toBounds;
		if (CGRectIsNull(toBounds))
			toBounds = fromBounds;
	}

	// An empty grid hides its cells rather than dropping them. They hold no
	// bitmaps, and the next activation usually shows the same cells again, so
	// it costs a lookup per cell instead of a rebuilt tree.
	self.gridRoot.hidden = count == 0;
	if (count == 0)
		return;
	while ([self.cellLayers count] > count) {
		[[self.cellLayers lastObject] removeFromSuperlayer];
		[self.cellLayers removeLastObject];
	}

	CGFloat screenHeight = self.bounds.size.height;
	CGFloat screenWidth = self.bounds.size.width;
	for (NSUInteger idx = 0; idx < count; idx++) {
		NeruCellLayer *cell = idx < [self.cellLayers count] ? self.cellLayers[idx] : [self addCellLayer];
		CGRect rect;
		BOOL isMatched = NO;
		int matchedPrefixLength = 0;
		NSString *label = nil;
		NSString *fadeLabel = nil;
		CGFloat labelOpacity = 1.0;

		if (inTransition) {
			GridCellItem *fromCell = idx < [fromCells count] ? fromCells[idx] : nil;
			GridCellItem *toCell = idx < [toCells count] ? toCells[idx] : nil;
			CGRect startRect = fromCell ? fromCell.bounds : fromBounds;
			CGRect endRect = toCell ? toCell.bounds : toBounds;
			rect = CGRectMake(
			    startRect.origin.x + (endRect.origin.x - startRect.origin.x) * progress,
			    startRect.origin.y + (endRect.origin.y - startRect.origin.y) * progress,
			    startRect.size.width + (endRect.size.width - startRect.size.width) * progress,
			    startRect.size.height + (endRect.size.height - startRect.size.height) * progress);
			NSString *fromLabel = fromCell.label ?: @"";
			label = toCell.label ?: fromLabel;
			if (![fromLabel isEqualToString:label]) {
				fadeLabel = fromLabel;
				labelOpacity = progress;
			}
			cell.placedGeneration = 0;
		} else {
			GridCellItem *item = toCells[idx];
			if (self.hideUnmatched && !item.isMatched && !item.isSubgrid) {
				if (!cell.hidden)
					cell.hidden = YES;
				continue;
			}
			rect = item.bounds;
			isMatched = item.isMatched;
			matchedPrefixLength = item.matchedPrefixLength;
			label = item.label ?: @"";

			BOOL unchanged = cell.placedGeneration == generation && CGRectEqualToRect(cell.placedRect, rect) &&
			                 cell.placedMatched == isMatched && cell.placedMatchedPrefixLength == matchedPrefixLength &&
			                 [cell.placedLabel isEqualToString:label];
			if (cell.hidden)
				cell.hidden = NO;
			if (unchanged)
				continue;
			cell.placedGeneration = generation;
			cell.placedRect = rect;
			cell.placedMatched = isMatched;
			cell.placedMatchedPrefixLength = matchedPrefixLength;
			cell.placedLabel = label;
		}

		NSRect cellRect = NSMakeRect(
		    rect.origin.x, screenHeight - rect.origin.y - rect.size.height, rect.size.width, rect.size.height);

		// The fill covers the cell exactly and the border straddles its edge.
		// Cells stack in order, so each one's fill covers the outer half of the
		// border before it, as filling each cell after the last one's border did.
		NSRect borderRect = [self gridBorderRectForCellRect:cellRect screenWidth:screenWidth];
		cell.hidden = NO;
		cell.frame = cellRect;
		cell.backgroundColor =
		    (isMatched && self.gridMatchedBackgroundColor ? self.gridMatchedBackgroundColor : self.gridBackgroundColor)
		        .CGColor;
		NeruPlaceStrokeLayer(
		    cell.border, NSOffsetRect(borderRect, -cellRect.origin.x, -cellRect.origin.y), self.gridBorderWidth, 0.0,
		    (isMatched && self.gridMatchedBorderColor ? self.gridMatchedBorderColor : self.gridBorderColor));

		// A cell with no label draws no preview, except mid-transition.
		BOOL snap = !inTransition;
		BOOL previewFits = drawSubKeys && ([label length] > 0 || inTransition);
		[self placeSubKeyPreviewInCell:cell
		                      cellRect:cellRect
		                  parentOrigin:cellRect.origin
		                        glyphs:(previewFits ? subKeyGlyphs : nil)scale:scale
		                          snap:snap];

		if (!layoutsPrepared && !hideLabel && [label length] > 0 && ![glyphs.layouts objectForKey:label]) {
			layoutsPrepared = YES;
			NSMutableArray<NSString *> *texts = [NSMutableArray arrayWithCapacity:count - idx];
			for (NSUInteger next = idx; next < count; next++)
				[texts addObject:toCells[next].label ?: @""];
			[self prepareLayoutsOfTexts:texts inSet:glyphs];
		}

		cell.label = [self placeGridLabel:cell.label
		                             text:(hideLabel ? nil : label)inCell:cell
		                         cellRect:cellRect
		                     parentOrigin:cellRect.origin
		                           glyphs:glyphs
		                    matchedGlyphs:matchedGlyphs
		                        isMatched:isMatched
		              matchedPrefixLength:matchedPrefixLength
		                          opacity:labelOpacity
		                            scale:scale
		                             snap:snap];
		cell.fadeLabel = [self placeGridLabel:cell.fadeLabel
		                                 text:(hideLabel ? nil : fadeLabel)inCell:cell
		                             cellRect:cellRect
		                         parentOrigin:cellRect.origin
		                               glyphs:glyphs
		                        matchedGlyphs:nil
		                            isMatched:NO
		                  matchedPrefixLength:0
		                              opacity:1.0 - progress
		                                scale:scale
		                                 snap:snap];
	}
}

@end

#pragma mark - Overlay Window Controller Interface

@interface OverlayWindowController : NSObject
@property(nonatomic, strong) NSPanel *window;           ///< Panel instance (non-activating overlay)
@property(nonatomic, strong) OverlayView *overlayView;  ///< Overlay view instance
@property(nonatomic, assign) NSInteger sharingType;     ///< Current window sharing type
@property(nonatomic, assign) BOOL sharingTypeExplicit;  ///< Whether sharingType was explicitly configured
@property(nonatomic, assign) BOOL shouldBeVisible;      ///< Whether the window should currently be visible on screen
@property(nonatomic, assign) BOOL needsWindowServerReattach;
@property(nonatomic, assign) BOOL windowServerReattachScheduled;
@property(nonatomic, assign) CFTimeInterval lastOnscreenVerifyTime;
@property(nonatomic, assign) int onscreenProbeFailureStreak;
@property(nonatomic, assign) uint64_t freshOrderGeneration;
- (void)applyOverlayCollectionBehavior;
- (void)reattachToAllSpacesIfVisible;
- (void)verifyOnscreenAfterFreshOrder;
@end

// Coalesces bursts of invalidation notifications into one reattach cycle.
static const int64_t kNeruWindowServerReattachDebounceNs = 300 * NSEC_PER_MSEC;

// Rate limit for the synchronous onscreen probe: keeps it off per-tick and
// per-keypress paths and caps a failing probe at one reattach per interval.
static const CFTimeInterval kNeruOnscreenVerifyInterval = 1.0;

// A probe still failing after this many repairs is lying (e.g. a list API
// quirk) — stop probing rather than blink the overlay forever.
static const int kNeruOnscreenProbeFailureLimit = 3;

// Delay before the one-shot probe that follows a fresh order-front. The
// WindowServer needs a few frames to commit the order, and the repair of a
// pinned window still lands before the user notices it missing.
static const int64_t kNeruFreshOrderVerifyDelayNs = 80 * NSEC_PER_MSEC;

static BOOL NeruWindowIsOnscreenPerWindowServer(NSInteger windowNumber);

#pragma mark - Overlay Window Controller Implementation

@implementation OverlayWindowController

- (void)dealloc {
	[[[NSWorkspace sharedWorkspace] notificationCenter] removeObserver:self];
	[[NSNotificationCenter defaultCenter] removeObserver:self];
}

/// Initialize
/// @return Initialized instance
- (instancetype)init {
	self = [super init];
	if (self) {
		_shouldBeVisible = NO;
		_needsWindowServerReattach = NO;
		_windowServerReattachScheduled = NO;
		[self createWindow];
	}
	return self;
}

- (void)applyOverlayCollectionBehavior {
	[self.window setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces |
	                                   NSWindowCollectionBehaviorStationary | NSWindowCollectionBehaviorIgnoresCycle |
	                                   NSWindowCollectionBehaviorFullScreenAuxiliary];
}

- (BOOL)hasDrawableFrame {
	NSRect frame = self.window.frame;
	return frame.size.width > 1.0 || frame.size.height > 1.0;
}

// Expensive recovery path for a genuinely invalidated window-server
// attachment (wake, display reconfiguration) — never for routine Space
// switches; `delayNs` absorbs notification bursts into one pending cycle.
- (void)reattachToAllSpacesIfVisibleAfterDelay:(int64_t)delayNs {
	if (!self.shouldBeVisible || ![self hasDrawableFrame] || self.windowServerReattachScheduled)
		return;

	self.windowServerReattachScheduled = YES;
	dispatch_after(dispatch_time(DISPATCH_TIME_NOW, delayNs), dispatch_get_main_queue(), ^{
		if (!self.shouldBeVisible || ![self hasDrawableFrame]) {
			self.windowServerReattachScheduled = NO;
			return;
		}

		[self.window orderOut:nil];
		[self.window setCollectionBehavior:NSWindowCollectionBehaviorDefault];
		dispatch_async(dispatch_get_main_queue(), ^{
			self.windowServerReattachScheduled = NO;
			if (!self.shouldBeVisible || ![self hasDrawableFrame])
				return;

			self.needsWindowServerReattach = NO;
			[self applyOverlayCollectionBehavior];
			[self.window setIsVisible:YES];
			[self.window orderFrontRegardless];
			[self.window display];
			[self.overlayView setNeedsDisplay:YES];
			// Just reordered — hold off probing for one interval. One
			// detach/reattach does not always un-pin, so confirm it took.
			self.lastOnscreenVerifyTime = CACurrentMediaTime();
			[self verifyOnscreenAfterFreshOrder];
		});
	});
}

- (void)reattachToAllSpacesIfVisible {
	[self reattachToAllSpacesIfVisibleAfterDelay:0];
}

// A healthy CanJoinAllSpaces window is already on the new Space; repair only
// a window AppKit or the WindowServer reports dropped.
- (void)reassertOverlayOnActiveSpace {
	if (!self.shouldBeVisible || ![self hasDrawableFrame] || self.windowServerReattachScheduled)
		return;

	if (self.window.isVisible && self.window.isOnActiveSpace) {
		// AppKit looks healthy — double-check server truth, rate-limited.
		CFTimeInterval now = CACurrentMediaTime();
		if (now - self.lastOnscreenVerifyTime < kNeruOnscreenVerifyInterval ||
		    self.onscreenProbeFailureStreak >= kNeruOnscreenProbeFailureLimit)
			return;

		self.lastOnscreenVerifyTime = now;
		if (NeruWindowIsOnscreenPerWindowServer(self.window.windowNumber)) {
			self.onscreenProbeFailureStreak = 0;
			return;
		}
		self.onscreenProbeFailureStreak++;
	}

	// Re-applying cached-identical state never reaches the server; only the
	// full detach/reattach cycle un-pins a window stuck on one Space.
	self.needsWindowServerReattach = YES;
	[self reattachToAllSpacesIfVisible];
}

// A window hidden across a fullscreen transition comes back pinned to the
// Space it last showed on. The Space handler skips hidden windows and the
// order-time probe skips fresh orders, so neither sees it. A Hide before the
// tick cancels the check.
- (void)verifyOnscreenAfterFreshOrder {
	uint64_t generation = ++self.freshOrderGeneration;
	dispatch_after(dispatch_time(DISPATCH_TIME_NOW, kNeruFreshOrderVerifyDelayNs), dispatch_get_main_queue(), ^{
		if (generation != self.freshOrderGeneration || !self.shouldBeVisible || ![self hasDrawableFrame] ||
		    self.windowServerReattachScheduled || !self.window.isVisible)
			return;

		// AppKit's cache can claim the active Space while the server disagrees,
		// so the server confirms a healthy answer. That is one enumeration per Show.
		self.lastOnscreenVerifyTime = CACurrentMediaTime();
		BOOL pinned = !self.window.isOnActiveSpace || !NeruWindowIsOnscreenPerWindowServer(self.window.windowNumber);
		if (!pinned) {
			self.onscreenProbeFailureStreak = 0;
			return;
		}
		// The streak gates the repair, not the check, so a repair that took
		// on the last allowed attempt still clears the streak above.
		if (self.onscreenProbeFailureStreak >= kNeruOnscreenProbeFailureLimit)
			return;
		self.onscreenProbeFailureStreak++;
		self.needsWindowServerReattach = YES;
		[self reattachToAllSpacesIfVisible];
	});
}

- (void)handleActiveSpaceDidChange:(NSNotification *)notification {
	(void)notification;
	if ([NSThread isMainThread]) {
		[self reassertOverlayOnActiveSpace];
		return;
	}

	dispatch_async(dispatch_get_main_queue(), ^{
		[self reassertOverlayOnActiveSpace];
	});
}

- (void)handleWindowServerAttachmentInvalidated:(NSNotification *)notification {
	(void)notification;
	if ([NSThread isMainThread]) {
		self.needsWindowServerReattach = YES;
		[self reattachToAllSpacesIfVisibleAfterDelay:kNeruWindowServerReattachDebounceNs];
		return;
	}

	dispatch_async(dispatch_get_main_queue(), ^{
		self.needsWindowServerReattach = YES;
		[self reattachToAllSpacesIfVisibleAfterDelay:kNeruWindowServerReattachDebounceNs];
	});
}

- (void)handleWindowServerAttachmentWillInvalidate:(NSNotification *)notification {
	(void)notification;
	if ([NSThread isMainThread]) {
		self.needsWindowServerReattach = YES;
		return;
	}

	dispatch_async(dispatch_get_main_queue(), ^{
		self.needsWindowServerReattach = YES;
	});
}

/// Create window
- (void)createWindow {
	// Start at 1x1 so the backing store is minimal until a resize is applied.
	// Full-screen overlays call NeruResizeOverlayToActiveScreen before Show, and
	// small indicator overlays (mode indicator, sticky modifiers) use
	// NeruPositionOverlayRelative to set a small frame centered on the cursor.
	// This saves some memory of backing store per hidden Retina overlay.
	NSRect initialRect = NSMakeRect(0, 0, 1, 1);

	// Use NSPanel for better floating overlay behavior.
	// Non-activating panel won't steal focus from other apps.
	NSPanel *panel =
	    [[NSPanel alloc] initWithContentRect:initialRect
	                               styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
	                                 backing:NSBackingStoreBuffered
	                                   defer:NO];
	[panel setHidesOnDeactivate:NO];
	[panel setReleasedWhenClosed:NO];
	[panel setTitle:@"neru-overlay"];

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
	[self applyOverlayCollectionBehavior];

	[[[NSWorkspace sharedWorkspace] notificationCenter] addObserver:self
	                                                       selector:@selector(handleActiveSpaceDidChange:)
	                                                           name:NSWorkspaceActiveSpaceDidChangeNotification
	                                                         object:nil];
	[[[NSWorkspace sharedWorkspace] notificationCenter] addObserver:self
	                                                       selector:@selector(handleWindowServerAttachmentInvalidated:)
	                                                           name:NSWorkspaceDidWakeNotification
	                                                         object:nil];
	[[[NSWorkspace sharedWorkspace] notificationCenter]
	    addObserver:self
	       selector:@selector(handleWindowServerAttachmentWillInvalidate:)
	           name:NSWorkspaceWillSleepNotification
	         object:nil];
	[[NSNotificationCenter defaultCenter] addObserver:self
	                                         selector:@selector(handleWindowServerAttachmentInvalidated:)
	                                             name:NSApplicationDidChangeScreenParametersNotification
	                                           object:nil];

	// Set sharing type — default to visible (NSWindowSharingReadOnly = 1) unless explicitly configured
	if (!self.sharingTypeExplicit) {
		self.sharingType = NSWindowSharingReadOnly;
	}
	[self.window setSharingType:self.sharingType];

	// Create and attach overlay view
	NSRect viewFrame = NSMakeRect(0, 0, 1, 1);
	self.overlayView = [[OverlayView alloc] initWithFrame:viewFrame];
	[self.window setContentView:self.overlayView];
}

@end

#pragma mark - C Interface Implementation

/// Server-truth visibility check — AppKit's cached window state goes stale
/// when the WindowServer silently drops a Space attachment. Must enumerate via
/// CGWindowListCopyWindowInfo: CGWindowListCreateDescriptionFromArray omits
/// NSWindowSharingNone windows, which overlays are when hidden from capture.
static BOOL NeruWindowIsOnscreenPerWindowServer(NSInteger windowNumber) {
	if (windowNumber <= 0)
		return NO;

	CFArrayRef windows = CGWindowListCopyWindowInfo(kCGWindowListOptionAll, kCGNullWindowID);
	if (!windows)
		return YES;  // enumeration failure is not evidence of detachment

	BOOL onscreen = NO;
	CFIndex count = CFArrayGetCount(windows);
	for (CFIndex i = 0; i < count; i++) {
		NSDictionary *info = (__bridge NSDictionary *)CFArrayGetValueAtIndex(windows, i);
		NSNumber *number = info[(__bridge NSString *)kCGWindowNumber];
		if (number.integerValue == windowNumber) {
			onscreen = [info[(__bridge NSString *)kCGWindowIsOnscreen] boolValue];
			break;
		}
	}
	CFRelease(windows);

	return onscreen;
}

/// Order an overlay window front once it has a drawable frame.
/// Hidden overlays are shrunk to 1x1 to release backing memory; callers may
/// request Show before the small indicator window has been resized. In that
/// case Show records shouldBeVisible but intentionally does not order the tiny
/// window. The resize path calls this helper again after it has a real frame.
static void NeruOrderOverlayWindowIfDrawable(OverlayWindowController *controller, BOOL displayNow) {
	if (!controller || !controller.shouldBeVisible)
		return;

	NSRect frame = controller.window.frame;
	if (frame.size.width <= 1.0 && frame.size.height <= 1.0)
		return;

	if (controller.needsWindowServerReattach || controller.windowServerReattachScheduled) {
		[controller reattachToAllSpacesIfVisible];
		return;
	}

	BOOL freshlyOrdered = !controller.window.isVisible;

	[controller applyOverlayCollectionBehavior];
	[controller.window setIsVisible:YES];
	[controller.window orderFrontRegardless];

	// The server can silently pin the window to one Space while AppKit's
	// cache still looks healthy, making the calls above no-ops — probe server
	// truth and repair with the full reattach cycle. A just-ordered window is
	// not committed server-side yet, so it gets one interval before probing.
	CFTimeInterval now = CACurrentMediaTime();
	if (freshlyOrdered) {
		controller.lastOnscreenVerifyTime = now;
		[controller verifyOnscreenAfterFreshOrder];
	} else if (
	    now - controller.lastOnscreenVerifyTime >= kNeruOnscreenVerifyInterval &&
	    controller.onscreenProbeFailureStreak < kNeruOnscreenProbeFailureLimit) {
		controller.lastOnscreenVerifyTime = now;
		if (!NeruWindowIsOnscreenPerWindowServer(controller.window.windowNumber)) {
			controller.onscreenProbeFailureStreak++;
			controller.needsWindowServerReattach = YES;
			[controller reattachToAllSpacesIfVisible];
			return;
		}
		controller.onscreenProbeFailureStreak = 0;
	}

	if (displayNow) {
		[controller.window display];
	}

	[controller.overlayView setNeedsDisplay:YES];
}

/// Create overlay window
/// @return Overlay window handle
OverlayWindow NeruCreateOverlayWindow(void) {
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

	void (^destroyBlock)(void) = ^{
		@autoreleasepool {
			OverlayWindowController *controller = CFBridgingRelease(window);
			controller.shouldBeVisible = NO;
			[controller.window close];
		}
	};

	if ([NSThread isMainThread]) {
		destroyBlock();
	} else {
		dispatch_sync(dispatch_get_main_queue(), destroyBlock);
	}
}

/// Show overlay window
/// @param window Overlay window handle
void NeruShowOverlayWindow(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			controller.shouldBeVisible = YES;

			[controller.window setLevel:kCGMaximumWindowLevel];
			[controller applyOverlayCollectionBehavior];

			NeruOrderOverlayWindowIfDrawable(controller, YES);
		}
	});
}

/// Hide overlay window
/// @param window Overlay window handle
void NeruHideOverlayWindow(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	// Hiding keeps the window-server attachment intact, so it must not set
	// needsWindowServerReattach — only invalidation events (sleep, display
	// reconfiguration) do.
	if ([NSThread isMainThread]) {
		controller.shouldBeVisible = NO;
		controller.freshOrderGeneration++;
		[controller.window orderOut:nil];
		// Shrink to 1x1 to release the large backing store (saves ~47MB per
		// Retina-resolution full-screen window). The next resize/show call
		// will restore the proper frame before the window becomes visible.
		[controller.window setFrame:NSMakeRect(0, 0, 1, 1) display:NO];
		[controller.overlayView setFrame:NSMakeRect(0, 0, 1, 1)];
	} else {
		dispatch_async(dispatch_get_main_queue(), ^{
			@autoreleasepool {
				controller.shouldBeVisible = NO;
				controller.freshOrderGeneration++;
				[controller.window orderOut:nil];
				[controller.window setFrame:NSMakeRect(0, 0, 1, 1) display:NO];
				[controller.overlayView setFrame:NSMakeRect(0, 0, 1, 1)];
			}
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
		[controller.overlayView clearContent];
	} else {
		dispatch_async(dispatch_get_main_queue(), ^{
			@autoreleasepool {
				[controller.overlayView clearContent];
			}
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
		@autoreleasepool {
			NSScreen *mainScreen = [NSScreen mainScreen];
			if (!mainScreen)
				return;

			NSRect screenFrame = [mainScreen frame];
			[controller.window setFrame:screenFrame display:YES];

			NSRect viewFrame = NSMakeRect(0, 0, screenFrame.size.width, screenFrame.size.height);
			[controller.overlayView setFrame:viewFrame];
			[controller.overlayView setNeedsDisplay:YES];

			[controller.window setLevel:kCGMaximumWindowLevel];
			NeruOrderOverlayWindowIfDrawable(controller, NO);
		}
	});
}

/// Resize overlay to active screen
/// @param window Overlay window handle
void NeruResizeOverlayToActiveScreen(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
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

			[controller.window setLevel:kCGMaximumWindowLevel];
			NeruOrderOverlayWindowIfDrawable(controller, NO);
		}
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
		@autoreleasepool {
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

			[controller.window setLevel:kCGMaximumWindowLevel];
			NeruOrderOverlayWindowIfDrawable(controller, NO);

			if (callback)
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
	if (style->boundaryBackgroundColor)
		free((void *)style->boundaryBackgroundColor);
	if (style->boundaryBorderColor)
		free((void *)style->boundaryBorderColor);
}

static inline void free_search_input_style_strings(const SearchInputStyle *style) {
	if (style->fontFamily)
		free((void *)style->fontFamily);
	if (style->backgroundColor)
		free((void *)style->backgroundColor);
	if (style->textColor)
		free((void *)style->textColor);
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
/// @param placement Label placement relative to the target
/// @return Array of HintItem objects
static NSMutableArray<HintItem *> *buildHintItems(HintData *hints, int count, BOOL showArrow, int placement) {
	NSMutableArray<HintItem *> *hintItems = [NSMutableArray arrayWithCapacity:count];
	for (int i = 0; i < count; i++) {
		HintData hint = hints[i];
		HintItem *hintItem = [[HintItem alloc] init];
		hintItem.label = hint.label ? @(hint.label) : @"";
		hintItem.position = hint.position;
		hintItem.size = hint.size;
		hintItem.matchedPrefixLength = hint.matchedPrefixLength;
		hintItem.showArrow = showArrow;
		hintItem.placement = placement;
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

	@autoreleasepool {
		OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

		// Build hint items upfront — safe from any thread
		NSMutableArray<HintItem *> *hintItems =
		    buildHintItems(hints, count, style.showArrow ? YES : NO, style.placement);

		if ([NSThread isMainThread]) {
			[controller.overlayView.hints removeAllObjects];
			[controller.overlayView applyStyle:style];
			[controller.overlayView.hints addObjectsFromArray:hintItems];
			[controller.overlayView setNeedsDisplay:YES];
			NeruOrderOverlayWindowIfDrawable(controller, style.forceFlush ? YES : NO);
		} else {
			// Copy style strings before crossing the thread boundary
			HintStyle styleCopy = {
			    .fontSize = style.fontSize,
			    .borderRadius = style.borderRadius,
			    .borderWidth = style.borderWidth,
			    .paddingX = style.paddingX,
			    .paddingY = style.paddingY,
			    .showArrow = style.showArrow,
			    .placement = style.placement,
			    .forceFlush = style.forceFlush,
			    .boundaryHighlightEnabled = style.boundaryHighlightEnabled,
			    .boundaryBorderWidth = style.boundaryBorderWidth,
			    .boundaryBorderRadius = style.boundaryBorderRadius,
			    .fontFamily = safe_strdup(style.fontFamily),
			    .backgroundColor = safe_strdup(style.backgroundColor),
			    .textColor = safe_strdup(style.textColor),
			    .matchedTextColor = safe_strdup(style.matchedTextColor),
			    .borderColor = safe_strdup(style.borderColor),
			    .boundaryBackgroundColor = safe_strdup(style.boundaryBackgroundColor),
			    .boundaryBorderColor = safe_strdup(style.boundaryBorderColor)};

			dispatch_async(dispatch_get_main_queue(), ^{
				@autoreleasepool {
					[controller.overlayView.hints removeAllObjects];
					[controller.overlayView applyStyle:styleCopy];
					[controller.overlayView.hints addObjectsFromArray:hintItems];
					[controller.overlayView setNeedsDisplay:YES];
					NeruOrderOverlayWindowIfDrawable(controller, styleCopy.forceFlush ? YES : NO);

					free_hint_style_strings(&styleCopy);
				}
			});
		}
	}
}

static SearchInputItem *buildSearchInputItem(SearchInputData input) {
	SearchInputItem *item = [[SearchInputItem alloc] init];
	item.query = input.query ? @(input.query) : @"";
	item.resultCount = input.resultCount;
	item.position = input.position;
	item.width = input.width;
	return item;
}

static void applySearchInputStyle(OverlayView *view, SearchInputStyle style) {
	NSColor *defaultBackground = [[NSColor colorWithWhite:1.0 alpha:1.0] colorWithAlphaComponent:0.95];
	NSColor *defaultText = [NSColor blackColor];
	NSColor *defaultBorder = [NSColor blackColor];

	NSString *fontFamily = style.fontFamily ? @(style.fontFamily) : @"";
	CGFloat fontSize = style.fontSize > 0 ? style.fontSize : kDefaultHintFontSize;
	NSFont *font = nil;
	if ([fontFamily length] > 0) {
		font = [view resolveFont:fontFamily size:fontSize bold:NO];
	}
	if (!font) {
		font = [NSFont systemFontOfSize:fontSize];
	}

	view.searchInputFont = font;
	view.searchInputBackgroundColor =
	    [view colorFromHex:(style.backgroundColor ? @(style.backgroundColor) : nil) defaultColor:defaultBackground];
	view.searchInputTextColor =
	    [view colorFromHex:(style.textColor ? @(style.textColor) : nil) defaultColor:defaultText];
	view.searchInputBorderColor =
	    [view colorFromHex:(style.borderColor ? @(style.borderColor) : nil) defaultColor:defaultBorder];
	view.searchInputBorderRadius = style.borderRadius;
	view.searchInputBorderWidth = style.borderWidth;
	view.searchInputPaddingX = style.paddingX;
	view.searchInputPaddingY = style.paddingY;
}

void NeruDrawHintSearchInput(OverlayWindow window, SearchInputData input, SearchInputStyle style) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	SearchInputItem *item = buildSearchInputItem(input);
	SearchInputStyle styleCopy = {
	    .fontSize = style.fontSize,
	    .borderRadius = style.borderRadius,
	    .borderWidth = style.borderWidth,
	    .paddingX = style.paddingX,
	    .paddingY = style.paddingY,
	    .fontFamily = safe_strdup(style.fontFamily),
	    .backgroundColor = safe_strdup(style.backgroundColor),
	    .textColor = safe_strdup(style.textColor),
	    .borderColor = safe_strdup(style.borderColor)};

	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			controller.overlayView.searchInput = item;
			applySearchInputStyle(controller.overlayView, styleCopy);
			[controller.overlayView setNeedsDisplay:YES];
			free_search_input_style_strings(&styleCopy);
		}
	});
}

void NeruHideHintSearchInput(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			controller.overlayView.searchInput = nil;
			[controller.overlayView setNeedsDisplay:YES];
		}
	});
}

/// Update hint match prefix (incremental update for typing).
/// Only the hints whose matchedPrefixLength changed redraw.
/// @param window Overlay window handle
/// @param prefix Match prefix
void NeruUpdateHintMatchPrefix(OverlayWindow window, const char *prefix) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
	NSString *prefixStr = prefix ? @(prefix) : @"";

	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			BOOL anyChanged = NO;
			NSUInteger prefixLen = [prefixStr length];

			for (HintItem *hintItem in controller.overlayView.hints) {
				NSString *label = hintItem.label ?: @"";
				int newMatchedPrefixLength = 0;
				if (prefixLen > 0 && [label hasPrefix:prefixStr]) {
					newMatchedPrefixLength = (int)prefixLen;
				}

				if (hintItem.matchedPrefixLength != newMatchedPrefixLength) {
					hintItem.matchedPrefixLength = newMatchedPrefixLength;
					anyChanged = YES;
				}
			}

			if (anyChanged) {
				[controller.overlayView setNeedsDisplay:YES];
			}
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
		hintItemsToAdd = buildHintItems(hintsToAdd, addCount, style.showArrow ? YES : NO, style.placement);
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
	NSString *boundaryBgHex = style.boundaryBackgroundColor ? @(style.boundaryBackgroundColor) : nil;
	NSString *boundaryBorderHex = style.boundaryBorderColor ? @(style.boundaryBorderColor) : nil;
	int borderRadius = style.borderRadius;
	int borderWidth = style.borderWidth;
	int paddingX = style.paddingX;
	int paddingY = style.paddingY;
	int boundaryHighlightEnabled = style.boundaryHighlightEnabled;
	int boundaryBorderWidth = style.boundaryBorderWidth;
	int boundaryBorderRadius = style.boundaryBorderRadius;

	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
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
			if (boundaryBgHex) {
				NSColor *defaultBoundaryBg = [[NSColor systemBlueColor] colorWithAlphaComponent:0.08];
				controller.overlayView.hintBoundaryBackgroundColor =
				    [controller.overlayView colorFromHex:boundaryBgHex defaultColor:defaultBoundaryBg];
			}
			if (boundaryBorderHex) {
				NSColor *defaultBoundaryBorder = [[NSColor systemBlueColor] colorWithAlphaComponent:0.45];
				controller.overlayView.hintBoundaryBorderColor =
				    [controller.overlayView colorFromHex:boundaryBorderHex defaultColor:defaultBoundaryBorder];
			}

			// Apply geometry properties
			controller.overlayView.hintBorderRadius = borderRadius;
			controller.overlayView.hintBorderWidth = borderWidth >= 0 ? borderWidth : 1.0;
			controller.overlayView.hintPaddingX = paddingX;
			controller.overlayView.hintPaddingY = paddingY;
			controller.overlayView.hintBoundaryHighlightEnabled = boundaryHighlightEnabled ? YES : NO;
			controller.overlayView.hintBoundaryBorderWidth = boundaryBorderWidth >= 0 ? boundaryBorderWidth : 1.0;
			controller.overlayView.hintBoundaryBorderRadius = boundaryBorderRadius >= 0 ? boundaryBorderRadius : 4.0;

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
		}
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
			oldController.shouldBeVisible = NO;
			[oldController.window close];
			CFRelease(*pwindow);  // Balance the CFBridgingRetain from NeruCreateOverlayWindow
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
	BOOL hideLabel = style.hideLabel != 0;
	BOOL drawSubKeyPreview = style.drawSubKeyPreview ? YES : NO;
	int subKeyGridCols = style.subKeyGridCols;
	int subKeyGridRows = style.subKeyGridRows;
	CGFloat subKeyFontSize = style.subKeyFontSize > 0 ? style.subKeyFontSize : 6.0;
	NSString *subKeyFontFamily = nil;
	if (style.subKeyFontFamily) {
		subKeyFontFamily = @(style.subKeyFontFamily);
		if (subKeyFontFamily.length == 0)
			subKeyFontFamily = nil;
	}
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
		@autoreleasepool {
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
			controller.overlayView.gridMatchedBackgroundColor =
			    [controller.overlayView colorFromHex:matchedBgHex defaultColor:[NSColor blueColor]];
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
			controller.overlayView.gridHideLabel = hideLabel;

			// Apply sub-key preview settings
			controller.overlayView.gridDrawSubKeyPreview = drawSubKeyPreview;
			controller.overlayView.gridSubKeyCols = subKeyGridCols;
			controller.overlayView.gridSubKeyRows = subKeyGridRows;
			controller.overlayView.gridSubKeyLabels = subKeyLabels;
			if (drawSubKeyPreview) {
				if (subKeyFontSize != controller.overlayView.cachedGridSubKeyFontSize ||
				    (subKeyFontFamily != controller.overlayView.cachedGridSubKeyFontFamily &&
				     ![subKeyFontFamily isEqualToString:controller.overlayView.cachedGridSubKeyFontFamily])) {
					NSFont *subFont = nil;
					if (subKeyFontFamily && [subKeyFontFamily length] > 0) {
						subFont = [controller.overlayView resolveFont:subKeyFontFamily size:subKeyFontSize bold:NO];
					}
					if (!subFont) {
						subFont = [NSFont systemFontOfSize:subKeyFontSize];
					}
					controller.overlayView.gridSubKeyFont = subFont;
					controller.overlayView.cachedGridSubKeyFontSize = subKeyFontSize;
					controller.overlayView.cachedGridSubKeyFontFamily = subKeyFontFamily;
				}
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
		}
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
	BOOL hideLabel = style.hideLabel != 0;
	CGFloat transitionFontSize = style.transitionFontSize > 0 ? style.transitionFontSize : fontSize;
	BOOL transitionHideLabel = style.transitionHideLabel != 0;
	BOOL transitionHideSubKeyPreview = style.transitionHideSubKeyPreview != 0;
	BOOL drawSubKeyPreview = style.drawSubKeyPreview ? YES : NO;
	int subKeyGridCols = style.subKeyGridCols;
	int subKeyGridRows = style.subKeyGridRows;
	CGFloat subKeyFontSize = style.subKeyFontSize > 0 ? style.subKeyFontSize : 6.0;
	CGFloat transitionSubKeyFontSize =
	    style.transitionSubKeyFontSize > 0 ? style.transitionSubKeyFontSize : subKeyFontSize;
	NSString *subKeyFontFamily = nil;
	if (style.subKeyFontFamily) {
		subKeyFontFamily = @(style.subKeyFontFamily);
		if (subKeyFontFamily.length == 0)
			subKeyFontFamily = nil;
	}
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
		@autoreleasepool {
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
			controller.overlayView.gridMatchedBackgroundColor =
			    [controller.overlayView colorFromHex:matchedBgHex defaultColor:[NSColor blueColor]];
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
			controller.overlayView.gridHideLabel = hideLabel;
			controller.overlayView.gridDrawSubKeyPreview = drawSubKeyPreview;
			controller.overlayView.gridSubKeyCols = subKeyGridCols;
			controller.overlayView.gridSubKeyRows = subKeyGridRows;
			controller.overlayView.gridSubKeyLabels = subKeyLabels;
			if (drawSubKeyPreview) {
				if (subKeyFontSize != controller.overlayView.cachedGridSubKeyFontSize ||
				    (subKeyFontFamily != controller.overlayView.cachedGridSubKeyFontFamily &&
				     ![subKeyFontFamily isEqualToString:controller.overlayView.cachedGridSubKeyFontFamily])) {
					NSFont *subFont = nil;
					if (subKeyFontFamily && [subKeyFontFamily length] > 0) {
						subFont = [controller.overlayView resolveFont:subKeyFontFamily size:subKeyFontSize bold:NO];
					}
					if (!subFont) {
						subFont = [NSFont systemFontOfSize:subKeyFontSize];
					}
					controller.overlayView.gridSubKeyFont = subFont;
					controller.overlayView.cachedGridSubKeyFontSize = subKeyFontSize;
					controller.overlayView.cachedGridSubKeyFontFamily = subKeyFontFamily;
				}
				controller.overlayView.gridSubKeyTextColor = [controller.overlayView colorFromHex:subKeyTextHex
				                                                                     defaultColor:[NSColor grayColor]];
			}

			// What the transition holds from its first frame to its last. The fonts
			// come from a cache by size. A depth's held size recurs on every visit.
			controller.overlayView.gridTransitionHideLabel = transitionHideLabel;
			controller.overlayView.gridTransitionFont =
			    transitionHideLabel
			        ? nil
			        : [controller.overlayView transitionFontForFamily:fontFamily size:transitionFontSize];
			controller.overlayView.gridTransitionHideSubKeyPreview = transitionHideSubKeyPreview;
			controller.overlayView.gridTransitionSubKeyFont =
			    (drawSubKeyPreview && !transitionHideSubKeyPreview)
			        ? [controller.overlayView transitionFontForFamily:subKeyFontFamily size:transitionSubKeyFontSize]
			        : nil;

			controller.overlayView.cachedGridTextColor = controller.overlayView.gridTextColor;
			controller.overlayView.cachedGridMatchedTextColor = controller.overlayView.gridMatchedTextColor;
			[controller.overlayView startGridTransitionToCells:cellItems duration:duration];
		}
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
		@autoreleasepool {
			OverlayView *view = controller.overlayView;
			NSUInteger cellCount = [view.gridCells count];
			if (cellCount == 0)
				return;

			NSUInteger prefixLen = [prefixStr length];
			BOOL anyChanged = NO;
			for (GridCellItem *cellItem in view.gridCells) {
				NSString *label = cellItem.label ?: @"";
				BOOL newIsMatched = (prefixLen > 0 && [label hasPrefix:prefixStr]);
				int newMatchedPrefixLength = newIsMatched ? (int)prefixLen : 0;
				if (cellItem.isMatched != newIsMatched || cellItem.matchedPrefixLength != newMatchedPrefixLength) {
					cellItem.isMatched = newIsMatched;
					cellItem.matchedPrefixLength = newMatchedPrefixLength;
					anyChanged = YES;
				}
			}

			if (anyChanged) {
				[view setNeedsDisplay:YES];
			}
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
			@autoreleasepool {
				[controller.window setLevel:level];
			}
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
		@autoreleasepool {
			controller.overlayView.hideUnmatched = hide ? YES : NO;
			[controller.overlayView setNeedsDisplay:YES];
		}
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
		@autoreleasepool {
			controller.sharingType = sharingType;
			[controller.window setSharingType:sharingType];
		}
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
		@autoreleasepool {
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
				controller.overlayView.gridLabelBackgroundColor =
				    [controller.overlayView colorFromHex:labelBgHex
				                            defaultColor:[[NSColor colorWithRed:1.0 green:0.84 blue:0.0
				                                                          alpha:1.0] colorWithAlphaComponent:0.8]];
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
		}
	});
}

/// Show a virtual cursor indicator at the specified point.
/// Draws the configured character using the configured font and text color.
/// @param window Overlay window handle
/// @param position Indicator center position in overlay coordinates
/// @param style Indicator style
void NeruShowCursorIndicator(OverlayWindow window, CGPoint position, CursorIndicatorStyle style) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	NSString *labelChar = style.labelChar ? @(style.labelChar) : @"\u25CF";
	NSString *fontFamily = style.fontFamily ? @(style.fontFamily) : nil;
	NSString *textHex = style.textColor ? @(style.textColor) : nil;

	CGFloat fontSize = style.fontSize > 0 ? (CGFloat)style.fontSize : 8.0;

	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
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
			controller.overlayView.cursorIndicatorLabel = labelChar;

			NSFont *font = nil;
			if (fontFamily.length > 0) {
				font = [controller.overlayView resolveFont:fontFamily size:fontSize bold:NO];
			}
			if (!font) {
				font = [NSFont systemFontOfSize:fontSize];
			}
			controller.overlayView.cursorIndicatorFont = font;
			controller.overlayView.cursorIndicatorTextColor =
			    [controller.overlayView colorFromHex:textHex defaultColor:[NSColor whiteColor]];

			[controller.overlayView setNeedsDisplay:YES];
		}
	});
}

static NSPoint NeruAppKitPointFromQuartzPoint(CGPoint point);
static NSScreen *NeruScreenContainingQuartzPoint(CGPoint point);

/// Position a small overlay window on the cursor and draw the virtual pointer indicator.
/// Draws the configured character using the configured font and text color.
void NeruPositionAndDrawVirtualPointer(
    OverlayWindow window, double absoluteX, double absoluteY, CursorIndicatorStyle style) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	NSString *labelChar = style.labelChar ? @(style.labelChar) : @"\u25CF";
	NSString *fontFamily = style.fontFamily ? @(style.fontFamily) : nil;
	NSString *textHex = style.textColor ? @(style.textColor) : nil;
	CGFloat margin = 2.0;

	CGFloat fontSize = style.fontSize > 0 ? (CGFloat)style.fontSize : 8.0;

	// Measure the actual glyph size so the window accommodates wide/tall characters
	// (e.g. "⬤", "◆") that can exceed fontSize.
	// Use the same resolution strategy as resolveFont:size:bold: (without bold trait)
	// to keep measurement consistent with the rendering font set inside the dispatch block.
	NSFont *measureFont = nil;
	if (fontFamily.length > 0) {
		measureFont = [NSFont fontWithName:fontFamily size:fontSize];
		if (!measureFont) {
			NSFontManager *fm = [NSFontManager sharedFontManager];
			measureFont = [fm fontWithFamily:fontFamily traits:0 weight:5 size:fontSize];
		}
	}
	if (!measureFont) {
		measureFont = [NSFont systemFontOfSize:fontSize];
	}
	NSSize labelSize = [labelChar sizeWithAttributes:@{NSFontAttributeName : measureFont}];
	CGFloat textDimension = MAX(labelSize.width, labelSize.height);
	CGFloat windowSize = textDimension + margin * 2.0;

	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			CGPoint desiredCenter = CGPointMake(absoluteX, absoluteY);
			NSPoint appKitCenter = NeruAppKitPointFromQuartzPoint(desiredCenter);

			NSScreen *cursorScreen = NeruScreenContainingQuartzPoint(desiredCenter);
			if (cursorScreen != nil) {
				NSRect screenFrame = cursorScreen.frame;
				CGFloat half = windowSize / 2.0;
				CGFloat minX = screenFrame.origin.x + half;
				CGFloat maxX = NSMaxX(screenFrame) - half;
				CGFloat minY = screenFrame.origin.y + half;
				CGFloat maxY = NSMaxY(screenFrame) - half;
				if (minX <= maxX) {
					appKitCenter.x = MAX(minX, MIN(appKitCenter.x, maxX));
				}
				if (minY <= maxY) {
					appKitCenter.y = MAX(minY, MIN(appKitCenter.y, maxY));
				}
			}

			NSRect frame = NSMakeRect(
			    appKitCenter.x - windowSize / 2.0, appKitCenter.y - windowSize / 2.0, windowSize, windowSize);

			[controller.window setFrame:frame display:NO];
			[controller.overlayView setFrame:NSMakeRect(0, 0, windowSize, windowSize)];
			NeruOrderOverlayWindowIfDrawable(controller, NO);

			[controller.overlayView cancelCursorIndicatorTransition];
			controller.overlayView.cursorIndicatorVisible = YES;
			controller.overlayView.cursorIndicatorPosition = NSMakePoint(windowSize / 2.0, windowSize / 2.0);
			controller.overlayView.cursorIndicatorLabel = labelChar;

			NSFont *font = nil;
			if (fontFamily.length > 0) {
				font = [controller.overlayView resolveFont:fontFamily size:fontSize bold:NO];
			}
			if (!font) {
				font = [NSFont systemFontOfSize:fontSize];
			}
			controller.overlayView.cursorIndicatorFont = font;
			controller.overlayView.cursorIndicatorTextColor =
			    [controller.overlayView colorFromHex:textHex defaultColor:[NSColor whiteColor]];

			[controller.overlayView setNeedsDisplay:YES];
		}
	});
}

/// Hide the virtual cursor indicator.
/// @param window Overlay window handle
void NeruHideCursorIndicator(OverlayWindow window) {
	if (!window)
		return;

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			if (!controller.overlayView.cursorIndicatorVisible)
				return;

			[controller.overlayView cancelCursorIndicatorTransition];
			controller.overlayView.cursorIndicatorVisible = NO;
			[controller.overlayView setNeedsDisplay:YES];
		}
	});
}

static NSColor *NeruColorFromHexString(NSString *hexString, NSColor *defaultColor) {
	if (!hexString || hexString.length == 0)
		return defaultColor;

	NSString *hex =
	    [hexString stringByTrimmingCharactersInSet:[NSCharacterSet characterSetWithCharactersInString:@"#"]];
	unsigned int alpha = 255;
	unsigned int red = 255;
	unsigned int green = 255;
	unsigned int blue = 255;

	if (hex.length == 8) {
		[[NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(0, 2)]] scanHexInt:&alpha];
		[[NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(2, 2)]] scanHexInt:&red];
		[[NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(4, 2)]] scanHexInt:&green];
		[[NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(6, 2)]] scanHexInt:&blue];
	} else if (hex.length == 6) {
		[[NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(0, 2)]] scanHexInt:&red];
		[[NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(2, 2)]] scanHexInt:&green];
		[[NSScanner scannerWithString:[hex substringWithRange:NSMakeRange(4, 2)]] scanHexInt:&blue];
	} else if (hex.length == 3) {
		unichar r = [hex characterAtIndex:0];
		unichar g = [hex characterAtIndex:1];
		unichar b = [hex characterAtIndex:2];
		NSString *expanded = [NSString stringWithFormat:@"%C%C%C%C%C%C", r, r, g, g, b, b];
		return NeruColorFromHexString(expanded, defaultColor);
	} else {
		return defaultColor;
	}

	return [NSColor colorWithRed:(CGFloat)red / 255.0
	                       green:(CGFloat)green / 255.0
	                        blue:(CGFloat)blue / 255.0
	                       alpha:(CGFloat)alpha / 255.0];
}

static CAMediaTimingFunction *NeruTimingFunction(NSString *easing) {
	if ([easing isEqualToString:@"linear"])
		return [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionLinear];
	if ([easing isEqualToString:@"ease_in"])
		return [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseIn];
	if ([easing isEqualToString:@"ease_in_out"])
		return [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseInEaseOut];

	return [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseOut];
}

static NSPoint NeruAppKitPointFromQuartzPoint(CGPoint point) {
	CGDirectDisplayID displayID = 0;
	uint32_t displayCount = 0;
	CGGetDisplaysWithPoint(point, 1, &displayID, &displayCount);

	if (displayCount == 0) {
		NSRect mainFrame = [NSScreen mainScreen].frame;
		return NSMakePoint(point.x, NSMaxY(mainFrame) - point.y);
	}

	CGRect displayBounds = CGDisplayBounds(displayID);
	for (NSScreen *screen in [NSScreen screens]) {
		NSNumber *screenNumber = screen.deviceDescription[@"NSScreenNumber"];
		if (screenNumber.unsignedIntValue != displayID)
			continue;

		CGFloat localX = point.x - displayBounds.origin.x;
		CGFloat localY = point.y - displayBounds.origin.y;
		return NSMakePoint(screen.frame.origin.x + localX, NSMaxY(screen.frame) - localY);
	}

	NSRect mainFrame = [NSScreen mainScreen].frame;
	return NSMakePoint(point.x, NSMaxY(mainFrame) - point.y);
}

/// Resolve the NSScreen that contains the given Quartz point.
/// Falls back to mainScreen, then the first available screen.
/// Returns nil only if no screens are attached.
static NSScreen *NeruScreenContainingQuartzPoint(CGPoint point) {
	CGDirectDisplayID displayID = 0;
	uint32_t displayCount = 0;
	CGGetDisplaysWithPoint(point, 1, &displayID, &displayCount);

	if (displayCount > 0) {
		for (NSScreen *screen in [NSScreen screens]) {
			NSNumber *screenNumber = screen.deviceDescription[@"NSScreenNumber"];
			if (screenNumber.unsignedIntValue == displayID)
				return screen;
		}
	}

	return [NSScreen mainScreen] ?: [NSScreen.screens firstObject];
}

/// Position and resize overlay window to a specific rect centered on a point.
/// Converts absolute Quartz coordinates to AppKit (bottom-left origin) and
/// clamps the resulting frame to the screen containing the desired center so
/// the entire window stays within a single display.
///
/// Clamping is critical: without it, a window straddling two displays causes
/// AppKit to oscillate the window's `screen` (and therefore the layer's
/// contentsScale) as the cursor moves near the boundary. That oscillation
/// triggers `viewDidChangeBackingProperties` redraws at different scales and
/// is perceived as a flicker on multi-monitor setups.
///

void NeruPositionAndSizeOverlayToFitHint(
    OverlayWindow window, double absoluteX, double absoluteY, const char *label, HintStyle style, double *outWidth,
    double *outHeight) {
	if (!window || !label) {
		if (outWidth)
			*outWidth = 0;
		if (outHeight)
			*outHeight = 0;

		return;
	}

	OverlayWindowController *controller = (__bridge OverlayWindowController *)window;

	void (^positionBlock)(void) = ^{
		@autoreleasepool {
			NSString *fontFamily = style.fontFamily ? @(style.fontFamily) : @"";
			CGFloat fontSize = style.fontSize > 0 ? style.fontSize : kDefaultHintFontSize;
			NSFont *font = nil;
			if ([fontFamily length] > 0) {
				font = [controller.overlayView resolveFont:fontFamily size:fontSize bold:YES];
			}
			if (!font) {
				font = [NSFont boldSystemFontOfSize:fontSize];
			}

			NSMutableAttributedString *attrString = [[NSMutableAttributedString alloc] initWithString:@(label)];
			NSRange fullRange = NSMakeRange(0, attrString.length);
			[attrString setAttributes:@{NSFontAttributeName : font} range:fullRange];
			NSSize textSize = [attrString size];

			CGFloat paddingX = style.paddingX >= 0.0 ? style.paddingX : MAX(4.0, round(fontSize * 0.4));
			CGFloat paddingY = style.paddingY >= 0.0 ? style.paddingY : MAX(2.0, round(fontSize * 0.2));
			CGFloat borderWidth = style.borderWidth >= 0 ? style.borderWidth : 1.0;

			CGFloat contentWidth = textSize.width + (paddingX * 2);
			CGFloat contentHeight = textSize.height + (paddingY * 2);
			CGFloat boxWidth = MAX(contentWidth, contentHeight);
			CGFloat boxHeight = contentHeight;

			// Add small margin for border anti-aliasing
			CGFloat windowWidth = boxWidth + borderWidth * 2.0 + 4.0;
			CGFloat windowHeight = boxHeight + borderWidth * 2.0 + 4.0;

			CGPoint desiredCenter = CGPointMake(absoluteX, absoluteY);
			NSPoint appKitCenter = NeruAppKitPointFromQuartzPoint(desiredCenter);

			NSScreen *cursorScreen = NeruScreenContainingQuartzPoint(desiredCenter);
			if (cursorScreen != nil) {
				NSRect screenFrame = cursorScreen.frame;
				CGFloat halfW = windowWidth / 2.0;
				CGFloat halfH = windowHeight / 2.0;
				CGFloat minX = screenFrame.origin.x + halfW;
				CGFloat maxX = NSMaxX(screenFrame) - halfW;
				CGFloat minY = screenFrame.origin.y + halfH;
				CGFloat maxY = NSMaxY(screenFrame) - halfH;
				if (minX <= maxX) {
					appKitCenter.x = MAX(minX, MIN(appKitCenter.x, maxX));
				}
				if (minY <= maxY) {
					appKitCenter.y = MAX(minY, MIN(appKitCenter.y, maxY));
				}
			}

			NSRect frame = NSMakeRect(
			    appKitCenter.x - windowWidth / 2.0, appKitCenter.y - windowHeight / 2.0, windowWidth, windowHeight);

			[controller.window setFrame:frame display:NO];
			NSRect viewFrame = NSMakeRect(0, 0, windowWidth, windowHeight);
			[controller.overlayView setFrame:viewFrame];
			NeruOrderOverlayWindowIfDrawable(controller, NO);

			if (outWidth)
				*outWidth = (double)windowWidth;
			if (outHeight)
				*outHeight = (double)windowHeight;
		}
	};

	if ([NSThread isMainThread]) {
		positionBlock();
	} else {
		dispatch_sync(dispatch_get_main_queue(), positionBlock);
	}
}

/// Show a transient mouse action indicator in its own overlay window.
/// @param position Global cursor position in Quartz coordinates
/// @param style Indicator style
static _Atomic int _NeruMouseActionPanelCount = 0;
static const int _NeruMaxMouseActionPanels = 10;
static NSMutableSet *_NeruMouseActionPanels;
static dispatch_once_t _NeruMouseActionPanelsOnceToken;

void NeruShowMouseActionIndicator(CGPoint position, MouseActionIndicatorStyle style) {
	NSString *backgroundHex = style.backgroundColor ? @(style.backgroundColor) : nil;
	NSString *borderHex = style.borderColor ? @(style.borderColor) : nil;
	NSString *shape = style.shape ? @(style.shape) : @"circle";
	NSString *easing = style.easing ? @(style.easing) : @"ease_out";

	dispatch_once(&_NeruMouseActionPanelsOnceToken, ^{
		_NeruMouseActionPanels = [NSMutableSet set];
	});

	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			if (atomic_load(&_NeruMouseActionPanelCount) >= _NeruMaxMouseActionPanels) {
				return;
			}
			CGFloat size = MAX(style.size, 1);
			CGFloat endScale = style.endScale > 0 ? style.endScale : 1.0;
			CGFloat maxScale = MAX(style.startScale > 0 ? style.startScale : 1.0, MAX(endScale, 1.0));
			CGFloat canvasSize = ceil(size * maxScale + MAX(style.borderWidth, 0) * 4.0);
			NSPoint center = NeruAppKitPointFromQuartzPoint(position);
			NSRect frame = NSMakeRect(center.x - canvasSize / 2.0, center.y - canvasSize / 2.0, canvasSize, canvasSize);

			NSPanel *panel =
			    [[NSPanel alloc] initWithContentRect:frame
			                               styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
			                                 backing:NSBackingStoreBuffered
			                                   defer:NO];
			[panel setHidesOnDeactivate:NO];
			[panel setReleasedWhenClosed:NO];
			[panel setTitle:@"neru-overlay"];
			[panel setLevel:kCGMaximumWindowLevel];
			[panel setOpaque:NO];
			[panel setBackgroundColor:[NSColor clearColor]];
			[panel setIgnoresMouseEvents:YES];
			[panel setAcceptsMouseMovedEvents:NO];
			[panel setHasShadow:NO];
			[panel setSharingType:style.hideInScreenShare ? NSWindowSharingNone : NSWindowSharingReadOnly];
			[panel setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces |
			                             NSWindowCollectionBehaviorStationary |
			                             NSWindowCollectionBehaviorFullScreenAuxiliary |
			                             NSWindowCollectionBehaviorIgnoresCycle];
			atomic_fetch_add(&_NeruMouseActionPanelCount, 1);
			[_NeruMouseActionPanels addObject:panel];

			NSView *view = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, canvasSize, canvasSize)];
			view.wantsLayer = YES;
			view.layer.backgroundColor = NSColor.clearColor.CGColor;
			[panel setContentView:view];

			CGRect indicatorRect = CGRectMake((canvasSize - size) / 2.0, (canvasSize - size) / 2.0, size, size);
			CGFloat cornerRadius = [shape isEqualToString:@"square"] ? MAX(size * 0.18, 2.0) : size / 2.0;

			CAShapeLayer *layer = [CAShapeLayer layer];
			layer.frame = view.bounds;
			CGPathRef path = CGPathCreateWithRoundedRect(indicatorRect, cornerRadius, cornerRadius, NULL);
			layer.path = path;
			CGPathRelease(path);
			layer.fillColor = NeruColorFromHexString(backgroundHex, [NSColor clearColor]).CGColor;
			layer.strokeColor = NeruColorFromHexString(borderHex, [NSColor whiteColor]).CGColor;
			layer.lineWidth = MAX(style.borderWidth, 0);
			layer.opacity = style.startOpacity;
			[view.layer addSublayer:layer];

			[panel orderFrontRegardless];

			CFTimeInterval duration = MAX(style.durationMS, 1) / 1000.0;
			CAMediaTimingFunction *timing = NeruTimingFunction(easing);

			CABasicAnimation *scaleAnimation = [CABasicAnimation animationWithKeyPath:@"transform.scale"];
			scaleAnimation.fromValue = @(style.startScale);
			scaleAnimation.toValue = @(style.endScale);
			scaleAnimation.duration = duration;
			scaleAnimation.timingFunction = timing;

			CABasicAnimation *opacityAnimation = [CABasicAnimation animationWithKeyPath:@"opacity"];
			opacityAnimation.fromValue = @(style.startOpacity);
			opacityAnimation.toValue = @(style.endOpacity);
			opacityAnimation.duration = duration;
			opacityAnimation.timingFunction = timing;

			layer.transform = CATransform3DMakeScale(style.endScale, style.endScale, 1.0);
			layer.opacity = style.endOpacity;
			[layer addAnimation:scaleAnimation forKey:@"mouseActionScale"];
			[layer addAnimation:opacityAnimation forKey:@"mouseActionOpacity"];

			__weak NSPanel *weakPanel = panel;
			dispatch_after(
			    dispatch_time(DISPATCH_TIME_NOW, (int64_t)(duration * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
				    @autoreleasepool {
					    NSPanel *strongPanel = weakPanel;
					    if (!strongPanel)
						    return;
					    [strongPanel.contentView.layer removeAllAnimations];
					    [strongPanel setContentView:nil];
					    [strongPanel orderOut:nil];
					    [strongPanel close];
					    [_NeruMouseActionPanels removeObject:strongPanel];
					    atomic_fetch_add(&_NeruMouseActionPanelCount, -1);
				    }
			    });
		}
	});
}

#pragma mark - System Cursor

/// Private CGS API to allow cursor changes from background processes.
typedef int CGSConnectionID;
CGError CGSSetConnectionProperty(CGSConnectionID cid, CGSConnectionID targetCID, CFStringRef key, CFTypeRef value);
CGSConnectionID _CGSDefaultConnection(void);

static void _NeruEnableCursorInBackground(void) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		CGSConnectionID cid = _CGSDefaultConnection();
		CFStringRef key = CFSTR("SetsCursorInBackground");
		CGSSetConnectionProperty(cid, cid, key, kCFBooleanTrue);
	});
}

void NeruHideSystemCursor(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		_NeruEnableCursorInBackground();
		CGDisplayHideCursor(kCGNullDirectDisplay);
	});
}

void NeruShowSystemCursor(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		CGDisplayShowCursor(kCGNullDirectDisplay);
	});
}

void NeruRehideSystemCursor(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		_NeruEnableCursorInBackground();
		CGDisplayShowCursor(kCGNullDirectDisplay);
		CGDisplayHideCursor(kCGNullDirectDisplay);
	});
}
