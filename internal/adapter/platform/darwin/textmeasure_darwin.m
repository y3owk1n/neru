//
//  textmeasure_darwin.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "textmeasure.h"

#import <CoreText/CoreText.h>

#pragma mark - Font Lookup

/// Whether a CoreText font is the one that was asked for. CTFontCreateWithName
/// never fails, it substitutes, so the answer has to be checked against the
/// request: its PostScript name, its full name or its family.
static BOOL fontAnswersToName(CTFontRef font, NSString *name) {
	NSArray<NSString *> *names = @[
		CFBridgingRelease(CTFontCopyPostScriptName(font)) ?: @"",
		CFBridgingRelease(CTFontCopyFullName(font)) ?: @"",
		CFBridgingRelease(CTFontCopyFamilyName(font)) ?: @"",
	];
	for (NSString *candidate in names) {
		if ([candidate caseInsensitiveCompare:name] == NSOrderedSame)
			return YES;
	}
	return NO;
}

/// The font a name resolves to, +1 retained, or NULL when nothing answers to
/// it. Mirrors resolveFont:size:bold: in overlay_darwin.m.
static CTFontRef createNamedFont(NSString *name, CGFloat size) {
	if (name.length == 0)
		return NULL;

	CTFontRef font = CTFontCreateWithName((__bridge CFStringRef)name, size, NULL);
	if (font && fontAnswersToName(font, name))
		return font;
	if (font)
		CFRelease(font);

	NSDictionary *attributes = @{(__bridge NSString *)kCTFontFamilyNameAttribute : name};
	CTFontDescriptorRef descriptor = CTFontDescriptorCreateWithAttributes((__bridge CFDictionaryRef)attributes);
	if (!descriptor)
		return NULL;

	font = CTFontCreateWithFontDescriptor(descriptor, size, NULL);
	CFRelease(descriptor);
	if (font && fontAnswersToName(font, name))
		return font;
	if (font)
		CFRelease(font);

	return NULL;
}

#pragma mark - Text Measurement Functions

int NeruMeasureText(const char *text, const char *family, double size, int bold, double *outWidth, double *outHeight) {
	if (!text || !outWidth || !outHeight || size <= 0)
		return 0;

	@autoreleasepool {
		NSString *string = [NSString stringWithUTF8String:text];
		if (!string)
			return 0;

		NSString *name = family ? [NSString stringWithUTF8String:family] : nil;
		CTFontRef font = createNamedFont(name, (CGFloat)size);
		if (!font) {
			font = CTFontCreateUIFontForLanguage(
			    bold ? kCTFontUIFontEmphasizedSystem : kCTFontUIFontSystem, (CGFloat)size, NULL);
		}
		if (!font)
			return 0;

		if (bold && !(CTFontGetSymbolicTraits(font) & kCTFontTraitBold)) {
			CTFontRef boldFont =
			    CTFontCreateCopyWithSymbolicTraits(font, (CGFloat)size, NULL, kCTFontTraitBold, kCTFontTraitBold);
			if (boldFont) {
				CFRelease(font);
				font = boldFont;
			}
		}

		NSDictionary *attributes = @{(__bridge NSString *)kCTFontAttributeName : (__bridge id)font};
		NSAttributedString *attributed = [[NSAttributedString alloc] initWithString:string attributes:attributes];
		CTLineRef line = CTLineCreateWithAttributedString((__bridge CFAttributedStringRef)attributed);
		CFRelease(font);
		if (!line)
			return 0;

		CGFloat ascent = 0, descent = 0, leading = 0;
		double width = CTLineGetTypographicBounds(line, &ascent, &descent, &leading);
		CFRelease(line);

		*outWidth = width;
		*outHeight = (double)(ascent + descent + leading);
		return 1;
	}
}
