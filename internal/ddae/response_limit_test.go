package ddae

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecodeBoundedRejectsOversizeAndTrailingJSON(t *testing.T) {
	var value PingResponse
	if err := decodeBounded(strings.NewReader(`{"status":"ok"}`), 4, &value); err == nil {
		t.Fatal("expected size error")
	}
	if err := decodeBounded(strings.NewReader(`{"status":"ok"} {}`), 64, &value); err == nil {
		t.Fatal("expected trailing JSON error")
	}
}

func TestDecodeBoundedRejectsMalformedTruncatedAndDeepJSON(t *testing.T) {
	for _, input := range []string{``, `{"status":`, `{"status":"ok"} garbage`} {
		var value PingResponse
		if err := decodeBounded(strings.NewReader(input), 128, &value); err == nil {
			t.Fatalf("accepted malformed input %q", input)
		}
	}
	deep := bytes.Repeat([]byte("["), 10001)
	deep = append(deep, bytes.Repeat([]byte("]"), 10001)...)
	var destination any
	if err := decodeBounded(bytes.NewReader(deep), int64(len(deep)+1), &destination); err == nil {
		t.Fatal("accepted JSON beyond decoder nesting limit")
	}
}
