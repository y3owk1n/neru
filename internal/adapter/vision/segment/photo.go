package segment

// Photo handling. A photograph read with the UI threshold is texture from
// edge to edge, one dense blob that would swallow whatever is drawn on it. A
// dense blob too big to be one target is therefore read again, its own pixels
// only, with a contrast threshold high enough that only what sits on top of it
// (desktop icons and their captions, a hero's buttons, a player's controls)
// still counts as ink.
const (
	// overlayStep is the per-channel step that still counts as ink on a photo.
	overlayStep = 56
	// photoFill is how dense a blob too big to be one target must be to be a
	// photo. Layout containers are outlines and far sparser. A wallpaper with
	// a window cut out of it is not as dense as a thumbnail.
	photoFill = 0.5
)

// densePhotos marks the blobs dense enough to be photos and too big to be one
// target, and reports whether there were any.
//
//nolint:varnamelen // blob indices and box sides read as the geometry writes them
func densePhotos(blobs []blob, lim limits) ([]bool, bool) {
	photos := make([]bool, len(blobs))
	found := false

	for i := range blobs {
		b := &blobs[i]
		w, h := b.width(), b.height()

		if (w > lim.maxPictureSide || h > lim.maxPictureSide) &&
			float64(b.count) >= photoFill*float64(w*h) {
			photos[i], found = true, true
		}
	}

	return photos, found
}

// remaskPhotos marks the photos' own pixels again with only the steps of at
// least overlayStep, which is what text, icons and controls drawn on a photo
// have and its own texture mostly does not. A window lying on a wallpaper is
// blobs of its own, and keeps its mask.
//
//nolint:varnamelen // pixel kernel: coordinates read as the algorithm writes them
func remaskPhotos(steps []uint8, mask []bool, width int, ink runMap, photos []bool) {
	for y := range len(ink.rowStart) - 1 {
		for _, r := range ink.runs[ink.rowStart[y]:ink.rowStart[y+1]] {
			if !photos[r.label] {
				continue
			}

			for x := int(r.x0); x <= int(r.x1); x++ {
				mask[y*width+x] = steps[y*width+x] >= overlayStep
			}
		}
	}
}
