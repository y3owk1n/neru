#include "ocr.h"

#include "screencapture.h"

#include <errno.h>
#include <pthread.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <tesseract/capi.h>
#include <time.h>

// The engine is cached for the daemon's lifetime, and that is a latency
// decision rather than a convenience: TessBaseAPIInit2 loads an LSTM model from
// disk, which costs hundreds of milliseconds, and a hint activation that paid
// it every time would be unusable. What is cached is the *engine*, never a
// frame — every recognition ends in neru_ocr_reset_locked, so between calls the
// handle holds a loaded model and nothing derived from anyone's screen.
//
// Recognition is serialized on this mutex. TessBaseAPI is not thread-safe, and
// the daemon can reach here from the IPC handler and a mode at once.
//
// Nothing waits on it forever, which is the point of neru_ocr_lock below. The
// only bound on a recognition is tesseract's own deadline, and tesseract checks
// that *between* recognition units — so one pathological frame can overrun it.
// A plain lock would then wedge every later activation in cgo, where Go cannot
// cancel it, and the daemon would need restarting to hint again. A bounded wait
// turns that into one failed activation that says so.
static pthread_mutex_t neru_ocr_mutex = PTHREAD_MUTEX_INITIALIZER;
static TessBaseAPI *neru_ocr_api;
static char *neru_ocr_datapath;
static char *neru_ocr_language;

// How long a caller with no budget of its own waits for the engine. Long enough
// to queue behind a normal recognition (tens of milliseconds), short enough
// that a wedged one is reported rather than waited on.
#define NERU_OCR_DEFAULT_WAIT_MS 3000

// neru_ocr_now_ms is a monotonic millisecond clock, for measuring how long a
// recognition took. Monotonic so a clock adjustment mid-recognition cannot turn
// a fast frame into a reported timeout.
static int64_t neru_ocr_now_ms(void) {
	struct timespec now;
	if (clock_gettime(CLOCK_MONOTONIC, &now) != 0) {
		return 0;
	}

	return (int64_t)now.tv_sec * 1000 + now.tv_nsec / 1000000;
}

// neru_ocr_lock takes the engine lock, waiting at most wait_ms. Returns
// NERU_OCR_OK when it holds the lock, NERU_OCR_ERR_BUSY otherwise.
static int neru_ocr_lock(int wait_ms) {
	if (wait_ms <= 0) {
		wait_ms = NERU_OCR_DEFAULT_WAIT_MS;
	}

	struct timespec deadline;
	if (clock_gettime(CLOCK_REALTIME, &deadline) != 0) {
		// No clock to build a deadline from. Blocking is still better than
		// refusing every recognition on a machine whose clock call failed.
		return pthread_mutex_lock(&neru_ocr_mutex) == 0 ? NERU_OCR_OK : NERU_OCR_ERR_BUSY;
	}

	deadline.tv_sec += wait_ms / 1000;
	deadline.tv_nsec += (long)(wait_ms % 1000) * 1000000L;

	if (deadline.tv_nsec >= 1000000000L) {
		deadline.tv_sec += 1;
		deadline.tv_nsec -= 1000000000L;
	}

	return pthread_mutex_timedlock(&neru_ocr_mutex, &deadline) == 0 ? NERU_OCR_OK : NERU_OCR_ERR_BUSY;
}

// Screens are close enough to 96 dpi, and saying so silences tesseract's
// "Invalid resolution 0 dpi" warning on stderr as well as improving its
// segmentation.
#define NERU_OCR_SOURCE_DPI 96

// RGBA8888, the layout NeruCapture and Go's image.RGBA both use.
#define NERU_OCR_BYTES_PER_PIXEL 4

// How many words to allocate room for before growing. UI text is scattered
// labels; a window with more than this is doubled rather than reallocated per
// word.
#define NERU_OCR_INITIAL_CAPACITY 64

static int neru_ocr_string_equal(const char *a, const char *b) {
	if (a == NULL || b == NULL) {
		return a == b;
	}

	return strcmp(a, b) == 0;
}

static void neru_ocr_release_locked(void) {
	if (neru_ocr_api != NULL) {
		TessBaseAPIEnd(neru_ocr_api);
		TessBaseAPIDelete(neru_ocr_api);
		neru_ocr_api = NULL;
	}

	free(neru_ocr_datapath);
	neru_ocr_datapath = NULL;
	free(neru_ocr_language);
	neru_ocr_language = NULL;
}

// neru_ocr_acquire_locked returns the engine for this datapath and language,
// creating it on first use and rebuilding it if either changed. NULL means
// tesseract refused to initialize, which for a caller that resolved the
// datapath itself means unreadable or unusable language data.
static TessBaseAPI *neru_ocr_acquire_locked(const NeruOCRConfig *config) {
	if (config == NULL || config->datapath == NULL || config->language == NULL) {
		return NULL;
	}

	if (neru_ocr_api != NULL && neru_ocr_string_equal(neru_ocr_datapath, config->datapath) &&
	    neru_ocr_string_equal(neru_ocr_language, config->language)) {
		return neru_ocr_api;
	}

	neru_ocr_release_locked();

	TessBaseAPI *api = TessBaseAPICreate();
	if (api == NULL) {
		return NULL;
	}

	// OEM_LSTM_ONLY rather than the legacy engine or the combination: the
	// legacy half is slower, needs its own data files, and is what tesseract 5
	// itself defaults away from.
	if (TessBaseAPIInit2(api, config->datapath, config->language, OEM_LSTM_ONLY) != 0) {
		TessBaseAPIDelete(api);

		return NULL;
	}

	// Sparse text rather than the document-oriented default: UI text is
	// scattered labels in no reading order, and the layout analysis that helps
	// a scanned page actively merges unrelated controls into "paragraphs".
	TessBaseAPISetPageSegMode(api, PSM_SPARSE_TEXT);

	// Both of these are privacy settings, not tuning. tessedit_write_images
	// dumps the thresholded input to tessinput.tif in the working directory,
	// and debug_file is where tesseract writes diagnostics that can quote
	// recognized text. Neither may ever hold screen content.
	TessBaseAPISetVariable(api, "tessedit_write_images", "F");
	TessBaseAPISetVariable(api, "debug_file", "/dev/null");

	neru_ocr_datapath = strdup(config->datapath);
	neru_ocr_language = strdup(config->language);

	if (neru_ocr_datapath == NULL || neru_ocr_language == NULL) {
		TessBaseAPIEnd(api);
		TessBaseAPIDelete(api);
		free(neru_ocr_datapath);
		neru_ocr_datapath = NULL;
		free(neru_ocr_language);
		neru_ocr_language = NULL;

		return NULL;
	}

	neru_ocr_api = api;

	return neru_ocr_api;
}

// neru_ocr_reset_locked drops the frame and the recognition results. Every path
// out of a recognition goes through it, so no pixels and no recognized text
// reach the next one.
//
// It deliberately does *not* call TessBaseAPIClearAdaptiveClassifier, and that
// is a measured decision rather than an oversight. Clearing the adaptive
// classifier per frame reads like the tidy thing to do — it holds character
// shapes learned from the frame just read — but it also resets the document
// dictionary, and the reinitialization that costs is paid by the *next*
// recognition: on a 1904x994 frame it took recognition from 0.5s to 3.5s, a
// sevenfold regression on the one path where latency is the product. Keeping it
// measured flat across repeated frames instead.
//
// What is retained is classifier state — character shape templates — and not
// text, not pixels, and nothing that can be read back as screen content. It
// lives in memory for the life of the daemon and never touches disk.
static void neru_ocr_reset_locked(TessBaseAPI *api) { TessBaseAPIClear(api); }

int neru_ocr_probe(const NeruOCRConfig *config) {
	int locked = neru_ocr_lock(config != NULL ? config->timeoutMS : 0);
	if (locked != NERU_OCR_OK) {
		return locked;
	}

	TessBaseAPI *api = neru_ocr_acquire_locked(config);
	pthread_mutex_unlock(&neru_ocr_mutex);

	return api != NULL ? NERU_OCR_OK : NERU_OCR_ERR_INIT;
}

// neru_ocr_append grows words and copies one finding into it. The copy is what
// lets the tesseract-owned string be wiped and freed immediately, so a run of
// screen text lives in exactly one buffer.
static int neru_ocr_append(
    NeruOCRWord **words, int *count, int *capacity, const char *text, float confidence, int left, int top, int right,
    int bottom) {
	if (*count == *capacity) {
		int next = (*capacity == 0) ? NERU_OCR_INITIAL_CAPACITY : (*capacity * 2);

		NeruOCRWord *grown = realloc(*words, (size_t)next * sizeof(NeruOCRWord));
		if (grown == NULL) {
			return NERU_OCR_ERR_ALLOC;
		}

		*words = grown;
		*capacity = next;
	}

	char *copy = strdup(text);
	if (copy == NULL) {
		return NERU_OCR_ERR_ALLOC;
	}

	NeruOCRWord *word = &(*words)[*count];
	word->text = copy;
	word->x = left;
	word->y = top;
	word->width = right - left;
	word->height = bottom - top;
	word->confidence = confidence;

	(*count)++;

	return NERU_OCR_OK;
}

// neru_ocr_collect walks the recognition result at one iterator level. It is
// split out so the caller can release the iterator and clear the engine on
// every path, including the allocation failures.
static int neru_ocr_collect(
    TessResultIterator *iterator, TessPageIteratorLevel level, NeruOCRWord **words, int *count, int *capacity) {
	do {
		char *text = TessResultIteratorGetUTF8Text(iterator, level);
		if (text == NULL) {
			// In a do-while, continue evaluates the condition — which is the
			// iterator advance — so this skips the element rather than
			// spinning on it.
			continue;
		}

		int left = 0;
		int top = 0;
		int right = 0;
		int bottom = 0;

		const TessPageIterator *page = TessResultIteratorGetPageIteratorConst(iterator);

		int status = NERU_OCR_OK;
		if (page != NULL && TessPageIteratorBoundingBox(page, level, &left, &top, &right, &bottom) && right > left &&
		    bottom > top) {
			float confidence = TessResultIteratorConfidence(iterator, level) / 100.0f;
			if (confidence < 0.0f) {
				confidence = 0.0f;
			}
			if (confidence > 1.0f) {
				confidence = 1.0f;
			}

			status = neru_ocr_append(words, count, capacity, text, confidence, left, top, right, bottom);
		}

		// The tesseract-owned string held screen text; wipe it before handing
		// it back rather than leaving a readable copy in freed heap.
		neru_capture_wipe((unsigned char *)text, strlen(text));
		TessDeleteText(text);

		if (status != NERU_OCR_OK) {
			return status;
		}
	} while (TessResultIteratorNext(iterator, level));

	return NERU_OCR_OK;
}

int neru_ocr_recognize(
    const unsigned char *pixels, int width, int height, int stride, const NeruOCRConfig *config, NeruOCRResult *out) {
	if (out == NULL) {
		return NERU_OCR_ERR_IMAGE;
	}

	out->words = NULL;
	out->count = 0;
	out->elapsedMS = 0;

	if (pixels == NULL || width <= 0 || height <= 0 || stride < width * NERU_OCR_BYTES_PER_PIXEL) {
		return NERU_OCR_ERR_IMAGE;
	}

	int locked = neru_ocr_lock(config->timeoutMS);
	if (locked != NERU_OCR_OK) {
		return locked;
	}

	TessBaseAPI *api = neru_ocr_acquire_locked(config);
	if (api == NULL) {
		pthread_mutex_unlock(&neru_ocr_mutex);

		return NERU_OCR_ERR_INIT;
	}

	TessBaseAPISetImage(api, pixels, width, height, NERU_OCR_BYTES_PER_PIXEL, stride);
	TessBaseAPISetSourceResolution(api, NERU_OCR_SOURCE_DPI);

	// The deadline is why recognition goes through a monitor at all: tesseract
	// has no other way to stop, and a caller blocked in cgo cannot be canceled
	// from Go.
	ETEXT_DESC *monitor = NULL;
	if (config->timeoutMS > 0) {
		monitor = TessMonitorCreate();
		if (monitor == NULL) {
			neru_ocr_reset_locked(api);
			pthread_mutex_unlock(&neru_ocr_mutex);

			return NERU_OCR_ERR_ALLOC;
		}

		TessMonitorSetDeadlineMSecs(monitor, config->timeoutMS);
	}

	int64_t started = neru_ocr_now_ms();

	int recognized = TessBaseAPIRecognize(api, monitor);

	int64_t elapsed = neru_ocr_now_ms() - started;
	out->elapsedMS = (int)(elapsed > 0 ? elapsed : 0);

	if (monitor != NULL) {
		TessMonitorDelete(monitor);
	}

	if (recognized != 0) {
		neru_ocr_reset_locked(api);
		pthread_mutex_unlock(&neru_ocr_mutex);

		// Tesseract reports one failure code for "gave up on the deadline" and
		// "could not read this frame", and the two want opposite responses from
		// a user. The clock is what separates them: recognition that ran to the
		// budget was stopped by it.
		if (config->timeoutMS > 0 && elapsed >= (int64_t)config->timeoutMS) {
			return NERU_OCR_ERR_TIMEOUT;
		}

		return NERU_OCR_ERR_RECOGNIZE;
	}

	TessPageIteratorLevel level = config->wordLevel ? RIL_WORD : RIL_TEXTLINE;

	NeruOCRWord *words = NULL;
	int count = 0;
	int capacity = 0;
	int status = NERU_OCR_OK;

	TessResultIterator *iterator = TessBaseAPIGetIterator(api);
	if (iterator != NULL) {
		status = neru_ocr_collect(iterator, level, &words, &count, &capacity);
		TessResultIteratorDelete(iterator);
	}

	neru_ocr_reset_locked(api);
	pthread_mutex_unlock(&neru_ocr_mutex);

	if (status != NERU_OCR_OK) {
		NeruOCRResult partial = {words, count};
		neru_ocr_result_free(&partial);

		return status;
	}

	out->words = words;
	out->count = count;

	return NERU_OCR_OK;
}

void neru_ocr_result_free(NeruOCRResult *result) {
	if (result == NULL) {
		return;
	}

	for (int i = 0; i < result->count; i++) {
		char *text = result->words[i].text;
		if (text == NULL) {
			continue;
		}

		neru_capture_wipe((unsigned char *)text, strlen(text));
		free(text);
	}

	free(result->words);
	result->words = NULL;
	result->count = 0;
}
