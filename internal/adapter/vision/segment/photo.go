package segment

import "image"

// Photo handling. A photograph read with the UI threshold is texture from
// edge to edge, one dense blob that would swallow whatever is drawn on it. A
// dense blob too big to be one target is therefore read again with a contrast
// threshold high enough that only what sits on top of it (desktop icons and
// their captions, a hero's buttons, a player's controls) still counts as ink.
const (
	// overlayStep is the per-channel step that still counts as ink on a photo.
	overlayStep = 56
	// photoFill is how dense a blob too big to be one target must be to be a
	// photo. Layout containers are outlines, far sparser; a wallpaper with a
	// window cut out of it is not as dense as a thumbnail.
	photoFill = 0.5
)

// densePhotos returns the boxes, in mask coordinates, of blobs dense enough to
// be photos and too big to be one target.
func densePhotos(blobs []blob, lim limits) []image.Rectangle {
	var regions []image.Rectangle

	for i := range blobs {
		b := &blobs[i]
		w, h := b.width(), b.height()

		if (w > lim.maxPictureSide || h > lim.maxPictureSide) &&
			float64(b.count) >= photoFill*float64(w*h) {
			regions = append(regions, image.Rect(b.minX, b.minY, b.maxX+1, b.maxY+1))
		}
	}

	return regions
}

// remaskPhotos marks each photo region again with only the steps of at least
// overlayStep, which is what text, icons and controls drawn on a photo have
// and its own texture mostly does not. Regions are in mask coordinates.
func remaskPhotos(steps []uint8, mask []bool, width int, regions []image.Rectangle) {
	for _, region := range regions {
		for y := region.Min.Y; y < region.Max.Y; y++ {
			for x := region.Min.X; x < region.Max.X; x++ {
				mask[y*width+x] = steps[y*width+x] >= overlayStep
			}
		}
	}
}
