//go:build integration && darwin

package vision

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"
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

// TestReadTextSample_ReadsEveryScriptWithAModelOfItsOwn pins the sample the
// daemon reads to have Vision compile its models before the first vim
// session. Arabic and Devanagari each have a model compiled only once a read
// meets them, so a sample that drew either as missing glyphs would leave its
// compile to a later read.
func TestReadTextSample_ReadsEveryScriptWithAModelOfItsOwn(t *testing.T) {
	img, err := textSample()
	if err != nil {
		t.Fatalf("draw the sample: %v", err)
	}

	pieces, err := readSampleWith(img, "")
	if err != nil {
		t.Fatalf("read the sample: %v", err)
	}

	for _, script := range []struct {
		name  string
		table *unicode.RangeTable
	}{
		{"Latin", unicode.Latin},
		{"Arabic", unicode.Arabic},
		{"Devanagari", unicode.Devanagari},
		{"Han", unicode.Han},
	} {
		found := slices.ContainsFunc(pieces, func(piece linePiece) bool {
			return strings.ContainsFunc(
				piece.Text,
				func(r rune) bool { return unicode.Is(script.table, r) },
			)
		})
		if !found {
			t.Errorf("the sample read no %s text", script.name)
		}
	}

	if t.Failed() {
		logSampleDiagnosis(t, img, pieces)
	}
}

// readSampleWith reads img through the helper's request, with language as the
// request's second language, or the user's when it is empty.
func readSampleWith(img *image.RGBA, language string) ([]linePiece, error) {
	request, err := encodeLinesRequest(img, img.Rect, prepareTimeout.Milliseconds(), language)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer

	err = ServeTextLines(context.Background(), request, &out)
	if err != nil {
		return nil, err
	}

	var pieces []linePiece

	err = json.Unmarshal(out.Bytes(), &pieces)

	return pieces, err
}

// logSampleDiagnosis says what the sample read, whether Vision reads
// Devanagari at all when Hindi is asked for by name, and which machine this
// is. The sample is fixed text, so logging what was read of it logs nothing
// of the user's.
func logSampleDiagnosis(t *testing.T, img *image.RGBA, pieces []linePiece) {
	t.Helper()

	for _, piece := range pieces {
		t.Logf("read line %d: %q", piece.Line, piece.Text)
	}

	hindi, err := readSampleWith(img, "hi-IN")
	if err != nil {
		t.Logf("reading with hi-IN failed: %v", err)
	}

	for _, piece := range hindi {
		t.Logf("read with hi-IN, line %d: %q", piece.Line, piece.Text)
	}

	machine, err := exec.CommandContext(
		t.Context(), "sysctl", "-n", "hw.model", "kern.osversion", "machdep.cpu.brand_string",
	).Output()
	t.Logf("machine: %q, err %v", strings.TrimSpace(string(machine)), err)
}
