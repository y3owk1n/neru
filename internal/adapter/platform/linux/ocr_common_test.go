//go:build linux

package linux

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Tesseract resolves its language data at *use*, from a directory no linking
// decision can pin down. Distributions disagree about where tessdata lives, and
// the language pack is a separate package from the library. So the whole of
// "the vision strategy is unavailable because eng.traineddata is not
// installed" is decided here, in Go, where the message can name the file and
// the tests can run without an engine present.

func TestResolveTessdataDir_PrefersTheEnvironmentOverTheSystemPaths(t *testing.T) {
	env := stubEnv{tessdataEnvVar: "/opt/tessdata"}
	exists := stubExists{
		"/opt/tessdata/eng.traineddata":       true,
		"/usr/share/tessdata/eng.traineddata": true,
	}

	dir, err := resolveTessdataDir(env.get, exists.has)
	if err != nil {
		t.Fatalf("resolveTessdataDir: %v", err)
	}

	if dir != "/opt/tessdata" {
		t.Errorf("resolved %q, want the directory TESSDATA_PREFIX names", dir)
	}
}

// TestResolveTessdataDir_AcceptsBothMeaningsOfTessdataPrefix covers the version
// split that trips every tesseract consumer. Tesseract 4 read TESSDATA_PREFIX
// as the *parent* of a tessdata/ directory, tesseract 5 reads it as the
// directory itself, and both spellings are in the wild in shell profiles today.
// Refusing one of them would look like missing language data.
func TestResolveTessdataDir_AcceptsBothMeaningsOfTessdataPrefix(t *testing.T) {
	env := stubEnv{tessdataEnvVar: "/usr/share/tesseract-ocr/5"}
	exists := stubExists{
		tessdataDebianDir + "/" + ocrLanguageData: true,
	}

	dir, err := resolveTessdataDir(env.get, exists.has)
	if err != nil {
		t.Fatalf("resolveTessdataDir: %v", err)
	}

	if dir != tessdataDebianDir {
		t.Errorf("resolved %q, want the tessdata subdirectory", dir)
	}
}

func TestResolveTessdataDir_FallsBackToTheDistributionPaths(t *testing.T) {
	env := stubEnv{}
	exists := stubExists{tessdataDebianDir + "/" + ocrLanguageData: true}

	dir, err := resolveTessdataDir(env.get, exists.has)
	if err != nil {
		t.Fatalf("resolveTessdataDir: %v", err)
	}

	if dir != tessdataDebianDir {
		t.Errorf("resolved %q, want the Debian tessdata directory", dir)
	}
}

// TestResolveTessdataDir_NamesWhatIsMissing is the acceptance criterion in test
// form. A user whose distribution installed the library but not the language
// pack must be told which file is absent, not that "vision failed".
func TestResolveTessdataDir_NamesWhatIsMissing(t *testing.T) {
	dir, err := resolveTessdataDir(stubEnv{}.get, stubExists{}.has)
	if err == nil {
		t.Fatalf("resolveTessdataDir succeeded with %q and no tessdata anywhere", dir)
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("code %q, want CodeNotSupported so callers degrade", derrors.GetCode(err))
	}

	if !strings.Contains(err.Error(), ocrLanguageData) {
		t.Errorf("message %q does not name %q", err.Error(), ocrLanguageData)
	}
}

// TestResolveTessdataDir_IgnoresAnEmptyEnvironmentEntry keeps an exported but
// blank TESSDATA_PREFIX, which a wrapper script can leave behind, from
// resolving to the root directory.
func TestResolveTessdataDir_IgnoresAnEmptyEnvironmentEntry(t *testing.T) {
	env := stubEnv{tessdataEnvVar: ""}
	exists := stubExists{"/usr/share/tessdata/eng.traineddata": true}

	dir, err := resolveTessdataDir(env.get, exists.has)
	if err != nil {
		t.Fatalf("resolveTessdataDir: %v", err)
	}

	if dir != "/usr/share/tessdata" {
		t.Errorf("resolved %q, want the system path", dir)
	}
}

// TestOCRError_MapsEveryStatusToAnActionableSentence pins the half of the
// native contract a Go test can see: which failures a caller should stop
// retrying, and that no status falls through to an empty message.
func TestOCRError_MapsEveryStatusToAnActionableSentence(t *testing.T) {
	tests := []struct {
		status       ocrStatus
		notSupported bool
	}{
		{ocrStatusInit, true},
		{ocrStatusAlloc, false},
		{ocrStatusImage, false},
		{ocrStatusRecognize, false},
		{ocrStatusBusy, false},
		{ocrStatusTimeout, false},
		{ocrStatus(99), false},
	}

	for _, test := range tests {
		err := ocrError(test.status)
		if err == nil {
			t.Errorf("status %d produced no error", test.status)

			continue
		}

		if strings.TrimSpace(err.Error()) == "" {
			t.Errorf("status %d produced an empty message", test.status)
		}

		if got := derrors.IsNotSupported(err); got != test.notSupported {
			t.Errorf(
				"status %d: IsNotSupported=%v, want %v (%v)",
				test.status,
				got,
				test.notSupported,
				err,
			)
		}
	}
}

// TestOCRError_OKIsStillAnError guards the shape every status mapper in this
// package shares. ocrError is only reached on a failure path, so being handed
// the success code means the caller's branch is wrong, and answering nil would
// turn that into a silent success.
func TestOCRError_OKIsStillAnError(t *testing.T) {
	err := ocrError(ocrStatusOK)
	if err == nil {
		t.Fatal("ocrError(ocrStatusOK) returned nil")
	}
}

type stubEnv map[string]string

func (s stubEnv) get(key string) string { return s[key] }

type stubExists map[string]bool

func (s stubExists) has(path string) bool { return s[path] }

func TestWideLanguage_TakesTheFirstChineseJapaneseOrKoreanTheLocaleNames(t *testing.T) {
	const lang = "LANG"

	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "no locale", env: map[string]string{}, want: ""},
		{name: "English", env: map[string]string{lang: "en_US.UTF-8"}, want: ""},
		{
			name: "Simplified Chinese",
			env:  map[string]string{lang: "zh_CN.UTF-8"},
			want: "chi_sim",
		},
		{
			name: "Taiwan reads Traditional",
			env:  map[string]string{lang: "zh_TW.UTF-8"},
			want: "chi_tra",
		},
		{
			name: "Hong Kong reads Traditional",
			env:  map[string]string{"LC_ALL": "zh_HK"},
			want: "chi_tra",
		},
		{name: "Japanese", env: map[string]string{"LC_MESSAGES": "ja_JP.UTF-8"}, want: "jpn"},
		{name: "Korean", env: map[string]string{lang: "ko_KR.UTF-8"}, want: "kor"},
		{
			name: "a later LANGUAGE entry",
			env:  map[string]string{"LANGUAGE": "en_US:ko_KR", lang: "en_US.UTF-8"},
			want: "kor",
		},
		{
			name: "LANGUAGE before LANG",
			env:  map[string]string{"LANGUAGE": "ja", lang: "zh_CN.UTF-8"},
			want: "jpn",
		},
		{
			name: "a language merely starting with ja",
			env:  map[string]string{lang: "jam_JM"},
			want: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := wideLanguage(func(name string) string { return testCase.env[name] })
			if got != testCase.want {
				t.Fatalf("wideLanguage = %q, want %q", got, testCase.want)
			}
		})
	}
}
