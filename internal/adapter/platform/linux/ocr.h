#ifndef OCR_H
#define OCR_H

// Tesseract-backed text recognition for the Linux half of ports.VisionPort.
//
// The pixels come from screencapture.h; this turns them into positioned words.
// macOS answers the same port with three Vision requests (text, rectangles,
// saliency) — an OCR engine answers the text one only, which is why Linux
// `vision` is text-only and the five rectangle options are declared macOS-only
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
//
// Privacy: recognized text is screen content, exactly as the capture buffer it
// came from is. Nothing here writes a character of it anywhere but the result
// the caller asked for; neru_ocr_result_free wipes the strings before releasing
// them, the engine's `tessedit_write_images` and `debug_file` are pinned so
// tesseract writes neither an image nor a log, and every recognition ends with
// TessBaseAPIClear so the engine holds no copy of the frame between calls.

#define NERU_OCR_OK 0
// The engine could not be initialized for the given tessdata directory and
// language — in practice, language data that is absent or unreadable.
#define NERU_OCR_ERR_INIT 1
// Out of memory building the result.
#define NERU_OCR_ERR_ALLOC 2
// The caller passed no pixels, or a degenerate image.
#define NERU_OCR_ERR_IMAGE 3
// Recognition failed, or ran past the caller's deadline.
#define NERU_OCR_ERR_RECOGNIZE 4
// The engine was busy with another recognition for longer than this caller was
// willing to wait.
#define NERU_OCR_ERR_BUSY 5

typedef struct {
	// text is a NUL-terminated UTF-8 string owned by the result.
	char *text;
	// Bounding box in the captured image's own pixel space: origin (0, 0) is
	// the top-left of the buffer that was handed in, not of the screen.
	int x;
	int y;
	int width;
	int height;
	// confidence is 0..1. Tesseract reports 0..100 and it is normalized here,
	// so the Go side compares against hints.vision.minimum_confidence on the
	// same scale the macOS backend uses.
	float confidence;
} NeruOCRWord;

typedef struct {
	NeruOCRWord *words;
	int count;
} NeruOCRResult;

typedef struct {
	// datapath is the directory holding <language>.traineddata. Never NULL:
	// resolution is done in Go so a missing tessdata reports which file is
	// absent rather than failing inside tesseract.
	const char *datapath;
	const char *language;
	// wordLevel selects per-word boxes (1) over per-line boxes (0), which is
	// what `neru hints --split-word` asks for.
	int wordLevel;
	// timeoutMS bounds one recognition. <= 0 means no deadline.
	int timeoutMS;
} NeruOCRConfig;

// neru_ocr_probe reports whether the engine can be initialized for this
// datapath and language, warming it in the process. NERU_OCR_OK or
// NERU_OCR_ERR_INIT.
int neru_ocr_probe(const NeruOCRConfig *config);

// neru_ocr_recognize reads RGBA8888 pixels and fills out with the words found.
// stride is the byte distance between rows. A frame with no text is
// NERU_OCR_OK with a zero count, not an error.
int neru_ocr_recognize(
    const unsigned char *pixels, int width, int height, int stride, const NeruOCRConfig *config, NeruOCRResult *out);

// neru_ocr_result_free wipes and releases a result. Safe on a zeroed struct.
void neru_ocr_result_free(NeruOCRResult *result);

#endif /* OCR_H */
