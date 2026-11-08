package upstream

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadSSELineFastpathPreservesLargeFramesAndEOF(t *testing.T) {
	tests := []struct {
		name string
		input string
		want string
		wantEOF bool
	}{
		{"short", "data: hello\n", "data: hello\n", false},
		{"large", "data: " + strings.Repeat("x", 128*1024) + "\n", "data: " + strings.Repeat("x", 128*1024) + "\n", false},
		{"unterminated", "data: tail", "data: tail", true},
		{"empty", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readSSELine(bufio.NewReaderSize(strings.NewReader(tc.input), 64*1024))
			if got != tc.want {
				t.Fatalf("readSSELine lost bytes: got=%d want=%d", len(got), len(tc.want))
			}
			if tc.wantEOF && err != io.EOF {
				t.Fatalf("expected EOF, got %v", err)
			}
			if !tc.wantEOF && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestStreamHintPreservesLargeSingleEvent(t *testing.T) {
	content := strings.Repeat("a", 128*1024)
	frame := fmt.Sprintf(
		"data: {\"id\":\"abc\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"%s\"}}]}\n\ndata: [DONE]\n\n",
		content)
	out := httptest.NewRecorder()
	if err := StreamHint(out, strings.NewReader(frame), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Body.String(), content) {
		t.Fatal("large event content was truncated")
	}
	if strings.Count(out.Body.String(), "data: [DONE]") != 1 {
		t.Fatal("stream must contain exactly one [DONE]")
	}
}

type benchDiscardWriter struct { h http.Header }

func (w *benchDiscardWriter) Header() http.Header {
	if w.h == nil { w.h = make(http.Header) }
	return w.h
}
func (*benchDiscardWriter) Write(p []byte) (int, error) { return io.Discard.Write(p) }
func (*benchDiscardWriter) WriteHeader(int) {}
func (*benchDiscardWriter) Flush() {}

// BenchmarkStreamHint340Frames exercises 340 small SSE frames per iteration.
// Each frame is one synthetic delta, NOT a measured upstream generated token.
func BenchmarkStreamHint340Frames(b *testing.B) {
	var src strings.Builder
	for i := 0; i < 340; i++ {
		src.WriteString("data: {\"id\":\"r1\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"x\"}}]}\n\n")
	}
	src.WriteString("data: [DONE]\n\n")
	payload := src.String()
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := &benchDiscardWriter{}
		if err := StreamHint(w, strings.NewReader(payload), nil); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(340*float64(b.N)/b.Elapsed().Seconds(), "frames/s")
}
