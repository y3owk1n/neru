// Package contour is a contour-based target detector. The algorithm is a
// faithful port of the one in wl-kbptr (https://github.com/moverest/wl-kbptr,
// MIT) and its heuristics are theirs: grayscale, Gaussian blur, Sobel, Canny
// hysteresis, dilation and connected-component labeling over an RGBA frame,
// returning the bounding boxes of things that look like buttons, icons and
// text. It is pure Go with no cgo, so it builds everywhere Neru does,
// CGO_ENABLED=0 included; each vision adapter supplies the frame from its own
// capture backend and maps the boxes back to global coordinates.
//
// The frame is screen content. Detect reads it and returns rectangles; nothing
// here logs, copies or retains the pixels.
package contour
