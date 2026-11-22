//go:build darwin || linux || windows

package vision

import (
	"context"
	"errors"
	"image"
	"math"
	"strings"
	"sync"

	"github.com/y3owk1n/yomi"
	"github.com/y3owk1n/yomi/models/caucasian"
	"github.com/y3owk1n/yomi/models/cjk"
	"github.com/y3owk1n/yomi/models/indic"
	"github.com/y3owk1n/yomi/models/router"
	"github.com/y3owk1n/yomi/models/rtl"
	"github.com/y3owk1n/yomi/models/sea"
	"github.com/y3owk1n/yomi/models/western"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// yomiEngine is the text recognizer, loaded on first use. It has every yomi
// recognizer and the router, which picks the recognizer for each line, so no
// setting or locale chooses the languages it reads.
var yomiEngine = sync.OnceValues(func() (*yomi.Engine, error) {
	loads := []func() (*yomi.Model, error){
		cjk.Model,
		rtl.Model,
		indic.Model,
		sea.Model,
		caucasian.Model,
	}
	opts := make([]yomi.Option, 0, len(loads)+1)

	for _, load := range loads {
		model, err := load()
		if err != nil {
			return nil, derrors.Wrap(err, derrors.CodeInternal, "loading a text recognizer")
		}

		opts = append(opts, yomi.WithModel(model))
	}

	route, err := router.Model()
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeInternal, "loading the script router")
	}

	base, err := western.Model()
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeInternal, "loading a text recognizer")
	}

	return yomi.New(base, append(opts, yomi.WithRouter(route))...), nil
})

// wholeScaleSlack is how far past a whole scale a frame can be and still read
// as it, which covers the rounding in a capture's size.
const wholeScaleSlack = 0.2

// yomiScale is the whole-number scale yomi reads img at, img being a capture
// of region whose pixels are the region's points times the display's scale,
// as on macOS and Linux. A Wayland output at a fractional scale hands back
// frames at 1.25 or 1.5 pixels per point, and yomi reads them best as 2x. A
// 1.5x frame keeps 97.8% of words that way against 88.2% read as 1x, and a
// 1.25x one 96.5% against 95.3%.
func yomiScale(img *image.RGBA, region image.Rectangle) int {
	if region.Dy() <= 0 {
		return 1
	}

	return wholeScale(float64(img.Rect.Dy()) / float64(region.Dy()))
}

// wholeScale rounds a display scale up to the whole one yomi reads at, unless
// it is within wholeScaleSlack of the one below.
func wholeScale(scale float64) int {
	return max(1, int(math.Ceil(scale-wholeScaleSlack)))
}

// readYomi reads img, whose text is drawn at scale pixels per point, and
// returns its lines with boxes in img's pixels, origin at the top-left of the
// buffer.
func readYomi(ctx context.Context, img *image.RGBA, scale int) ([]yomi.Line, error) {
	engine, err := yomiEngine()
	if err != nil {
		return nil, err
	}

	lines, err := engine.Read(ctx, img, scale)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, derrors.Wrap(err, derrors.CodeTimeout, "text recognition ran out of time")
		}

		return nil, derrors.Wrap(err, derrors.CodeContextCanceled, "text recognition canceled")
	}

	for i := range lines {
		for j := range lines[i].Words {
			word := &lines[i].Words[j]
			word.Box = scaleRect(word.Box, scale)

			for k := range word.Chars {
				word.Chars[k] = scaleRect(word.Chars[k], scale)
			}
		}
	}

	return lines, nil
}

func scaleRect(r image.Rectangle, scale int) image.Rectangle {
	return image.Rect(r.Min.X*scale, r.Min.Y*scale, r.Max.X*scale, r.Max.Y*scale)
}

// yomiWords gives lines as recognized words, one per word when wordLevel asks
// for it and otherwise one per line, as `neru hints --split-word` means. A
// line's text is its words in screen order, its confidence its least sure
// word's.
func yomiWords(lines []yomi.Line, wordLevel bool) []recognizedWord {
	var words []recognizedWord

	for _, line := range lines {
		if wordLevel {
			for _, word := range line.Words {
				words = append(words, recognizedWord{
					Text:       word.Text,
					Bounds:     word.Box,
					Confidence: float64(word.Confidence),
				})
			}

			continue
		}

		texts := make([]string, len(line.Words))
		bounds := line.Words[0].Box
		confidence := line.Words[0].Confidence

		for i, word := range line.Words {
			texts[i] = word.Text
			bounds = bounds.Union(word.Box)
			confidence = min(confidence, word.Confidence)
		}

		words = append(words, recognizedWord{
			Text:       strings.Join(texts, " "),
			Bounds:     bounds,
			Confidence: float64(confidence),
		})
	}

	return words
}

// yomiSymbols gives lines as one symbol per character, in reading order
// within each word, for piecesFromSymbols.
func yomiSymbols(lines []yomi.Line) []lineSymbol {
	var symbols []lineSymbol

	for lineIndex, line := range lines {
		for wordIndex, word := range line.Words {
			for charIndex, char := range []rune(word.Text) {
				symbols = append(symbols, lineSymbol{
					text: string(char),
					ink:  word.Chars[charIndex],
					line: lineIndex,
					word: wordIndex,
				})
			}
		}
	}

	return symbols
}

// readYomiLines reads img, a capture of region with its text at scale pixels
// per point, as ReadLines pieces in the global coordinates of region.
func readYomiLines(
	ctx context.Context,
	img *image.RGBA,
	region image.Rectangle,
	scale int,
) ([][]ports.TextPiece, error) {
	found, err := readYomi(ctx, img, scale)
	if err != nil {
		return nil, err
	}

	return piecesFromSymbols(yomiSymbols(found), img, region), nil
}
