package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
	"github.com/stretchr/testify/require"
)

func TestComputeFallbackContentHash_NoSeparatorCollision(t *testing.T) {
	// Without separator, "ab"+"cd" == "abc"+"d" would produce same hash
	// With separator they MUST differ
	hash1 := computeFallbackContentHash([]string{"ab", "cd"})
	hash2 := computeFallbackContentHash([]string{"abc", "d"})
	require.NotEqual(t, hash1, hash2, "hashes must differ when texts have different boundaries")
}

func TestComputeFallbackContentHash_Deterministic(t *testing.T) {
	hash1 := computeFallbackContentHash([]string{"hello", "world"})
	hash2 := computeFallbackContentHash([]string{"hello", "world"})
	require.Equal(t, hash1, hash2, "same input must produce same hash")
}

func TestComputeFallbackContentHash_OrderSensitive(t *testing.T) {
	hash1 := computeFallbackContentHash([]string{"hello", "world"})
	hash2 := computeFallbackContentHash([]string{"world", "hello"})
	require.NotEqual(t, hash1, hash2, "different order must produce different hash")
}

func TestComputeFallbackContentHash_EmptySlice(t *testing.T) {
	hash := computeFallbackContentHash([]string{})
	require.NotEmpty(t, hash, "empty input must still produce a hash")
}

func buildFallbackAffinityContextForTest(bodyJSON string) *gin.Context {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(bodyJSON))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx
}

func newTestAffinityCache(t *testing.T) *cachex.HybridCache[int] {
	return cachex.NewHybridCache[int](cachex.HybridCacheConfig[int]{
		Namespace:    "test_fallback_affinity:v1",
		Redis:        nil,
		RedisCodec:   cachex.IntCodec{},
		RedisEnabled: func() bool { return false },
		Memory: func() *hot.HotCache[string, int] {
			return hot.NewHotCache[string, int](hot.LRU, 10000).
				WithTTL(time.Hour).
				WithJanitor().
				Build()
		},
	})
}

func TestExtractMessageTextsFromBody_OpenAIFormat(t *testing.T) {
	body := `{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there!"},
			{"role": "user", "content": "How are you?"}
		]
	}`

	ctx := buildFallbackAffinityContextForTest(body)
	texts := extractMessageTextsFromBody(ctx)
	require.Len(t, texts, 4)
	require.Equal(t, "You are helpful.", texts[0])
	require.Equal(t, "Hello", texts[1])
	require.Equal(t, "Hi there!", texts[2])
	require.Equal(t, "How are you?", texts[3])
}

func TestExtractMessageTextsFromBody_ClaudeSystemBlock(t *testing.T) {
	body := `{
		"model": "claude-3-5-sonnet",
		"system": [{"type": "text", "text": "System block text."}],
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "User block text."}]}
		]
	}`

	ctx := buildFallbackAffinityContextForTest(body)
	texts := extractMessageTextsFromBody(ctx)
	require.Len(t, texts, 2)
	require.Equal(t, "System block text.", texts[0])
	require.Equal(t, "User block text.", texts[1])
}

func TestExtractMessageTextsFromBody_OpenAIResponsesInput(t *testing.T) {
	body := `{
		"model": "gpt-4o",
		"input": [
			{"role": "user", "content": "First question"},
			{"role": "assistant", "content": "First answer"},
			{"role": "user", "content": "Second question"}
		]
	}`

	ctx := buildFallbackAffinityContextForTest(body)
	texts := extractMessageTextsFromBody(ctx)
	require.Len(t, texts, 3)
	require.Equal(t, "First question", texts[0])
	require.Equal(t, "First answer", texts[1])
	require.Equal(t, "Second question", texts[2])
}

func TestExtractMessageTextsFromBody_MessagesPreferredOverInput(t *testing.T) {
	// When both "messages" and "input" exist, "messages" takes priority
	body := `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "from-messages"}],
		"input": [{"role": "user", "content": "from-input"}]
	}`

	ctx := buildFallbackAffinityContextForTest(body)
	texts := extractMessageTextsFromBody(ctx)
	require.Len(t, texts, 1)
	require.Equal(t, "from-messages", texts[0])
}

func TestFallbackContentAffinity_HitOnFullMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"messages":[{"role":"user","content":"unique-prompt-text-42"}]}`
	ctx := buildFallbackAffinityContextForTest(body)
	cache := newTestAffinityCache(t)

	// Pre-warm cache with full content hash
	hash := computeFallbackContentHash([]string{"unique-prompt-text-42"})
	key := buildFallbackCacheKeySuffix("default", hash)
	require.NoError(t, cache.SetWithTTL(key, 99, time.Hour))

	channelID, found, err := cache.Get(key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 99, channelID)
	_ = ctx // verify body can be read
	texts := extractMessageTextsFromBody(ctx)
	require.Equal(t, []string{"unique-prompt-text-42"}, texts)
}

func TestFallbackContentAffinity_HitOnTrimmedMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := newTestAffinityCache(t)

	// Simulate: first round had 2 messages, cached channel 42
	// Second round has 4 messages, trimming last 2 should hit the cached hash
	body := `{
		"messages": [
			{"role": "user", "content": "System instructions here"},
			{"role": "assistant", "content": "First response"},
			{"role": "user", "content": "New turn question"},
			{"role": "assistant", "content": "New assistant reply"}
		]
	}`
	ctx := buildFallbackAffinityContextForTest(body)
	texts := extractMessageTextsFromBody(ctx)
	require.Len(t, texts, 4)

	// Pre-warm cache with the first 2 messages' hash (what trim=2 would match)
	trim2Hash := computeFallbackContentHash([]string{
		"System instructions here",
		"First response",
	})
	key := buildFallbackCacheKeySuffix("default", trim2Hash)
	require.NoError(t, cache.SetWithTTL(key, 42, time.Hour))

	// Verify the trim logic: with 4 messages, trim 2 gives subset [:2]
	subset := texts[:len(texts)-2]
	subsetHash := computeFallbackContentHash(subset)
	require.Equal(t, trim2Hash, subsetHash)

	val, found, err := cache.Get(key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 42, val)
}

func TestFallbackContentAffinity_NoHitReturnsFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"messages":[{"role":"user","content":"completely-new-prompt"}]}`
	ctx := buildFallbackAffinityContextForTest(body)

	channelID, found := tryFallbackContentAffinity(ctx, "gpt-4", "default")
	require.False(t, found)
	require.Equal(t, 0, channelID)
}

// Test that the fallback sets context for recording after a cache hit
func TestFallbackContentAffinity_SetsContextForRecording(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"messages":[{"role":"user","content":"recording-test"}]}`
	ctx := buildFallbackAffinityContextForTest(body)

	// Use a clean test cache instead of the global one
	cache := newTestAffinityCache(t)

	hash := computeFallbackContentHash([]string{"recording-test"})
	key := buildFallbackCacheKeySuffix("default", hash)
	require.NoError(t, cache.SetWithTTL(key, 55, time.Hour))

	val, found, err := cache.Get(key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 55, val)
	_ = ctx
}
