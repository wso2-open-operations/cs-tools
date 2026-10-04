package eventbus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

// fakeReader serves msgs in order, then returns final (or, when final is
// nil, blocks until ctx is done). It records commits and Close calls.
type fakeReader struct {
	mu        sync.Mutex
	msgs      []kafka.Message
	final     error
	committed []int64
	closed    bool
}

func (f *fakeReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	f.mu.Lock()
	if len(f.msgs) > 0 {
		m := f.msgs[0]
		f.msgs = f.msgs[1:]
		f.mu.Unlock()
		return m, nil
	}
	final := f.final
	f.mu.Unlock()
	if final != nil {
		return kafka.Message{}, final
	}
	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func (f *fakeReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range msgs {
		f.committed = append(f.committed, m.Offset)
	}
	return nil
}

func (f *fakeReader) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeReader) state() (committed []int64, closed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.committed...), f.closed
}

func TestConsumerRun_HandlesAndCommitsThenReturnsErrorOnEOF(t *testing.T) {
	r := &fakeReader{
		msgs:  []kafka.Message{{Offset: 1, Value: []byte("a")}, {Offset: 2, Value: []byte("b")}},
		final: io.EOF,
	}
	c := &Consumer{reader: r}
	var handled []int64
	err := c.Run(context.Background(), func(_ context.Context, rec Record) error {
		handled = append(handled, rec.Offset)
		return nil
	})
	if !errors.Is(err, ErrReaderClosed) || !errors.Is(err, io.EOF) {
		t.Fatalf("Run error = %v, want ErrReaderClosed wrapping io.EOF", err)
	}
	if fmt.Sprint(handled) != "[1 2]" {
		t.Errorf("handled offsets = %v, want [1 2]", handled)
	}
	committed, _ := r.state()
	if fmt.Sprint(committed) != "[1 2]" {
		t.Errorf("committed offsets = %v, want [1 2]", committed)
	}
}

func TestConsumerRun_WrappedEOFIsAnUnexpectedExit(t *testing.T) {
	c := &Consumer{reader: &fakeReader{final: fmt.Errorf("broker connection: %w", io.EOF)}}
	if err := c.Run(context.Background(), func(context.Context, Record) error { return nil }); !errors.Is(err, ErrReaderClosed) {
		t.Fatalf("Run error = %v, want ErrReaderClosed", err)
	}
}

func TestConsumerRun_ContextCancelReturnsNil(t *testing.T) {
	c := &Consumer{reader: &fakeReader{}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx, func(context.Context, Record) error { return nil }) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run error = %v, want nil on cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

func TestConsumerRun_HandlerErrorStillCommits(t *testing.T) {
	r := &fakeReader{msgs: []kafka.Message{{Offset: 7}}, final: io.EOF}
	c := &Consumer{reader: r}
	_ = c.Run(context.Background(), func(context.Context, Record) error { return errors.New("boom") })
	committed, _ := r.state()
	if fmt.Sprint(committed) != "[7]" {
		t.Errorf("committed = %v, want [7] even when the handler fails", committed)
	}
}

func TestSupervisor_RestartsConsumerAfterUnexpectedExit(t *testing.T) {
	var mu sync.Mutex
	var readers []*fakeReader
	newConsumer := func() *Consumer {
		mu.Lock()
		defer mu.Unlock()
		r := &fakeReader{}
		if len(readers) < 2 {
			r.final = io.EOF // the first two consumers die immediately
		}
		readers = append(readers, r)
		return &Consumer{reader: r}
	}
	s := NewSupervisor(newConsumer, func(context.Context, Record) error { return nil })
	s.backoffMin = time.Millisecond
	s.backoffMax = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	deadline := time.Now().Add(2 * time.Second)
	for !(s.Running() && s.Restarts() == 2) {
		if time.Now().After(deadline) {
			t.Fatalf("supervisor did not settle on a running third consumer (running=%v restarts=%d)", s.Running(), s.Restarts())
		}
		time.Sleep(time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Supervisor.Run did not return after cancellation")
	}
	if s.Running() {
		t.Error("Running() = true after Run returned")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(readers) != 3 {
		t.Fatalf("consumers created = %d, want 3", len(readers))
	}
	for i, r := range readers {
		if _, closed := r.state(); !closed {
			t.Errorf("consumer %d was not closed", i)
		}
	}
}

func TestSupervisor_NotRunningBeforeStart(t *testing.T) {
	s := NewSupervisor(func() *Consumer { return &Consumer{reader: &fakeReader{}} }, nil)
	if s.Running() {
		t.Error("Running() = true before Run")
	}
}

func TestNewConsumer_BatchesCommits(t *testing.T) {
	c := NewConsumer(Config{Broker: "localhost:9093", Topic: "t"}, "g", LatestOffset)
	defer c.Close()
	r, ok := c.reader.(*kafka.Reader)
	if !ok {
		t.Fatalf("reader is %T, want *kafka.Reader", c.reader)
	}
	if got := r.Config().CommitInterval; got != commitInterval {
		t.Errorf("CommitInterval = %v, want %v (batched, not per-record synchronous)", got, commitInterval)
	}
}

func TestConsumerRun_PassesRecordTime(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	c := &Consumer{reader: &fakeReader{msgs: []kafka.Message{{Offset: 1, Time: at}}, final: io.EOF}}
	var got time.Time
	_ = c.Run(context.Background(), func(_ context.Context, rec Record) error {
		got = rec.Time
		return nil
	})
	if !got.Equal(at) {
		t.Errorf("Record.Time = %v, want %v", got, at)
	}
}
