#ifndef NERU_WLKBPTR_H
#define NERU_WLKBPTR_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define NERU_WLKBPTR_OK 0
#define NERU_WLKBPTR_ERR_INVALID 1
#define NERU_WLKBPTR_ERR_ALLOC 2

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

// neru_wlkbptr_detect replicates the target detection algorithm from wl-kbptr:
// 1. Convert RGBA to Grayscale
// 2. 5x5 Gaussian blur
// 3. Sobel edge detection & gradient magnitude
// 4. Non-maximum suppression
// 5. Canny hysteresis thresholding (low: 70, high: 220)
// 6. Morphological dilation using a scale-dependent rectangular kernel
// 7. Suzuki-Abe border following for hierarchical contour extraction
// 8. Bounding box computation and hierarchical heuristic filtering
//
// Returns NERU_WLKBPTR_OK on success, or an error code on failure.
// Result rects must be freed with neru_wlkbptr_free().
int neru_wlkbptr_detect(
    const unsigned char *rgba, int width, int height, int stride, double scale, NeruTargetResult *out_result);

void neru_wlkbptr_free(NeruTargetResult *result);

#ifdef __cplusplus
}
#endif

#endif /* NERU_WLKBPTR_H */
