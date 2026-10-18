package strings_test

import (
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/strings"
)

func TestCStringIsNULTerminated(t *testing.T) {
	p := strings.CString("abc")
	got := unsafe.Slice(p, 4)
	if string(got) != "abc\x00" {
		t.Fatalf("CString(%q) = %q, want %q", "abc", got, "abc\x00")
	}
}

func TestGoStringStopsAtNUL(t *testing.T) {
	b := []byte("abc\x00def\x00")
	if got := strings.GoString(&b[0]); got != "abc" {
		t.Fatalf("GoString = %q, want %q", got, "abc")
	}
}

func TestGoStringOfNilIsEmpty(t *testing.T) {
	if got := strings.GoString(nil); got != "" {
		t.Fatalf("GoString(nil) = %q, want empty", got)
	}
}
