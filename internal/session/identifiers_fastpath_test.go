package session

import (
    "encoding/json"
    "strings"
    "testing"
)

func referenceConversationID(body []byte) string {
    var obj map[string]any
    if json.Unmarshal(body, &obj) != nil { return "" }
    if meta, ok := obj["metadata"].(map[string]any); ok {
        if v, ok := meta["conversation_id"].(string); ok && v != "" { return v }
        if v, ok := meta["conversationId"].(string); ok && v != "" { return v }
    }
    if v, ok := obj["conversation_id"].(string); ok && v != "" { return v }
    if v, ok := obj["conversationId"].(string); ok && v != "" { return v }
    return ""
}

func TestExtractIdentifiersMatchesBothLegacyContracts(t *testing.T) {
    giant := strings.Repeat("x", 1024*1024)
    cases := []string{
        `{"metadata":{"conversation_id":"meta","conversationId":"camel"},"prompt_cache_key":"cache"}`,
        `{"metadata":{"conversation_id":42,"conversationId":"meta"},"conversation_id":"root"}`,
        `{"metadata":{"conversation_id":null},"conversation_id":"root"}`,
        `{"metadata":null,"prompt_cache_key":"pck"}`,
        `{"metadata":123,"conversationId":"camel"}`,
        `{"conversation_id":false,"conversationId":"camel","prompt_cache_key":"cache"}`,
        `{"prompt_cache_key":"pck"}`,
        `{"messages":[{"role":"user","content":"` + giant + `"}],"conversation_id":"large"}`,
        `{"messages":[{"role":"user","content":"` + giant + `"}],"prompt_cache_key":"cache"}`,
        `{"messages":[invalid]}`,
        `{"metadata":{"conversation_id":"x"}`,
    }
    for i, value := range cases {
        body := []byte(value)
        affinity, conversation := ExtractIdentifiers(body)
        if want := referenceExtractKey(body); affinity != want {
            t.Fatalf("case %d affinity: got %q want %q", i, affinity, want)
        }
        if want := referenceConversationID(body); conversation != want {
            t.Fatalf("case %d conversation: got %q want %q", i, conversation, want)
        }
    }
}

func BenchmarkIdentifiersOriginalTwoPass1MiB(b *testing.B) {
    body := largeSessionBody()
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = referenceExtractKey(body)
        _ = referenceConversationID(body)
    }
}

func BenchmarkIdentifiersOptimizedSinglePass1MiB(b *testing.B) {
    body := largeSessionBody()
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = ExtractIdentifiers(body)
    }
}
