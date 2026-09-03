#ifndef NERU_CONTOUR_H
#define NERU_CONTOUR_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define NERU_CONTOUR_OK 0
#define NERU_CONTOUR_ERR_INVALID 1
#define NERU_CONTOUR_ERR_ALLOC 2

typedef struct {
	int x;
	int y;
	int w;
	int h;
} NeruTargetRect;

typedef struct {
	NeruTargetRect *rects;
	int count;
} NeruTargetResult;

// neru_contour_detect is a port of the target detection algorithm in wl-kbptr
// (https://github.com/moverest/wl-kbptr, MIT):
// 1. Convert RGBA to Grayscale
// 2. 5x5 Gaussian blur
// 3. Sobel edge detection & gradient magnitude
// 4. Non-maximum suppression
// 5. Canny hysteresis thresholding (low: 70, high: 220)
// 6. Morphological dilation using a scale-dependent rectangular kernel
// 7. Suzuki-Abe border following for hierarchical contour extraction
// 8. Bounding box computation and hierarchical heuristic filtering
//
// Returns NERU_CONTOUR_OK on success, or an error code on failure.
// Result rects must be freed with neru_contour_free().
int neru_contour_detect(
    const unsigned char *rgba, int width, int height, int stride, double scale, NeruTargetResult *out_result);

void neru_contour_free(NeruTargetResult *result);

#ifdef __cplusplus
}
#endif

#endif /* NERU_CONTOUR_H */
