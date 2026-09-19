//
//  textmeasure.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef TEXTMEASURE_H
#define TEXTMEASURE_H

#import <Foundation/Foundation.h>

#pragma mark - Text Measurement Functions

/// Measure one line of text with CoreText.
///
/// Safe on any thread and never dispatches to the main queue. Styles are built
/// on the main thread before its run loop is servicing, where a dispatch_sync
/// would deadlock, and on background goroutines afterwards.
///
/// The font is looked up the way OverlayView's resolveFont:size:bold: looks it
/// up (a PostScript or full name first, then the family, then the system font)
/// so the font measured is the font drawn.
/// @param text UTF-8 text to measure
/// @param family Font name (PostScript or family), or empty for the system font
/// @param size Font size in points
/// @param bold Whether to measure the bold variant
/// @param outWidth Receives the typographic width of the line
/// @param outHeight Receives the line height (ascent + descent + leading)
/// @return 1 on success, 0 when the text could not be measured
int NeruMeasureText(const char *text, const char *family, double size, int bold, double *outWidth, double *outHeight);

#endif /* TEXTMEASURE_H */
