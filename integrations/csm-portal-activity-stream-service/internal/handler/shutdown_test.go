package handler

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/stream"
)

// TestStreamCaseActivities_ServerShutdownClosesStreams drives a real
// http.Server: with a client mid-stream, Shutdown must return promptly
// (not wait out its deadline) and the client must receive the terminal
// shutdown event.
func TestStreamCaseActivities_ServerShutdownClosesStreams(t *testing.T) {
	hub := stream.NewBroadcastHub()
	h := NewStreamHandler(&mockEntityCaseClient{}, hub)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /cases/{id}/activities/stream", func(w http.ResponseWriter, r *http.Request) {
		h.StreamCaseActivities(w, r.WithContext(middleware.WithUserInfo(r.Context(), testUser)))
	})
	srv := httptest.NewUnstartedServer(mux)
	srv.Config.RegisterOnShutdown(hub.CloseAll)
	srv.Start()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/cases/" + streamTestCaseID + "/activities/stream")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()

	// Make sure the subscription is registered before shutting down: a
	// published event arriving proves the handler is in its loop.
	deadline := time.After(2 * time.Second)
	const payload = `{"caseId":"` + streamTestCaseID + `","type":"case.comment_added"}`
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
registered:
	for {
		hub.Publish(streamTestCaseID, payload)
		select {
		case l := <-lines:
			if strings.HasPrefix(l, "data: ") {
				break registered
			}
		case <-tick.C:
		case <-deadline:
			t.Fatal("stream never delivered an event")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	if err := srv.Config.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v (took %v)", err, time.Since(start))
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("Shutdown took %v, want prompt completion", took)
	}

	var sawEvent, sawData bool
	for l := range lines {
		if l == "event: "+EventShutdown {
			sawEvent = true
		}
		if sawEvent && l == `data: {"reason":"shutdown"}` {
			sawData = true
		}
	}
	if !sawEvent || !sawData {
		t.Errorf("client did not receive the terminal shutdown event (event=%v data=%v)", sawEvent, sawData)
	}
}
