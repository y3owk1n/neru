//go:build darwin

package vision

/*
#include "../platform/darwin/vision.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image"
	"io"
	"math"
	"os"
	"os/exec"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/buildinfo"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// The accurate read of text lines runs in a helper process of this
// executable, one per read. The first time Vision meets a script, it compiles
// a model for it, which takes seconds. Every other read in that process waits
// for the compile, including the fast reads a key is waiting on. A compile in
// another process of the same executable blocks nothing here, and it leaves
// the model cached under the executable's name for later reads.
//
// The daemon captures the region and writes a header and then the pixels to
// the helper's stdin. The helper writes the pieces it read to stdout as JSON.
// The pixels pass through a pipe to a child process of the same binary and
// user, never through a file, and neither side logs the pixels or the text.

// linesHeader opens a request to the helper. Version is the daemon's build,
// since a binary replaced under a running daemon may read the request
// differently.
type linesHeader struct {
	Magic   [4]byte
	Version [32]byte
	Width   uint32
	Height  uint32
	// The global rect, in points, the pixels show.
	X, Y, W, H float64
	// TimeoutMS bounds the read, compile included.
	TimeoutMS int64
	// Language is the Chinese, Japanese or Korean language to read with
	// too, such as "zh-Hans", or empty for the one the user prefers.
	Language [16]byte
}

// linesMagic opens every request.
var linesMagic = [4]byte{'N', 'R', 'L', '1'}

// linePiece is one piece of a line read, as the helper writes it.
type linePiece struct {
	Text       string  `json:"text"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
	Line       int     `json:"line"`
	Word       int     `json:"word"`
	Background uint32  `json:"background"`
}

// helperWaitDelay bounds the wait for the helper's pipes once it is killed.
const helperWaitDelay = 2 * time.Second

// maxLinesPixels is the most pixels a request may carry, an 8K display at
// two pixels per point, so a garbled header cannot ask for a huge buffer.
const maxLinesPixels = 15360 * 8640

// readLinesInHelper captures region and has the helper read it accurately.
func (a *Adapter) readLinesInHelper(
	ctx context.Context,
	region image.Rectangle,
	timeout C.int64_t,
) ([][]ports.TextPiece, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, derrors.Wrap(
			err,
			derrors.CodeInternal,
			"failed to find the executable to read text with",
		)
	}

	var captured C.CGRect

	cgImage := C.NeruCaptureRectAtScale(cRect(region), readLinesScale, &captured, timeout)
	if uintptr(cgImage) == 0 {
		return nil, derrors.New(derrors.CodeActionFailed, "failed to capture the region to read")
	}

	img, err := cgImageToRGBA(cgImage)
	C.CGImageRelease(cgImage)

	if err != nil {
		return nil, err
	}

	request, err := encodeLinesRequest(img, image.Rect(
		int(captured.origin.x), int(captured.origin.y),
		int(captured.origin.x+captured.size.width), int(captured.origin.y+captured.size.height),
	), int64(timeout), "")
	if err != nil {
		return nil, err
	}

	var stdout bytes.Buffer

	helper := exec.CommandContext(ctx, executable, a.linesHelper)
	helper.Stdin = request
	helper.Stdout = &stdout
	helper.WaitDelay = helperWaitDelay

	err = helper.Run()
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeActionFailed, "the text reading helper failed")
	}

	var pieces []linePiece

	err = json.Unmarshal(stdout.Bytes(), &pieces)
	if err != nil {
		return nil, derrors.Wrap(
			err,
			derrors.CodeActionFailed,
			"the text reading helper answered garbled",
		)
	}

	return groupLines(pieces), nil
}

// encodeLinesRequest writes the request for img, showing the global rect
// shows, in points, to be read within timeoutMS.
func encodeLinesRequest(
	img *image.RGBA,
	shows image.Rectangle,
	timeoutMS int64,
	language string,
) (*bytes.Buffer, error) {
	header := linesHeader{
		Magic:     linesMagic,
		Version:   versionField(),
		Width:     uint32(img.Rect.Dx()),
		Height:    uint32(img.Rect.Dy()),
		X:         float64(shows.Min.X),
		Y:         float64(shows.Min.Y),
		W:         float64(shows.Dx()),
		H:         float64(shows.Dy()),
		TimeoutMS: timeoutMS,
	}
	copy(header.Language[:], language)

	var request bytes.Buffer

	request.Grow(binary.Size(header) + len(img.Pix))

	err := binary.Write(&request, binary.LittleEndian, header)
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeInternal, "failed to write the read request")
	}

	writeRows(&request, img)

	return &request, nil
}

// writeRows writes img's pixels row by row, without any padding past each.
func writeRows(out *bytes.Buffer, img *image.RGBA) {
	rowBytes := img.Rect.Dx() * bytesPerPixel

	for row := range img.Rect.Dy() {
		start := row * img.Stride
		out.Write(img.Pix[start : start+rowBytes])
	}
}

// ServeTextLines is the helper's side. It reads one request from request,
// reads its pixels accurately and writes the pieces to out as JSON.
func ServeTextLines(_ context.Context, request io.Reader, out io.Writer) error {
	var header linesHeader

	err := binary.Read(request, binary.LittleEndian, &header)
	if err != nil {
		return derrors.Wrap(err, derrors.CodeInvalidInput, "failed to read the request header")
	}

	if header.Magic != linesMagic {
		return derrors.New(derrors.CodeInvalidInput, "the request is not a text lines request")
	}

	if header.Version != versionField() {
		return derrors.New(derrors.CodeInvalidInput, "the request comes from another build of neru")
	}

	pixels := uint64(header.Width) * uint64(header.Height)
	if pixels == 0 || pixels > maxLinesPixels {
		return derrors.New(derrors.CodeInvalidInput, "the request's image size is out of range")
	}

	img := image.NewRGBA(image.Rect(0, 0, int(header.Width), int(header.Height)))

	_, err = io.ReadFull(request, img.Pix)
	if err != nil {
		return derrors.Wrap(err, derrors.CodeInvalidInput, "the request's pixels were cut short")
	}

	pieces, err := readImageLines(img, header)
	if err != nil {
		return err
	}

	err = json.NewEncoder(out).Encode(pieces)
	if err != nil {
		return derrors.Wrap(err, derrors.CodeInternal, "failed to write the pieces read")
	}

	return nil
}

// versionField is this build's version as a request header carries it.
func versionField() [32]byte {
	var version [32]byte

	copy(version[:], buildinfo.Version)

	return version
}

// readImageLines reads img accurately, as showing the global rect header
// names.
func readImageLines(img *image.RGBA, header linesHeader) ([]linePiece, error) {
	var (
		pieces []linePiece
		read   bool
	)

	withCGImage(img, func(cgImage C.CGImageRef, _ C.CGColorSpaceRef) {
		bounds := C.CGRectMake(
			C.CGFloat(header.X), C.CGFloat(header.Y), C.CGFloat(header.W), C.CGFloat(header.H),
		)

		language := C.CString(string(bytes.TrimRight(header.Language[:], "\x00")))
		defer C.free(unsafe.Pointer(language))

		result := C.NeruReadImageLines(
			cgImage,
			bounds,
			1,
			language,
			C.int64_t(max(header.TimeoutMS, 1)),
		)
		if result == nil {
			return
		}
		defer C.NeruFreeTextResult(result)

		pieces, read = linePiecesOf(result), true
	})

	if !read {
		return nil, derrors.New(derrors.CodeActionFailed, "failed to read the image's text")
	}

	return pieces, nil
}

// linePiecesOf copies the pieces out of what the bridge read.
func linePiecesOf(result *C.NeruTextResult) []linePiece {
	native := unsafe.Slice(result.pieces, int(result.count))
	pieces := make([]linePiece, 0, len(native))

	for _, piece := range native {
		pieces = append(pieces, linePiece{
			Text:       C.GoString(piece.text),
			X:          float64(piece.x),
			Y:          float64(piece.y),
			W:          float64(piece.width),
			H:          float64(piece.height),
			Line:       int(piece.line),
			Word:       int(piece.word),
			Background: uint32(piece.background),
		})
	}

	return pieces
}

// groupLines puts pieces into their lines, each with its global bounds.
func groupLines(pieces []linePiece) [][]ports.TextPiece {
	var lines [][]ports.TextPiece

	for _, piece := range pieces {
		for len(lines) <= piece.Line {
			lines = append(lines, nil)
		}

		lines[piece.Line] = append(lines[piece.Line], ports.TextPiece{
			Text: piece.Text,
			Bounds: image.Rect(
				int(math.Round(piece.X)),
				int(math.Round(piece.Y)),
				int(math.Round(piece.X+piece.W)),
				int(math.Round(piece.Y+piece.H)),
			),
			Word:       piece.Word,
			Background: piece.Background,
		})
	}

	return lines
}

// withCGImage hands use img as a CGImage, with the color space it was made
// in, and keeps the pixels alive until use returns.
func withCGImage(img *image.RGBA, use func(C.CGImageRef, C.CGColorSpaceRef)) {
	width, height := img.Rect.Dx(), img.Rect.Dy()
	if width == 0 || height == 0 {
		return
	}

	// The context keeps its buffer across calls, so the pixels are copied
	// into C memory rather than handed over as a Go pointer.
	size := C.size_t(len(img.Pix))
	buffer := C.malloc(size)
	defer C.free(buffer)

	C.memcpy(buffer, unsafe.Pointer(&img.Pix[0]), size)

	space := C.CGColorSpaceCreateDeviceRGB()
	defer C.CGColorSpaceRelease(space)

	context := C.CGBitmapContextCreate(
		buffer,
		C.size_t(width),
		C.size_t(height),
		8,
		C.size_t(img.Stride),
		space,
		C.kCGImageAlphaPremultipliedLast,
	)
	if context == 0 {
		return
	}
	defer C.CGContextRelease(context)

	cgImage := C.CGBitmapContextCreateImage(context)
	if uintptr(cgImage) == 0 {
		return
	}
	defer C.CGImageRelease(cgImage)

	use(cgImage, space)
}
