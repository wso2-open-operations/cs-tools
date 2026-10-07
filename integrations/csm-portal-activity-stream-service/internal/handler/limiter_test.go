package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/stream"
)

func TestConnLimiter_PerUserCap(t *testing.T) {
	l := newConnLimiter(2, 0)
	r1, got := l.acquire("u1")
	if got != admitted {
		t.Fatalf("first acquire = %v, want admitted", got)
	}
	r2, got := l.acquire("u1")
	if got != admitted {
		t.Fatalf("second acquire = %v, want admitted", got)
	}
	if _, got := l.acquire("u1"); got != userLimitReached {
		t.Fatalf("third acquire = %v, want userLimitReached", got)
	}
	// Another user is unaffected by u1's cap.
	r3, got := l.acquire("u2")
	if got != admitted {
		t.Fatalf("other user acquire = %v, want admitted", got)
	}
	r3()

	// Releasing one slot lets one more in; a double release is a no-op.
	r1()
	r1()
	if u, total := l.counts("u1"); u != 1 || total != 1 {
		t.Fatalf("after release counts = (%d, %d), want (1, 1)", u, total)
	}
	r4, got := l.acquire("u1")
	if got != admitted {
		t.Fatalf("acquire after release = %v, want admitted", got)
	}
	r4()
	r2()
	if u, total := l.counts("u1"); u != 0 || total != 0 {
		t.Fatalf("final counts = (%d, %d), want (0, 0)", u, total)
	}
}

func TestConnLimiter_ReplicaCapTakesPrecedence(t *testing.T) {
	l := newConnLimiter(10, 2)
	r1, _ := l.acquire("u1")
	r2, _ := l.acquire("u2")
	if _, got := l.acquire("u3"); got != replicaLimitReached {
		t.Fatalf("acquire at replica cap = %v, want replicaLimitReached", got)
	}
	r1()
	if _, got := l.acquire("u3"); got != admitted {
		t.Fatalf("acquire after release = %v, want admitted", got)
	}
	r2()
}

func TestConnLimiter_ZeroDisablesDimension(t *testing.T) {
	l := newConnLimiter(0, 0)
	for i := 0; i < 50; i++ {
		if _, got := l.acquire("u1"); got != admitted {
			t.Fatalf("acquire #%d = %v, want admitted with limits disabled", i, got)
		}
	}
}

func TestConnLimiter_Concurrent(t *testing.T) {
	l := newConnLimiter(0, 0)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, _ := l.acquire("u")
			release()
		}()
	}
	wg.Wait()
	if u, total := l.counts("u"); u != 0 || total != 0 {
		t.Fatalf("counts after concurrent acquire/release = (%d, %d), want (0, 0)", u, total)
	}
}

// openStreams starts n streams for user and waits until each has committed
// its headers. The returned cancel ends them all.
func openStreams(t *testing.T, h *StreamHandler, user *middleware.UserInfo, n int) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ctx = middleware.WithUserInfo(ctx, user)
	recorders := make([]*syncRecorder, 0, n)
	for i := 0; i < n; i++ {
		req := httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil).WithContext(ctx)
		req.SetPathValue("id", streamTestCaseID)
		w := newSyncRecorder()
		recorders = append(recorders, w)
		go h.StreamCaseActivities(w, req)
	}
	deadline := time.Now().Add(2 * time.Second)
	for _, w := range recorders {
		for !w.started() {
			if time.Now().After(deadline) {
				cancel()
				t.Fatal("stream never started")
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
	return cancel
}

func requestStream(t *testing.T, h *StreamHandler, user *middleware.UserInfo) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil)
	req = req.WithContext(middleware.WithUserInfo(req.Context(), user))
	req.SetPathValue("id", streamTestCaseID)
	w := httptest.NewRecorder()
	h.StreamCaseActivities(w, req)
	return w
}

func TestStreamCaseActivities_PerUserLimit_Returns429(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub(), WithConnectionLimits(2, 0))
	cancel := openStreams(t, h, testUser, 2)
	defer cancel()

	// The same user's third stream is refused before any upstream call.
	w := requestStream(t, h, testUser)
	assertStatus(t, w, http.StatusTooManyRequests)
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 response missing Retry-After")
	}
	if ct := w.Header().Get("Content-Type"); ct == "text/event-stream" {
		t.Error("stream headers were written for a refused connection")
	}

	// A different user is still admitted (and we end it right away).
	other := &middleware.UserInfo{Email: "other@example.com", UserID: "uid-other"}
	otherCancel := openStreams(t, h, other, 1)
	otherCancel()
}

func TestStreamCaseActivities_ReplicaLimit_Returns503(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub(), WithConnectionLimits(0, 1))
	cancel := openStreams(t, h, testUser, 1)
	defer cancel()

	other := &middleware.UserInfo{Email: "other@example.com", UserID: "uid-other"}
	w := requestStream(t, h, other)
	assertStatus(t, w, http.StatusServiceUnavailable)
	if w.Header().Get("Retry-After") == "" {
		t.Error("503 response missing Retry-After")
	}
}

func TestStreamCaseActivities_SlotReleasedWhenStreamEnds(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub(), WithConnectionLimits(1, 0))
	cancel := openStreams(t, h, testUser, 1)
	if w := requestStream(t, h, testUser); w.Code != http.StatusTooManyRequests {
		t.Fatalf("second stream status = %d, want 429 while the first is open", w.Code)
	}
	cancel()

	// Once the first stream has ended its slot is free again.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if u, _ := h.limiter.counts(testUser.UserID); u == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("slot not released after the stream ended")
		}
		time.Sleep(2 * time.Millisecond)
	}
	cancel2 := openStreams(t, h, testUser, 1)
	cancel2()
}

func TestStreamCaseActivities_RefusedBeforeUpstreamCall(t *testing.T) {
	var calls int
	var mu sync.Mutex
	client := &mockEntityCaseClient{getCaseFn: func(ctx context.Context, caseID string) ([]byte, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return []byte(`{}`), nil
	}}
	h := NewStreamHandler(client, stream.NewBroadcastHub(), WithConnectionLimits(1, 0))
	cancel := openStreams(t, h, testUser, 1)
	defer cancel()

	w := requestStream(t, h, testUser)
	assertStatus(t, w, http.StatusTooManyRequests)
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Errorf("GetCase called %d times, want 1 (the refused request must not reach upstream)", calls)
	}
	if !strings.Contains(w.Body.String(), "message") {
		t.Errorf("429 body = %q, want the JSON error envelope", w.Body.String())
	}
}
