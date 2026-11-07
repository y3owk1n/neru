//go:build integration && darwin

package vision

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestServeTextLines_ReadsTheRequestedImage pins the helper the daemon reads
// text lines accurately with. A request carrying a fixture's pixels comes
// back as its lines, at the global rect the request says the pixels show.
func TestServeTextLines_ReadsTheRequestedImage(t *testing.T) {
	frame := loadFixtureFrame(
		t,
		filepath.Join("..", "platform", "testdata", "scripts", "cjk-16pt-2x.png"),
	)
	shows := image.Rect(1000, 500, 1000+frame.Rect.Dx()/2, 500+frame.Rect.Dy()/2)

	request, err := encodeLinesRequest(frame, shows, readLinesTestTimeoutMS, "")
	if err != nil {
		t.Fatalf("encode the request: %v", err)
	}

	var out bytes.Buffer

	err = ServeTextLines(context.Background(), request, &out)
	if err != nil {
		t.Fatalf("serve the request: %v", err)
	}

	var pieces []linePiece

	err = json.Unmarshal(out.Bytes(), &pieces)
	if err != nil {
		t.Fatalf("the answer is not JSON pieces: %v", err)
	}

	lines := groupLines(pieces)
	if len(lines) != len(cjkInk) {
		t.Fatalf("read %d lines, want %d", len(lines), len(cjkInk))
	}

	var first strings.Builder
	for _, piece := range lines[0] {
		first.WriteString(piece.Text)

		if !piece.Bounds.In(shows) {
			t.Fatalf(
				"%q is at %v, outside the rect the request shows, %v",
				piece.Text,
				piece.Bounds,
				shows,
			)
		}
	}

	//nolint:gosmopolitan
	if want := "今天的天气很好，我们去公园散步。"; first.String() != want {
		t.Fatalf("first line read as %q, want %q", first.String(), want)
	}
}

// TestServeTextLines_RefusesAnotherBuildsRequest pins that a helper from a
// binary replaced under a running daemon refuses its request rather than
// misread it.
func TestServeTextLines_RefusesAnotherBuildsRequest(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 4, 4))

	request, err := encodeLinesRequest(frame, image.Rect(0, 0, 2, 2), readLinesTestTimeoutMS, "")
	if err != nil {
		t.Fatalf("encode the request: %v", err)
	}

	raw := request.Bytes()
	copy(raw[len(linesMagic):], "some-other-build")

	err = ServeTextLines(context.Background(), bytes.NewReader(raw), &bytes.Buffer{})
	if err == nil {
		t.Fatal("a request from another build was served")
	}
}

// TestServeTextLines_ReadsAWideWordInALatinLine pins the second read in the
// user's Chinese, Japanese or Korean language. Detection reads a line with one
// model, so a Chinese word in an English line read as Latin noise. That line
// has to come back with the word, while a Korean and a Russian line, which the
// Chinese model would garble, keep detection's read.
func TestServeTextLines_ReadsAWideWordInALatinLine(t *testing.T) {
	frame := loadFixtureFrame(
		t,
		filepath.Join("..", "platform", "testdata", "scripts", "mixed-line-16pt-2x.png"),
	)

	request, err := encodeLinesRequest(
		frame,
		image.Rect(0, 0, frame.Rect.Dx()/2, frame.Rect.Dy()/2),
		readLinesTestTimeoutMS,
		"zh-Hans",
	)
	if err != nil {
		t.Fatalf("encode the request: %v", err)
	}

	var out bytes.Buffer

	err = ServeTextLines(context.Background(), request, &out)
	if err != nil {
		t.Fatalf("serve the request: %v", err)
	}

	var pieces []linePiece

	err = json.Unmarshal(out.Bytes(), &pieces)
	if err != nil {
		t.Fatalf("the answer is not JSON pieces: %v", err)
	}

	lines := groupLines(pieces)
	read := make([]string, 0, len(lines))

	for _, line := range lines {
		var text strings.Builder
		for _, piece := range line {
			text.WriteString(piece.Text)
		}

		read = append(read, text.String())
	}

	//nolint:gosmopolitan
	for _, want := range []string{"你好", "한국어", "Русский"} {
		if !slices.ContainsFunc(
			read,
			func(line string) bool { return strings.Contains(line, want) },
		) {
			t.Errorf("no line read has %q: %q", want, read)
		}
	}
}
