package server

import (
    "strings"
    "testing"
    "time"
)

func TestChatStatModelReuseMatchesOriginal(t *testing.T) {
    for _, tc := range []struct {
        model string
        stream bool
    }{
        {"deepseek-v4.1-flash", true},
        {"glm-5.3-flash", false},
        {"", true},
    } {
        payload := []byte(`{"model":"` + tc.model + `","messages":[{"role":"user","content":"hi"}]}`)
        old := newChatStat(time.Now(), payload, tc.stream, nil)
        modern := newChatStatWithModel(old.start, tc.model, tc.stream, nil)
        if old.model != modern.model || old.mode != modern.mode || old.toks != modern.toks {
            t.Fatalf("metadata changed: %+v vs %+v", old, modern)
        }
    }
}

func largeBenchBody() []byte {
    return []byte(`{"model":"deepseek-v4.1-flash","messages":[{"role":"user","content":"` +
        strings.Repeat("x", 1024*1024) + `"}]}`)
}

func BenchmarkChatStatLegacy1MiB(b *testing.B) {
    body := largeBenchBody()
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = newChatStat(time.Now(), body, true, nil)
    }
}

func BenchmarkChatStatReusedModel1MiB(b *testing.B) {
    body := largeBenchBody()
    _ = body // Same workload context, but model is already known from first decode.
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = newChatStatWithModel(time.Now(), "deepseek-v4.1-flash", true, nil)
    }
}
