package contour

import (
	"image"
	"math"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Canny thresholds and the size heuristics below are wl-kbptr's numbers,
// kept verbatim so the detector finds the same targets its origin does.
const (
	cannyLow  = 70
	cannyHigh = 220

	maxTargetHeight = 160.0
	maxTargetWidth  = 650.0
	minTargetHeight = 3.0
	minTargetWidth  = 7.0
	flatLineHeight  = 6.0
	containerHeight = 50.0
	sameCenterSlack = 8.0
	squareIconSlack = 5.0
	squareIconSize  = 40.0
)

// component is one 8-connected blob of dilated edge pixels with its place in
// the enclosure hierarchy. Indices are into the components slice; -1 is none.
type component struct {
	minX, minY, maxX, maxY int
	parent                 int
	firstChild             int
	nextSibling            int
}

func (c component) area() int64 {
	return int64(c.maxX-c.minX+1) * int64(c.maxY-c.minY+1)
}

// Detect detects interactive UI target bounding boxes from an RGBA image
// buffer using the contour detection algorithm ported from wl-kbptr: grayscale,
// 5x5 Gaussian blur, Sobel, non-maximum suppression, Canny hysteresis,
// scale-dependent dilation, connected components and hierarchical filtering.
// Rectangles are returned in logical coordinates (divided by scale). An empty
// frame is refused; a frame with nothing in it returns no rectangles and no
// error.
func Detect(img *image.RGBA, scale float64) ([]image.Rectangle, error) {
	if img == nil || img.Rect.Dx() <= 0 || img.Rect.Dy() <= 0 {
		return nil, derrors.New(
			derrors.CodeInvalidInput,
			"contour detection needs a non-empty frame",
		)
	}

	if scale <= 0 {
		scale = 1.0
	}

	width, height := img.Rect.Dx(), img.Rect.Dy()

	gray := grayscale(img.Pix, width, height, img.Stride)
	blurred := gaussianBlur(gray, width, height)
	edges := cannyEdges(blurred, width, height)
	dilated := dilate(edges, width, height, scale)
	comps := connectedComponents(dilated, width, height)
	buildHierarchy(comps)

	return filterTargets(comps, scale), nil
}

// grayscale converts RGBA to luma with ITU-R BT.601 integer arithmetic.
//
//nolint:mnd // BT.601 luma weights as the standard writes them
func grayscale(pix []uint8, width, height, stride int) []uint8 {
	gray := make([]uint8, width*height)

	for y := range height {
		row := pix[y*stride : y*stride+width*4]
		out := gray[y*width : (y+1)*width]

		for x := range width {
			r := int(row[x*4])
			g := int(row[x*4+1])
			b := int(row[x*4+2])
			out[x] = uint8((77*r + 150*g + 29*b + 128) >> 8)
		}
	}

	return gray
}

// gaussianBlur is a separable 5x5 blur with the [1 4 6 4 1]/16 kernel and
// clamped edges.
//
//nolint:mnd // the [1 4 6 4 1] kernel as the algorithm writes it
func gaussianBlur(gray []uint8, width, height int) []uint8 {
	tmp := make([]uint8, width*height)
	out := make([]uint8, width*height)

	for y := range height {
		row := gray[y*width : (y+1)*width]
		dst := tmp[y*width : (y+1)*width]

		for x := range width {
			xm2 := max(x-2, 0)
			xm1 := max(x-1, 0)
			xp1 := min(x+1, width-1)
			xp2 := min(x+2, width-1)
			dst[x] = uint8((int(row[xm2]) + 4*int(row[xm1]) + 6*int(row[x]) +
				4*int(row[xp1]) + int(row[xp2]) + 8) >> 4)
		}
	}

	for y := range height {
		ym2 := max(y-2, 0) * width
		ym1 := max(y-1, 0) * width
		yp1 := min(y+1, height-1) * width
		yp2 := min(y+2, height-1) * width
		yw := y * width

		for x := range width {
			out[yw+x] = uint8((int(tmp[ym2+x]) + 4*int(tmp[ym1+x]) + 6*int(tmp[yw+x]) +
				4*int(tmp[yp1+x]) + int(tmp[yp2+x]) + 8) >> 4)
		}
	}

	return out
}

// cannyEdges runs Sobel, non-maximum suppression along the gradient normal,
// and hysteresis thresholding. The result is 255 on edge pixels, 0 elsewhere.
//
//nolint:varnamelen,mnd // pixel kernel: coordinates and kernel coefficients read as the algorithm writes them
func cannyEdges(blurred []uint8, width, height int) []uint8 {
	total := width * height
	mag := make([]int16, total)
	dir := make([]uint8, total)

	for y := 1; y < height-1; y++ {
		yw := y * width
		yPrev := (y - 1) * width
		yNext := (y + 1) * width

		for x := 1; x < width-1; x++ {
			p00 := int(blurred[yPrev+x-1])
			p01 := int(blurred[yPrev+x])
			p02 := int(blurred[yPrev+x+1])
			p10 := int(blurred[yw+x-1])
			p12 := int(blurred[yw+x+1])
			p20 := int(blurred[yNext+x-1])
			p21 := int(blurred[yNext+x])
			p22 := int(blurred[yNext+x+1])

			gx := (p02 + 2*p12 + p22) - (p00 + 2*p10 + p20)
			gy := (p20 + 2*p21 + p22) - (p00 + 2*p01 + p02)

			absGx, absGy := abs(gx), abs(gy)
			mag[yw+x] = int16(absGx + absGy)

			switch {
			case absGy*1024 <= absGx*424:
				dir[yw+x] = 0 // horizontal normal
			case absGy*1024 >= absGx*2472:
				dir[yw+x] = 2 // vertical normal
			case (gx > 0 && gy > 0) || (gx < 0 && gy < 0):
				dir[yw+x] = 1 // 45 degree diagonal
			default:
				dir[yw+x] = 3 // 135 degree diagonal
			}
		}
	}

	nms := make([]int16, total)

	for y := 1; y < height-1; y++ {
		yw := y * width

		for x := 1; x < width-1; x++ {
			m := mag[yw+x]
			if m < cannyLow {
				continue
			}

			var n1, n2 int16

			switch dir[yw+x] {
			case 0:
				n1, n2 = mag[yw+x-1], mag[yw+x+1]
			case 1:
				n1, n2 = mag[yw-width+x+1], mag[yw+width+x-1]
			case 2:
				n1, n2 = mag[yw-width+x], mag[yw+width+x]
			default:
				n1, n2 = mag[yw-width+x-1], mag[yw+width+x+1]
			}

			if m >= n1 && m >= n2 {
				nms[yw+x] = m
			}
		}
	}

	edges := make([]uint8, total)
	queue := make([]int32, 0, total/8)

	for i, v := range nms {
		if v >= cannyHigh {
			edges[i] = 255
			queue = append(queue, int32(i))
		}
	}

	for head := 0; head < len(queue); head++ {
		idx := int(queue[head])
		cx, cy := idx%width, idx/width

		for k := range 8 {
			nx, ny := cx+dx8[k], cy+dy8[k]
			if nx < 0 || nx >= width || ny < 0 || ny >= height {
				continue
			}

			nidx := ny*width + nx
			if edges[nidx] == 0 && nms[nidx] >= cannyLow {
				edges[nidx] = 255
				queue = append(queue, int32(nidx))
			}
		}
	}

	return edges
}

var (
	dx8 = [8]int{-1, 0, 1, -1, 1, -1, 0, 1}
	dy8 = [8]int{-1, -1, -1, 0, 0, 1, 1, 1}
)

// dilate grows edges with a rectangular kernel of round(2.5*scale) rows by
// round(3.5*scale) columns, applied separably.
//
//nolint:varnamelen,mnd // pixel kernel: coordinates and kernel coefficients read as the algorithm writes them
func dilate(edges []uint8, width, height int, scale float64) []uint8 {
	kh := max(int(math.Round(2.5*scale)), 1)
	kw := max(int(math.Round(3.5*scale)), 1)

	horiz := make([]uint8, width*height)
	out := make([]uint8, width*height)

	radiusX := kw / 2
	for y := range height {
		yw := y * width

		for x := range width {
			start := max(x-radiusX, 0)
			end := min(x+kw-1-radiusX, width-1)

			for kx := start; kx <= end; kx++ {
				if edges[yw+kx] != 0 {
					horiz[yw+x] = 255
					break
				}
			}
		}
	}

	radiusY := kh / 2
	for x := range width {
		for y := range height {
			start := max(y-radiusY, 0)
			end := min(y+kh-1-radiusY, height-1)

			for ky := start; ky <= end; ky++ {
				if horiz[ky*width+x] != 0 {
					out[y*width+x] = 255
					break
				}
			}
		}
	}

	return out
}

// connectedComponents labels 8-connected blobs by breadth-first search, in
// raster order of their first pixel, and records each blob's bounding box.
//
//nolint:varnamelen,mnd // pixel kernel: coordinates and kernel coefficients read as the algorithm writes them
func connectedComponents(dilated []uint8, width, height int) []component {
	visited := make([]bool, width*height)
	queue := make([]int32, 0, 1024)
	comps := make([]component, 0, 256)

	for y := range height {
		yw := y * width

		for x := range width {
			idx := yw + x
			if dilated[idx] == 0 || visited[idx] {
				continue
			}

			comp := component{
				minX: x, maxX: x, minY: y, maxY: y,
				parent: -1, firstChild: -1, nextSibling: -1,
			}

			visited[idx] = true
			queue = append(queue[:0], int32(idx))

			for head := 0; head < len(queue); head++ {
				curr := int(queue[head])
				cx, cy := curr%width, curr/width

				comp.minX = min(comp.minX, cx)
				comp.maxX = max(comp.maxX, cx)
				comp.minY = min(comp.minY, cy)
				comp.maxY = max(comp.maxY, cy)

				for k := range 8 {
					nx, ny := cx+dx8[k], cy+dy8[k]
					if nx < 0 || nx >= width || ny < 0 || ny >= height {
						continue
					}

					nidx := ny*width + nx
					if dilated[nidx] != 0 && !visited[nidx] {
						visited[nidx] = true
						queue = append(queue, int32(nidx))
					}
				}
			}

			comps = append(comps, comp)
		}
	}

	return comps
}

// buildHierarchy links each component to the smallest other component whose
// box encloses it, with one pixel of slack on every side. A parent must be
// strictly larger than its child, which keeps the parent links acyclic so the
// child walk in filterTargets always ends. Dilated blobs are thick and
// 4-connected, so two disjoint ones cannot share a box within the slack; the
// rule is a guard on that argument, not a case a frame is known to reach.
//
//nolint:varnamelen // i and j index two components being compared
func buildHierarchy(comps []component) {
	for i := range comps {
		bestParent := -1
		bestArea := int64(-1)
		ownArea := comps[i].area()

		for j := range comps {
			if i == j {
				continue
			}

			c, p := &comps[i], &comps[j]
			if p.minX <= c.minX+1 && p.maxX >= c.maxX-1 &&
				p.minY <= c.minY+1 && p.maxY >= c.maxY-1 {
				area := p.area()
				if area > ownArea && (bestArea < 0 || area < bestArea) {
					bestArea = area
					bestParent = j
				}
			}
		}

		comps[i].parent = bestParent
		if bestParent >= 0 {
			comps[i].nextSibling = comps[bestParent].firstChild
			comps[bestParent].firstChild = i
		}
	}
}

// filterTargets projects component boxes into logical pixels and applies
// wl-kbptr's filter_rects heuristics, returning the surviving rectangles in
// component order.
//
//nolint:varnamelen,mnd // pixel kernel: coordinates and kernel coefficients read as the algorithm writes them
func filterTargets(comps []component, scale float64) []image.Rectangle {
	n := len(comps)
	filtered := make([]bool, n)
	rx := make([]float64, n)
	ry := make([]float64, n)
	rw := make([]float64, n)
	rh := make([]float64, n)

	for i, c := range comps {
		rx[i] = float64(c.minX) / scale
		ry[i] = float64(c.minY) / scale
		rw[i] = float64(c.maxX-c.minX+1) / scale
		rh[i] = float64(c.maxY-c.minY+1) / scale

		// Buttons and small inputs are typically under 50px high, but
		// notification cards, toasts and floating dialogs can reach 160px high
		// and 650px wide. Below the minimums is noise.
		if rh[i] >= maxTargetHeight || rw[i] >= maxTargetWidth ||
			rh[i] <= minTargetHeight || rw[i] <= minTargetWidth {
			filtered[i] = true
		}
	}

	// Walk every nested component depth-first from the ones with a parent,
	// deduplicating against the enclosing box.
	toExplore := make([]int, 0, n)
	for i, c := range comps {
		if c.parent >= 0 {
			toExplore = append(toExplore, i)
		}
	}

	for len(toExplore) > 0 {
		i := toExplore[len(toExplore)-1]
		toExplore = toExplore[:len(toExplore)-1]

		if !filtered[i] {
			p := comps[i].parent

			switch {
			case p >= 0 && filtered[p]:
				// The parent is a layout container (dialog, card), not a
				// button, so children inside it are kept as they are.
			case rh[i] <= flatLineHeight:
				// Flat inner lines, such as hamburger menu strokes.
				filtered[i] = true
			case p >= 0:
				cx := rx[i] + rw[i]/2
				cy := ry[i] + rh[i]/2
				pcx := rx[p] + rw[p]/2
				pcy := ry[p] + rh[p]/2

				// Inner targets sharing the parent's center, and inner detail
				// of square icons, duplicate the parent.
				if (math.Abs(cx-pcx) < sameCenterSlack && math.Abs(cy-pcy) < sameCenterSlack) ||
					(math.Abs(rh[p]-rw[p]) < squareIconSlack &&
						rh[p] < squareIconSize && rw[p] < squareIconSize) {
					filtered[i] = true
				}
			}
		}

		for child := comps[i].firstChild; child >= 0; child = comps[child].nextSibling {
			toExplore = append(toExplore, child)
		}
	}

	// A container (50 <= height < 160) holding button-sized children is a
	// dialog whose buttons are the targets; one holding none is a toast and
	// stays clickable itself.
	for i := range comps {
		if filtered[i] || rh[i] < containerHeight {
			continue
		}

		for child := comps[i].firstChild; child >= 0; child = comps[child].nextSibling {
			if !filtered[child] && rh[child] < containerHeight && rh[child] > flatLineHeight {
				filtered[i] = true
				break
			}
		}
	}

	rects := make([]image.Rectangle, 0, n)
	for i := range comps {
		if filtered[i] {
			continue
		}

		x, y := int(math.Round(rx[i])), int(math.Round(ry[i]))

		w, h := int(math.Round(rw[i])), int(math.Round(rh[i]))
		if w <= 0 || h <= 0 {
			continue
		}

		rects = append(rects, image.Rect(x, y, x+w, y+h))
	}

	return rects
}

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}
