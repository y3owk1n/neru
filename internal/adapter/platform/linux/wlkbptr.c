#include "wlkbptr.h"

#include <math.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
	int min_x;
	int min_y;
	int max_x;
	int max_y;
	int parent;
	int first_child;
	int next_sibling;
} Component;

int neru_wlkbptr_detect(
    const unsigned char *rgba, int width, int height, int stride, double scale, NeruTargetResult *out_result) {
	if (rgba == NULL || width <= 0 || height <= 0 || out_result == NULL) {
		return NERU_WLKBPTR_ERR_INVALID;
	}
	if (scale <= 0.0) {
		scale = 1.0;
	}

	out_result->rects = NULL;
	out_result->count = 0;

	int total_pixels = width * height;

	// 1. Grayscale conversion (ITU-R BT.601 integer arithmetic)
	uint8_t *gray = (uint8_t *)malloc(total_pixels);
	if (gray == NULL) {
		return NERU_WLKBPTR_ERR_ALLOC;
	}

	for (int y = 0; y < height; y++) {
		const unsigned char *row = rgba + y * stride;
		uint8_t *gray_row = gray + y * width;
		for (int x = 0; x < width; x++) {
			int r = row[x * 4 + 0];
			int g = row[x * 4 + 1];
			int b = row[x * 4 + 2];
			gray_row[x] = (uint8_t)((77 * r + 150 * g + 29 * b + 128) >> 8);
		}
	}

	// 2. 5x5 separable Gaussian blur ([1, 4, 6, 4, 1] / 16)
	uint8_t *tmp = (uint8_t *)malloc(total_pixels);
	uint8_t *blurred = (uint8_t *)malloc(total_pixels);
	if (tmp == NULL || blurred == NULL) {
		free(gray);
		free(tmp);
		free(blurred);
		return NERU_WLKBPTR_ERR_ALLOC;
	}

	for (int y = 0; y < height; y++) {
		int yw = y * width;
		for (int x = 0; x < width; x++) {
			int xm2 = (x >= 2) ? x - 2 : 0;
			int xm1 = (x >= 1) ? x - 1 : 0;
			int xp1 = (x + 1 < width) ? x + 1 : width - 1;
			int xp2 = (x + 2 < width) ? x + 2 : width - 1;
			tmp[yw + x] = (uint8_t)((gray[yw + xm2] + 4 * gray[yw + xm1] + 6 * gray[yw + x] + 4 * gray[yw + xp1] +
			                         gray[yw + xp2] + 8) >>
			                        4);
		}
	}
	free(gray);

	for (int y = 0; y < height; y++) {
		int ym2 = (y >= 2) ? y - 2 : 0;
		int ym1 = (y >= 1) ? y - 1 : 0;
		int yp1 = (y + 1 < height) ? y + 1 : height - 1;
		int yp2 = (y + 2 < height) ? y + 2 : height - 1;
		int yw = y * width;
		for (int x = 0; x < width; x++) {
			blurred[yw + x] = (uint8_t)((tmp[ym2 * width + x] + 4 * tmp[ym1 * width + x] + 6 * tmp[yw + x] +
			                             4 * tmp[yp1 * width + x] + tmp[yp2 * width + x] + 8) >>
			                            4);
		}
	}
	free(tmp);

	// 3. Sobel edge detection & gradient magnitude / direction
	int16_t *mag = (int16_t *)calloc(total_pixels, sizeof(int16_t));
	uint8_t *dir = (uint8_t *)calloc(total_pixels, sizeof(uint8_t));
	if (mag == NULL || dir == NULL) {
		free(blurred);
		free(mag);
		free(dir);
		return NERU_WLKBPTR_ERR_ALLOC;
	}

	for (int y = 1; y < height - 1; y++) {
		int yw = y * width;
		int y_prev = (y - 1) * width;
		int y_next = (y + 1) * width;
		for (int x = 1; x < width - 1; x++) {
			int p00 = blurred[y_prev + (x - 1)];
			int p01 = blurred[y_prev + x];
			int p02 = blurred[y_prev + (x + 1)];
			int p10 = blurred[yw + (x - 1)];
			int p12 = blurred[yw + (x + 1)];
			int p20 = blurred[y_next + (x - 1)];
			int p21 = blurred[y_next + x];
			int p22 = blurred[y_next + (x + 1)];

			int gx = (p02 + 2 * p12 + p22) - (p00 + 2 * p10 + p20);
			int gy = (p20 + 2 * p21 + p22) - (p00 + 2 * p01 + p02);

			int abs_gx = abs(gx);
			int abs_gy = abs(gy);
			mag[yw + x] = (int16_t)(abs_gx + abs_gy);

			if (abs_gy * 1024 <= abs_gx * 424) {
				dir[yw + x] = 0;  // Horizontal normal
			} else if (abs_gy * 1024 >= abs_gx * 2472) {
				dir[yw + x] = 2;  // Vertical normal
			} else if ((gx > 0 && gy > 0) || (gx < 0 && gy < 0)) {
				dir[yw + x] = 1;  // 45 deg diagonal
			} else {
				dir[yw + x] = 3;  // 135 deg diagonal
			}
		}
	}
	free(blurred);

	// 4. Non-Maximum Suppression (NMS)
	int16_t *nms = (int16_t *)calloc(total_pixels, sizeof(int16_t));
	if (nms == NULL) {
		free(mag);
		free(dir);
		return NERU_WLKBPTR_ERR_ALLOC;
	}

	for (int y = 1; y < height - 1; y++) {
		int yw = y * width;
		for (int x = 1; x < width - 1; x++) {
			int m = mag[yw + x];
			if (m < 70) {
				continue;
			}

			int d = dir[yw + x];
			int n1 = 0, n2 = 0;
			switch (d) {
			case 0:
				n1 = mag[yw + (x - 1)];
				n2 = mag[yw + (x + 1)];
				break;
			case 1:
				n1 = mag[(y - 1) * width + (x + 1)];
				n2 = mag[(y + 1) * width + (x - 1)];
				break;
			case 2:
				n1 = mag[(y - 1) * width + x];
				n2 = mag[(y + 1) * width + x];
				break;
			case 3:
				n1 = mag[(y - 1) * width + (x - 1)];
				n2 = mag[(y + 1) * width + (x + 1)];
				break;
			}

			if (m >= n1 && m >= n2) {
				nms[yw + x] = (int16_t)m;
			}
		}
	}
	free(mag);
	free(dir);

	// 5. Canny Hysteresis Thresholding (low = 70, high = 220)
	uint8_t *edges = (uint8_t *)calloc(total_pixels, sizeof(uint8_t));
	int *queue = (int *)malloc(total_pixels * sizeof(int));
	if (edges == NULL || queue == NULL) {
		free(nms);
		free(edges);
		free(queue);
		return NERU_WLKBPTR_ERR_ALLOC;
	}

	int q_head = 0, q_tail = 0;
	for (int i = 0; i < total_pixels; i++) {
		if (nms[i] >= 220) {
			edges[i] = 255;
			queue[q_tail++] = i;
		}
	}

	int dx8[8] = {-1, 0, 1, -1, 1, -1, 0, 1};
	int dy8[8] = {-1, -1, -1, 0, 0, 1, 1, 1};

	while (q_head < q_tail) {
		int idx = queue[q_head++];
		int cx = idx % width;
		int cy = idx / width;

		for (int k = 0; k < 8; k++) {
			int nx = cx + dx8[k];
			int ny = cy + dy8[k];
			if (nx >= 0 && nx < width && ny >= 0 && ny < height) {
				int nidx = ny * width + nx;
				if (!edges[nidx] && nms[nidx] >= 70) {
					edges[nidx] = 255;
					queue[q_tail++] = nidx;
				}
			}
		}
	}
	free(queue);
	free(nms);

	// 6. Morphological Dilation with rectangular kernel (round(2.5*scale) x round(3.5*scale))
	int kh = (int)round(2.5 * scale);
	int kw = (int)round(3.5 * scale);
	if (kh < 1) {
		kh = 1;
	}
	if (kw < 1) {
		kw = 1;
	}

	uint8_t *dilated_h = (uint8_t *)calloc(total_pixels, sizeof(uint8_t));
	uint8_t *dilated = (uint8_t *)calloc(total_pixels, sizeof(uint8_t));
	if (dilated_h == NULL || dilated == NULL) {
		free(edges);
		free(dilated_h);
		free(dilated);
		return NERU_WLKBPTR_ERR_ALLOC;
	}

	int radius_x = kw / 2;
	for (int y = 0; y < height; y++) {
		int yw = y * width;
		for (int x = 0; x < width; x++) {
			int start_x = (x - radius_x >= 0) ? x - radius_x : 0;
			int end_x = (x + kw - 1 - radius_x < width) ? x + kw - 1 - radius_x : width - 1;
			for (int kx = start_x; kx <= end_x; kx++) {
				if (edges[yw + kx]) {
					dilated_h[yw + x] = 255;
					break;
				}
			}
		}
	}
	free(edges);

	int radius_y = kh / 2;
	for (int x = 0; x < width; x++) {
		for (int y = 0; y < height; y++) {
			int start_y = (y - radius_y >= 0) ? y - radius_y : 0;
			int end_y = (y + kh - 1 - radius_y < height) ? y + kh - 1 - radius_y : height - 1;
			for (int ky = start_y; ky <= end_y; ky++) {
				if (dilated_h[ky * width + x]) {
					dilated[y * width + x] = 255;
					break;
				}
			}
		}
	}
	free(dilated_h);

	// 7. Connected Component Labeling via 8-connected BFS
	uint8_t *visited = (uint8_t *)calloc(total_pixels, sizeof(uint8_t));
	int *ccl_queue = (int *)malloc(total_pixels * sizeof(int));
	if (visited == NULL || ccl_queue == NULL) {
		free(dilated);
		free(visited);
		free(ccl_queue);
		return NERU_WLKBPTR_ERR_ALLOC;
	}

	size_t comp_cap = 256;
	size_t comp_count = 0;
	Component *comps = (Component *)malloc(comp_cap * sizeof(Component));
	if (comps == NULL) {
		free(dilated);
		free(visited);
		free(ccl_queue);
		return NERU_WLKBPTR_ERR_ALLOC;
	}

	for (int y = 0; y < height; y++) {
		int yw = y * width;
		for (int x = 0; x < width; x++) {
			int idx = yw + x;
			if (dilated[idx] && !visited[idx]) {
				if (comp_count >= comp_cap) {
					comp_cap *= 2;
					Component *re = (Component *)realloc(comps, comp_cap * sizeof(Component));
					if (re == NULL) {
						free(dilated);
						free(visited);
						free(ccl_queue);
						free(comps);
						return NERU_WLKBPTR_ERR_ALLOC;
					}
					comps = re;
				}

				int cidx = (int)comp_count++;
				comps[cidx].min_x = x;
				comps[cidx].max_x = x;
				comps[cidx].min_y = y;
				comps[cidx].max_y = y;
				comps[cidx].parent = -1;
				comps[cidx].first_child = -1;
				comps[cidx].next_sibling = -1;

				visited[idx] = 1;
				int head = 0, tail = 0;
				ccl_queue[tail++] = idx;

				while (head < tail) {
					int curr = ccl_queue[head++];
					int cx = curr % width;
					int cy = curr / width;

					if (cx < comps[cidx].min_x) {
						comps[cidx].min_x = cx;
					}
					if (cx > comps[cidx].max_x) {
						comps[cidx].max_x = cx;
					}
					if (cy < comps[cidx].min_y) {
						comps[cidx].min_y = cy;
					}
					if (cy > comps[cidx].max_y) {
						comps[cidx].max_y = cy;
					}

					for (int k = 0; k < 8; k++) {
						int nx = cx + dx8[k];
						int ny = cy + dy8[k];
						if (nx >= 0 && nx < width && ny >= 0 && ny < height) {
							int nidx = ny * width + nx;
							if (dilated[nidx] && !visited[nidx]) {
								visited[nidx] = 1;
								ccl_queue[tail++] = nidx;
							}
						}
					}
				}
			}
		}
	}
	free(dilated);
	free(visited);
	free(ccl_queue);

	// Build hierarchy: find immediate enclosing parent for each component
	for (size_t i = 0; i < comp_count; i++) {
		int best_p = -1;
		long long best_area = -1;

		for (size_t j = 0; j < comp_count; j++) {
			if (i == j) {
				continue;
			}
			if (comps[j].min_x <= comps[i].min_x + 1 && comps[j].max_x >= comps[i].max_x - 1 &&
			    comps[j].min_y <= comps[i].min_y + 1 && comps[j].max_y >= comps[i].max_y - 1) {
				long long jw = comps[j].max_x - comps[j].min_x + 1;
				long long jh = comps[j].max_y - comps[j].min_y + 1;
				long long area = jw * jh;
				if (best_area < 0 || area < best_area) {
					best_area = area;
					best_p = (int)j;
				}
			}
		}
		comps[i].parent = best_p;
		if (best_p >= 0) {
			comps[i].next_sibling = comps[best_p].first_child;
			comps[best_p].first_child = (int)i;
		}
	}

	// 8. Bounding box projection and heuristic filtering (matching wl-kbptr's filter_rects)
	uint8_t *filtered = (uint8_t *)calloc(comp_count, sizeof(uint8_t));
	double *rx = (double *)malloc(comp_count * sizeof(double));
	double *ry = (double *)malloc(comp_count * sizeof(double));
	double *rw = (double *)malloc(comp_count * sizeof(double));
	double *rh = (double *)malloc(comp_count * sizeof(double));

	if (filtered == NULL || rx == NULL || ry == NULL || rw == NULL || rh == NULL) {
		free(comps);
		free(filtered);
		free(rx);
		free(ry);
		free(rw);
		free(rh);
		return NERU_WLKBPTR_ERR_ALLOC;
	}

	for (size_t i = 0; i < comp_count; i++) {
		rx[i] = (double)comps[i].min_x / scale;
		ry[i] = (double)comps[i].min_y / scale;
		rw[i] = (double)(comps[i].max_x - comps[i].min_x + 1) / scale;
		rh[i] = (double)(comps[i].max_y - comps[i].min_y + 1) / scale;

		// Size bounds filtering: discard too tall/wide or tiny noise.
		// Buttons and small inputs are typically under 50px high, but desktop
		// notification cards, toasts, and floating dialog cards can be up to 160px
		// high and 650px wide.
		if (rh[i] >= 160.0 || rw[i] >= 650.0 || rh[i] <= 3.0 || rw[i] <= 7.0) {
			filtered[i] = 1;
		}
	}

	// Explore hierarchy for children with parent >= 0
	int *to_explore = (int *)malloc(comp_count * sizeof(int));
	if (to_explore == NULL) {
		free(comps);
		free(filtered);
		free(rx);
		free(ry);
		free(rw);
		free(rh);
		return NERU_WLKBPTR_ERR_ALLOC;
	}

	int exp_count = 0;
	for (size_t i = 0; i < comp_count; i++) {
		if (comps[i].parent >= 0) {
			to_explore[exp_count++] = (int)i;
		}
	}

	while (exp_count > 0) {
		int i = to_explore[--exp_count];
		if (!filtered[i]) {
			int p = comps[i].parent;
			if (p >= 0 && filtered[p]) {
				// Parent is an enclosing layout container (dialog box, card, etc.),
				// not an interactive button. Skip button-vs-button deduplication
				// so child targets inside the container are kept.
				goto skip_child;
			}

			// Flat inner lines (e.g. hamburger menu lines)
			if (rh[i] <= 6.0) {
				filtered[i] = 1;
				goto skip_child;
			}

			if (p >= 0) {
				double cx = rx[i] + rw[i] / 2.0;
				double cy = ry[i] + rh[i] / 2.0;
				double pcx = rx[p] + rw[p] / 2.0;
				double pcy = ry[p] + rh[p] / 2.0;

				// Discard inner targets with nearly same center
				if (fabs(cx - pcx) < 8.0 && fabs(cy - pcy) < 8.0) {
					filtered[i] = 1;
					goto skip_child;
				}

				// Discard inner targets of square icons/buttons
				if (fabs(rh[p] - rw[p]) < 5.0 && rh[p] < 40.0 && rw[p] < 40.0) {
					filtered[i] = 1;
					goto skip_child;
				}
			}
		}

	skip_child:
		int child = comps[i].first_child;
		while (child >= 0) {
			to_explore[exp_count++] = child;
			child = comps[child].next_sibling;
		}
	}

	// If a component is an enclosing container (50 <= height < 160) and contains
	// interactive button children, filter out the container so only the buttons
	// inside it are clicked (e.g. dialog with OK button).
	// If it contains no button children, it is a notification popup or toast card,
	// so it remains a clickable target.
	for (size_t i = 0; i < comp_count; i++) {
		if (!filtered[i] && rh[i] >= 50.0) {
			int child = comps[i].first_child;
			while (child >= 0) {
				if (!filtered[child] && rh[child] < 50.0 && rh[child] > 6.0) {
					filtered[i] = 1;
					break;
				}
				child = comps[child].next_sibling;
			}
		}
	}

	free(to_explore);
	free(comps);

	int final_count = 0;
	for (size_t i = 0; i < comp_count; i++) {
		if (!filtered[i]) {
			final_count++;
		}
	}

	out_result->count = final_count;
	out_result->rects = (NeruTargetRect *)malloc(final_count * sizeof(NeruTargetRect));
	if (final_count > 0 && out_result->rects == NULL) {
		free(filtered);
		free(rx);
		free(ry);
		free(rw);
		free(rh);
		return NERU_WLKBPTR_ERR_ALLOC;
	}

	int out_idx = 0;
	for (size_t i = 0; i < comp_count; i++) {
		if (!filtered[i]) {
			out_result->rects[out_idx].x = (int)round(rx[i]);
			out_result->rects[out_idx].y = (int)round(ry[i]);
			out_result->rects[out_idx].w = (int)round(rw[i]);
			out_result->rects[out_idx].h = (int)round(rh[i]);
			out_idx++;
		}
	}

	free(filtered);
	free(rx);
	free(ry);
	free(rw);
	free(rh);

	return NERU_WLKBPTR_OK;
}

void neru_wlkbptr_free(NeruTargetResult *result) {
	if (result != NULL && result->rects != NULL) {
		free(result->rects);
		result->rects = NULL;
		result->count = 0;
	}
}
