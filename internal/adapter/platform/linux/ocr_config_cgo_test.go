//go:build linux && cgo

package linux

import "testing"

// TestOCRConfig_ReadsASecondLanguageOnAnEngineOfItsOwn pins the engine slot a
// read takes. The fast read a keystroke waits on reads eng in slot 0, and a
// second-language read takes slot 1, whose lock it never shares, so the fast
// read cannot queue behind it.
func TestOCRConfig_ReadsASecondLanguageOnAnEngineOfItsOwn(t *testing.T) {
	tests := []struct {
		name     string
		language string
		engine   int
	}{
		{name: "eng by default", language: "", engine: 0},
		{name: "eng named", language: ocrLanguage, engine: 0},
		{name: "a second language", language: "chi_sim+eng", engine: 1},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			config, free := ocrConfig("/tessdata", OCRParams{Language: testCase.language})
			defer free()

			if int(config.engine) != testCase.engine {
				t.Fatalf("engine slot = %d, want %d", config.engine, testCase.engine)
			}
		})
	}
}
