//
//  monitor_select_overlay.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "overlay.h"

#import <Cocoa/Cocoa.h>
#import <CoreText/CoreText.h>
#import <QuartzCore/QuartzCore.h>

#pragma mark - Global State

static NSMutableArray *_NeruMonitorSelectPanels = nil;

#pragma mark - Helper Functions

static NSColor *monitorSelectColorFromHex(NSString *hexString, NSColor *defaultColor) {
	if (!hexString || hexString.length == 0)
		return defaultColor;

	NSString *clean = [hexString stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
	if ([clean hasPrefix:@"#"])
		clean = [clean substringFromIndex:1];
	clean = [clean lowercaseString];

	if (clean.length == 3) {
		clean = [NSString stringWithFormat:@"%c%c%c%c%c%c", [clean characterAtIndex:0], [clean characterAtIndex:0],
		                                   [clean characterAtIndex:1], [clean characterAtIndex:1],
		                                   [clean characterAtIndex:2], [clean characterAtIndex:2]];
	}

	if (clean.length != 6 && clean.length != 8)
		return defaultColor;

	unsigned long long hexValue = 0;
	NSScanner *scanner = [NSScanner scannerWithString:clean];
	if (![scanner scanHexLongLong:&hexValue])
		return defaultColor;

	CGFloat alpha = 1.0;
	if (clean.length == 8)
		alpha = ((hexValue & 0xFF000000) >> 24) / 255.0;
	CGFloat red = ((hexValue & 0x00FF0000) >> 16) / 255.0;
	CGFloat green = ((hexValue & 0x0000FF00) >> 8) / 255.0;
	CGFloat blue = (hexValue & 0x000000FF) / 255.0;

	return [NSColor colorWithRed:red green:green blue:blue alpha:alpha];
}

static NSFont *monitorSelectResolveFont(NSString *name, CGFloat size, BOOL bold) {
	if (!name || name.length == 0)
		return nil;

	NSFontManager *fm = [NSFontManager sharedFontManager];
	NSFont *font = [NSFont fontWithName:name size:size];
	if (!font) {
		NSFontTraitMask traits = bold ? NSBoldFontMask : 0;
		NSInteger weight = bold ? 9 : 5;
		font = [fm fontWithFamily:name traits:traits weight:weight size:size];
	}

	if (font && bold) {
		NSFontTraitMask actualTraits = [fm traitsOfFont:font];
		if (!(actualTraits & NSBoldFontMask)) {
			NSFont *boldFont = [fm convertFont:font toHaveTrait:NSBoldFontMask];
			NSFontTraitMask boldTraits = [fm traitsOfFont:boldFont];
			if (boldTraits & NSBoldFontMask) {
				font = boldFont;
			}
		}
	}

	return font;
}

#pragma mark - C Interface Implementation

void NeruShowMonitorSelectPanels(MonitorSelectTargetData *targets, int count, MonitorSelectStyle style) {
	if (_NeruMonitorSelectPanels) {
		dispatch_sync(dispatch_get_main_queue(), ^{
			@autoreleasepool {
				for (NSPanel *existing in _NeruMonitorSelectPanels) {
					[existing setContentView:nil];
					[existing orderOut:nil];
					[existing close];
				}
				[_NeruMonitorSelectPanels removeAllObjects];
			}
		});
	}

	if (!targets || count == 0)
		return;

	if (!_NeruMonitorSelectPanels)
		_NeruMonitorSelectPanels = [[NSMutableArray alloc] initWithCapacity:count];

	dispatch_sync(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			for (int i = 0; i < count; i++) {
				MonitorSelectTargetData target = targets[i];
				// Convert from CG coordinates (top-left origin) to NSScreen (bottom-left origin)
				NSScreen *primaryScreen = [[NSScreen screens] firstObject];
				CGFloat screenHeight = primaryScreen.frame.size.height;
				CGFloat nsY = screenHeight - (target.y + target.height);
				NSRect screenFrame = NSMakeRect(target.x, nsY, target.width, target.height);

				NSPanel *panel = [[NSPanel alloc]
				    initWithContentRect:screenFrame
				              styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
				                backing:NSBackingStoreBuffered
				                  defer:NO];
				panel.hidesOnDeactivate = NO;
				panel.releasedWhenClosed = NO;
				panel.animationBehavior = NSWindowAnimationBehaviorNone;
				panel.animations = @{};
				panel.alphaValue = 1.0;
				panel.level = NSScreenSaverWindowLevel;
				panel.opaque = NO;
				panel.backgroundColor = NSColor.clearColor;
				panel.ignoresMouseEvents = YES;
				panel.acceptsMouseMovedEvents = NO;
				panel.hasShadow = NO;
				panel.collectionBehavior =
				    NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary |
				    NSWindowCollectionBehaviorIgnoresCycle | NSWindowCollectionBehaviorFullScreenAuxiliary;
				panel.sharingType = style.hideInScreenShare ? NSWindowSharingNone : NSWindowSharingReadOnly;

				NSRect viewFrame = NSMakeRect(0, 0, screenFrame.size.width, screenFrame.size.height);
				NSView *contentView = [[NSView alloc] initWithFrame:viewFrame];
				contentView.wantsLayer = YES;
				contentView.layer.opaque = NO;
				panel.contentView = contentView;

				// Backdrop color
				NSString *backdropHex = style.backdropColor ? @(style.backdropColor) : nil;
				if (backdropHex.length > 0) {
					contentView.layer.backgroundColor =
					    [monitorSelectColorFromHex(backdropHex, NSColor.clearColor) CGColor];
				}

				NSString *label = target.label ? @(target.label) : @"";
				NSString *subtitle = target.subtitle ? @(target.subtitle) : @"";
				BOOL isCurrent = target.isCurrent ? YES : NO;

				// Colors
				NSString *bgHex = isCurrent ? (style.currentBackgroundColor ? @(style.currentBackgroundColor) : nil)
				                            : (style.backgroundColor ? @(style.backgroundColor) : nil);
				NSString *textHex = isCurrent ? (style.currentTextColor ? @(style.currentTextColor) : nil)
				                              : (style.textColor ? @(style.textColor) : nil);
				NSString *borderHex = isCurrent ? (style.currentBorderColor ? @(style.currentBorderColor) : nil)
				                                : (style.borderColor ? @(style.borderColor) : nil);

				NSColor *bgColor = monitorSelectColorFromHex(bgHex, [NSColor colorWithWhite:0.95 alpha:0.95]);
				NSColor *textColor = monitorSelectColorFromHex(textHex, [NSColor blackColor]);
				NSColor *borderColor = monitorSelectColorFromHex(borderHex, [NSColor colorWithWhite:0.5 alpha:0.5]);

				// Fonts
				CGFloat fontSize = style.fontSize > 0 ? style.fontSize : 96;
				NSString *fontFamily = style.fontFamily ? @(style.fontFamily) : @"";
				NSFont *labelFont = fontFamily.length > 0 ? monitorSelectResolveFont(fontFamily, fontSize, YES) : nil;
				if (!labelFont)
					labelFont = [NSFont boldSystemFontOfSize:fontSize];

				CGFloat subFontSize = style.subtitleFontSize > 0 ? style.subtitleFontSize : 18;
				NSString *subFamily = style.subtitleFontFamily ? @(style.subtitleFontFamily) : @"";
				NSFont *subtitleFont =
				    subFamily.length > 0 ? monitorSelectResolveFont(subFamily, subFontSize, NO) : nil;
				if (!subtitleFont)
					subtitleFont = [NSFont systemFontOfSize:subFontSize];

				NSDictionary *labelAttrs = @{NSFontAttributeName : labelFont};
				NSSize labelSize = [label sizeWithAttributes:labelAttrs];

				NSDictionary *subAttrs = @{NSFontAttributeName : subtitleFont};
				NSSize subtitleSize = NSZeroSize;
				if (subtitle.length > 0)
					subtitleSize = [subtitle sizeWithAttributes:subAttrs];

				CGFloat hPadding = style.paddingX >= 0 ? style.paddingX : MAX(24, round(fontSize * 0.3));
				CGFloat vPadding = style.paddingY >= 0 ? style.paddingY : MAX(12, round(fontSize * 0.15));

				CGFloat contentWidth = MAX(labelSize.width, subtitleSize.width);
				CGFloat badgeWidth = contentWidth + hPadding * 2;
				CGFloat badgeHeight = labelSize.height + vPadding * 2;
				if (subtitle.length > 0)
					badgeHeight += subtitleSize.height + 4;
				CGFloat maxW = viewFrame.size.width * 0.8;
				CGFloat maxH = viewFrame.size.height * 0.8;
				badgeWidth = MIN(badgeWidth, maxW);
				badgeHeight = MIN(badgeHeight, maxH);

				CGFloat badgeX = (viewFrame.size.width - badgeWidth) / 2;
				CGFloat badgeY = (viewFrame.size.height - badgeHeight) / 2;
				CGFloat radius = style.borderRadius >= 0 ? style.borderRadius : MIN(badgeHeight / 2, 16);

				// Text position
				CGFloat textX = badgeX + (badgeWidth - labelSize.width) / 2;
				CGFloat textY = badgeY + (badgeHeight - labelSize.height) / 2;
				if (subtitle.length > 0) {
					CGFloat totalTextH = labelSize.height + 4 + subtitleSize.height;
					textY = badgeY + (badgeHeight - totalTextH) / 2;
				}

				/// --- Build sublayers ---

				// Badge background (CAShapeLayer for rounded rect)
				CAShapeLayer *bgLayer = [CAShapeLayer layer];
				CGRect badgeRect = CGRectMake(badgeX, badgeY, badgeWidth, badgeHeight);
				bgLayer.frame = contentView.layer.bounds;
				bgLayer.fillColor = bgColor.CGColor;
				if (style.borderWidth > 0) {
					bgLayer.strokeColor = borderColor.CGColor;
					bgLayer.lineWidth = style.borderWidth;
				}
				// Build rounded rect CGPath
				CGMutablePathRef bgPath = CGPathCreateMutable();
				CGPathAddRoundedRect(bgPath, NULL, badgeRect, radius, radius);
				bgLayer.path = bgPath;
				CGPathRelease(bgPath);
				[contentView.layer addSublayer:bgLayer];

				// Label text layer
				CATextLayer *textLayer = [CATextLayer layer];
				textLayer.string = label;
				textLayer.font = (__bridge CTFontRef)labelFont;
				textLayer.fontSize = fontSize;
				textLayer.foregroundColor = textColor.CGColor;
				textLayer.contentsScale = contentView.layer.contentsScale;
				textLayer.frame = CGRectMake(textX, textY, labelSize.width, labelSize.height);
				[contentView.layer addSublayer:textLayer];

				// Subtitle text layer
				if (subtitle.length > 0) {
					NSString *subHex = style.subtitleTextColor ? @(style.subtitleTextColor) : nil;
					NSColor *subColor = monitorSelectColorFromHex(subHex, [NSColor colorWithWhite:0.3 alpha:0.8]);
					CGFloat subX = badgeX + (badgeWidth - subtitleSize.width) / 2;
					CGFloat subY = textY + labelSize.height + 4;

					CATextLayer *subLayer = [CATextLayer layer];
					subLayer.string = subtitle;
					subLayer.font = (__bridge CTFontRef)subtitleFont;
					subLayer.fontSize = subFontSize;
					subLayer.foregroundColor = subColor.CGColor;
					subLayer.contentsScale = contentView.layer.contentsScale;
					subLayer.frame = CGRectMake(subX, subY, subtitleSize.width, subtitleSize.height);
					[contentView.layer addSublayer:subLayer];
				}

				[panel orderFrontRegardless];
				[_NeruMonitorSelectPanels addObject:panel];
			}
		}
	});
}

void NeruHideMonitorSelectPanels(void) {
	if (!_NeruMonitorSelectPanels)
		return;

	dispatch_sync(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			for (NSPanel *panel in _NeruMonitorSelectPanels) {
				panel.contentView = nil;
				[panel orderOut:nil];
				[panel close];
			}
			[_NeruMonitorSelectPanels removeAllObjects];
		}
	});
}
