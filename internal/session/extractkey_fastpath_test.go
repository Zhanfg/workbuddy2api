package session

import (
    "encoding/json"
    "strings"
    "testing"
)

// This slow reference matches the original map[string]any interpretation.
func referenceExtractKey(body []byte) string {
    if len(body) == 0 { return "" }
    var obj map[string]any
    if json.Unmarshal(body, &obj) != nil { return "" }
    if meta, ok := obj["metadata"].(map[string]any); ok {
        if key, ok := meta["conversation_id"].(string); ok && key != "" { return key }
        if key, ok := meta["conversationId"].(string); ok && key != "" { return key }
    }
    for _, field := range []string{"conversation_id", "conversationId", "prompt_cache_key"} {
        if key, ok := obj[field].(string); ok && key != "" { return key }
    }
    return ""
}

func TestExtractKeyFastpathPreservesFallbacksAndLargeBodies(t *testing.T) {
    long := strings.Repeat("a", 1024*1024)
    cases := []string{
        "{}",
        `{"model":"m","messages":[{"role":"user","content":"` + long + `"}]}`,
        `{"model":"m","messages":[{"role":"user","content":"` + long + `"}],"metadata":{"conversation_id":"session-1"},"conversation_id":"backup"}`,
        `{"metadata":{"conversation_id":123,"conversationId":"camel"},"conversation_id":"root"}`,
        `{"metadata":{"conversation_id":null},"conversation_id":"root"}`,
        `{"metadata":"invalid","conversationId":"root"}`,
        `{"metadata":{"conversation_id":false,"conversationId":123},"prompt_cache_key":"pck"}`,
        `{"conversation_id":123,"conversationId":"other","prompt_cache_key":"last"}`,
        `{"metadata":{"conversation_id":""},"conversation_id":"","prompt_cache_key":"backup"}`,
        `{"metadata":{"conversation_id":"one","conversationId":"two"},"conversation_id":"three"}`,
        `{"metadata":{"user_id":"no-sticky"},"prompt_cache_key":"pck"}`,
        `{"metadata":[],"conversation_id":false}`,
        `{"metadata":null,"prompt_cache_key":0}`,
        `{"metadata":{"conversation_id":"bad"},"messages":[invalid]}`,
        `{"metadata":{"conversation_id":"bad"}`,
    }
    for i, body := range cases {
        got := ExtractKey([]byte(body))
        want := referenceExtractKey([]byte(body))
        if got != want { t.Fatalf("case %d: got %q, want %q", i, got, want) }
    }
}

func largeSessionBody() []byte {
    return []byte(`{"metadata":{"conversation_id":"session"},"messages":[{"role":"user","content":"` +
        strings.Repeat("x", 1024*1024) + `"}]}`)
}

func BenchmarkExtractKeyMasterStyle1MiB(b *testing.B) {
    payload := largeSessionBody()
    b.SetBytes(int64(len(payload)))
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        if referenceExtractKey(payload) != "session" { b.Fatal("reference key lost") }
    }
}

func BenchmarkExtractKeyOptimized1MiB(b *testing.B) {
    payload := largeSessionBody()
    b.SetBytes(int64(len(payload)))
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        if ExtractKey(payload) != "session" { b.Fatal("optimized key lost") }
    }
}
