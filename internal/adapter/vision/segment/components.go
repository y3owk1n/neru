package segment

import "image"

// rimWidth is how many outer rows at the top and bottom of a blob's box are
// read to tell a frame from a glyph cluster. A border marks the pixels on both
// sides of its step and a filled shape only the pixel outside it, so either
// covers one of the two.
const rimWidth = 2

// rgb is one pixel's color, signed so differences need no conversion.
type rgb [3]int32

// blob is one 8-connected run of ink with what classification needs to know
// about it. Bounds are inclusive frame pixels.
type blob struct {
	minX, minY, maxX, maxY int
	count                  int
	// bg is the pixel just left of the blob's first run, and ink is the pixel
	// in the blob farthest from it.
	bg, ink rgb
	inkDist int32
	// top and bottom count ink in each of the rimWidth outer rows. leftRows
	// and rightRows count the rows with ink in a side band half the box's
	// shorter dimension deep, deep enough to hold a pill's rounded end.
	// lastLeft and lastRight are the last row counted, so a row counts once.
	top, bottom         [rimWidth]int
	leftRows, rightRows int
	lastLeft, lastRight int
}

func (b *blob) width() int  { return b.maxX - b.minX + 1 }
func (b *blob) height() int { return b.maxY - b.minY + 1 }

func (b *blob) centerX() int { return (b.minX + b.maxX) / 2 } //nolint:mnd // the midpoint
func (b *blob) centerY() int { return (b.minY + b.maxY) / 2 } //nolint:mnd // the midpoint

// rect is the blob's box as the ink sits on screen. The step mask marks the
// pixel before each step, which is the background pixel on a shape's left and
// top edges and the shape's own pixel on its right and bottom, so the marked
// box starts one pixel early.
func (b *blob) rect() image.Rectangle {
	return image.Rect(b.minX+1, b.minY+1, b.maxX+1, b.maxY+1)
}

// encloses reports whether b's box holds o's box.
func (b *blob) encloses(o *blob) bool {
	return b.minX <= o.minX && b.maxX >= o.maxX && b.minY <= o.minY && b.maxY >= o.maxY
}

// run is one horizontal stretch of ink in one row, with a provisional label.
type run struct {
	x0, x1 int32
	label  int32
}

// components labels 8-connected ink over row runs with union-find, then
// measures every blob in two more passes over the runs: its box, ink and
// background on the first, the ink along each side of the now-known box on the
// second. Blobs are returned in raster order of their first run.
//
//nolint:varnamelen // pixel kernel: coordinates read as the algorithm writes them
func components(img *image.RGBA, mask []bool, width, height int) []blob {
	runs := make([]run, 0, width*height/16) //nolint:mnd // a first guess; append grows it
	rowStart := make([]int32, height+1)
	parent := make([]int32, 0, 1024) //nolint:mnd // a first guess; append grows it

	find := func(a int32) int32 {
		for parent[a] != a {
			parent[a] = parent[parent[a]]
			a = parent[a]
		}

		return a
	}

	prev0, prev1 := 0, 0

	for y := range height {
		rowStart[y] = int32(len(runs))
		row := mask[y*width : (y+1)*width]

		for x := 0; x < width; {
			if !row[x] {
				x++

				continue
			}

			x0 := x
			for x < width && row[x] {
				x++
			}

			cur := run{x0: int32(x0), x1: int32(x - 1), label: -1}

			for _, above := range runs[prev0:prev1] {
				if above.x0 > cur.x1+1 || cur.x0 > above.x1+1 {
					continue
				}

				root := find(above.label)

				switch {
				case cur.label < 0:
					cur.label = root
				case root != cur.label:
					lo, hi := min(root, cur.label), max(root, cur.label)
					parent[hi] = lo
					cur.label = lo
				}
			}

			if cur.label < 0 {
				cur.label = int32(len(parent))
				parent = append(parent, cur.label)
			}

			runs = append(runs, cur)
		}

		prev0, prev1 = int(rowStart[y]), len(runs)
	}

	rowStart[height] = int32(len(runs))

	index := make([]int32, len(parent))
	for i := range index {
		index[i] = -1
	}

	blobs := make([]blob, 0, len(parent))

	for y := range height {
		pix := img.Pix[y*img.Stride:]

		for r := rowStart[y]; r < rowStart[y+1]; r++ {
			cur := &runs[r]
			root := find(cur.label)
			cur.label = root

			bi := index[root]
			if bi < 0 {
				bi = int32(len(blobs))
				index[root] = bi
				left := max(int(cur.x0)-1, 0) * 4 //nolint:mnd // four bytes per RGBA pixel
				blobs = append(blobs, blob{
					minX: int(cur.x0), maxX: int(cur.x1), minY: y, maxY: y,
					bg:       rgb{int32(pix[left]), int32(pix[left+1]), int32(pix[left+2])},
					inkDist:  -1,
					lastLeft: -1, lastRight: -1,
				})
			}

			b := &blobs[bi]
			b.minX, b.maxX, b.maxY = min(b.minX, int(cur.x0)), max(b.maxX, int(cur.x1)), y
			b.count += int(cur.x1 - cur.x0 + 1)

			for x := cur.x0; x <= cur.x1; x++ {
				i := int(x) * 4 //nolint:mnd // four bytes per RGBA pixel
				px := rgb{int32(pix[i]), int32(pix[i+1]), int32(pix[i+2])}

				if d := distance(px, b.bg); d > b.inkDist {
					b.inkDist, b.ink = d, px
				}
			}
		}
	}

	for y := range height {
		for r := rowStart[y]; r < rowStart[y+1]; r++ {
			cur := runs[r]
			b := &blobs[index[cur.label]]
			n := int(cur.x1 - cur.x0 + 1)

			if k := y - b.minY; k < rimWidth {
				b.top[k] += n
			}

			if k := b.maxY - y; k < rimWidth {
				b.bottom[k] += n
			}

			//nolint:mnd // half the shorter side
			depth := int32(max(rimWidth, min(b.width(), b.height())/2))

			if cur.x0 < int32(b.minX)+depth && b.lastLeft != y {
				b.leftRows++
				b.lastLeft = y
			}

			if cur.x1 > int32(b.maxX)-depth && b.lastRight != y {
				b.rightRows++
				b.lastRight = y
			}
		}
	}

	return blobs
}

// distance is the sum of absolute channel differences.
//
//nolint:varnamelen // the two colors compared
func distance(from, to rgb) int32 {
	var sum int32

	for k := range from {
		if from[k] > to[k] {
			sum += from[k] - to[k]
		} else {
			sum += to[k] - from[k]
		}
	}

	return sum
}
