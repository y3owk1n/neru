//go:build linux

package linux

import "image"

// wl_output.transform values: a quarter turn count in the low two bits, and a
// horizontal flip applied before the turn in the third.
const (
	outputTransformTurns   = 3
	outputTransformFlipped = 4

	quarterTurn       = 1
	halfTurn          = 2
	threeQuarterTurns = 3
)

// orientToOutput returns a wlr-screencopy frame the way it appears on screen.
// The compositor hands the frame over in the output's buffer orientation, so on
// an output with a wl_output.transform the image is turned or mirrored relative
// to what the user sees. A normal output's frame comes back unchanged.
func orientToOutput(img *image.RGBA, transform int) *image.RGBA {
	turns := transform & outputTransformTurns
	flipped := transform&outputTransformFlipped != 0

	if turns == 0 && !flipped {
		return img
	}

	bufferWidth := img.Rect.Dx()
	bufferHeight := img.Rect.Dy()

	width, height := bufferWidth, bufferHeight
	if turns == quarterTurn || turns == threeQuarterTurns {
		width, height = bufferHeight, bufferWidth
	}

	out := image.NewRGBA(image.Rect(0, 0, width, height))

	for row := range height {
		for col := range width {
			sourceX, sourceY := col, row
			if flipped {
				sourceX = width - 1 - col
			}

			switch turns {
			case quarterTurn:
				sourceX, sourceY = sourceY, width-1-sourceX
			case halfTurn:
				sourceX, sourceY = width-1-sourceX, height-1-sourceY
			case threeQuarterTurns:
				sourceX, sourceY = height-1-sourceY, sourceX
			}

			from := img.PixOffset(img.Rect.Min.X+sourceX, img.Rect.Min.Y+sourceY)
			to := out.PixOffset(col, row)
			copy(out.Pix[to:to+4], img.Pix[from:from+4])
		}
	}

	return out
}
