package server

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestChatStatsFastpathPreservesBytesAndUsage(t *testing.T) {
	input := "data: {\"choices\":[{\"delta\":{\"content\":\"usage is plain text\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":340,\"credit\":0,\"prompt_cache_hit_tokens\":7}}\n\n" +
		"data: [DONE]\n\n"
	r := newChatStatsReaderSince(strings.NewReader(input), time.Now().Add(-time.Second))
	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != input {
		t.Fatal("stats reader must forward SSE byte for byte")
	}
	if got, ok := r.Tokens(); !ok || got != 340 {
		t.Fatalf("tokens=(%d,%t), want (340,true)", got, ok)
	}
	if got, ok := r.Credit(); !ok || got != 0 {
		t.Fatalf("explicit zero credit must be recognized, got (%v,%t)", got, ok)
	}
	if r.PromptTokens() != 11 || r.TTFB() <= 0 {
		t.Fatalf("usage/ttfb regression: prompt=%d, ttfb=%v", r.PromptTokens(), r.TTFB())
	}
	hit, _, _ := r.CacheTokens()
	if hit != 7 {
		t.Fatalf("cache-hit=%d, want 7", hit)
	}
}

func TestReadStatsLineLargeAndUnterminated(t *testing.T) {
	large := []byte("data: " + strings.Repeat("x", 128*1024) + "\n")
	for _, input := range [][]byte{[]byte("data: x\n"), large, []byte("data: tail")} {
		reader := bufio.NewReaderSize(bytes.NewReader(input), 64*1024)
		line, err := readStatsLine(reader)
		if !bytes.Equal(line, input) {
			t.Fatalf("data lost: input=%d, got=%d", len(input), len(line))
		}
		if bytes.HasSuffix(input, []byte("\n")) && err != nil {
			t.Fatalf("expected nil on terminated line, got %v", err)
		}
		if !bytes.HasSuffix(input, []byte("\n")) && err != io.EOF {
			t.Fatalf("expected EOF on unterminated line, got %v", err)
		}
	}
}

func TestChatStatsReaderSmallConsumerBuffer(t *testing.T) {
	input := []byte("data: {\"usage\":{\"completion_tokens\":4}}\n\n")
	reader := newChatStatsReaderSince(bytes.NewReader(input), time.Now())
	var result []byte
	buf := make([]byte, 3)
	for {
		n, err := reader.Read(buf)
		result = append(result, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(input, result) {
		t.Fatalf("unexpected stream %q", result)
	}
	if got, ok := reader.Tokens(); !ok || got != 4 {
		t.Fatalf("wrong usage: got %d ok %t", got, ok)
	}
}

func BenchmarkChatStats340Frames(b *testing.B) {
	var src strings.Builder
	for i := 0; i < 340; i++ {
		fmt.Fprintf(&src, "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}],\"id\":\"id\"}\n\n")
	}
	src.WriteString("data: {\"usage\":{\"completion_tokens\":340,\"prompt_tokens\":100}}\n\n")
	input := src.String()
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := newChatStatsReaderSince(strings.NewReader(input), time.Now())
		if _, err := io.Copy(io.Discard, r); err != nil {
			b.Fatal(err)
		}
		if got, ok := r.Tokens(); !ok || got != 340 {
			b.Fatal("usage missing")
		}
	}
	b.ReportMetric(340*float64(b.N)/b.Elapsed().Seconds(), "frames/s")
}
