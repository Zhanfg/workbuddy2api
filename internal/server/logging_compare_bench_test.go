package server

import (
    "io"
    "strings"
    "testing"
    "time"
)

// Comparable benchmark is intentionally source-compatible with the original
// master branch. CI copies this exact file to a detached master worktree, so
// both revisions measure the same reader and fixture, not different tests.
func BenchmarkChatStatsComparable340Frames(b *testing.B) {
    var payload strings.Builder
    for i := 0; i < 340; i++ {
        payload.WriteString("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}],\"id\":\"id\"}\n\n")
    }
    payload.WriteString("data: {\"usage\":{\"completion_tokens\":340,\"prompt_tokens\":100}}\n\n")
    input := payload.String()
    b.ReportAllocs()
    b.SetBytes(int64(len(input)))
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        reader := newChatStatsReaderSince(strings.NewReader(input), time.Now())
        if _, err := io.Copy(io.Discard, reader); err != nil {
            b.Fatal(err)
        }
        if tokens, ok := reader.Tokens(); !ok || tokens != 340 {
            b.Fatal("usage missing")
        }
    }
    b.ReportMetric(340*float64(b.N)/b.Elapsed().Seconds(), "frames/s")
}
