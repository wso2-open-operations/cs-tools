// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package notifications

import (
	"context"
	"errors"
	"testing"
)

type mockVoicer struct {
	err   error
	calls int
}

func (m *mockVoicer) Escalate(ctx context.Context, message string) error {
	m.calls++
	return m.err
}

type mockMessenger struct {
	err   error
	calls int
}

func (m *mockMessenger) SendMessage(ctx context.Context, text string) error {
	m.calls++
	return m.err
}

type mockMailer struct {
	err     error
	calls   int
	subject string
}

func (m *mockMailer) SendEscalation(ctx context.Context, subject, htmlBody string) error {
	m.calls++
	m.subject = subject
	return m.err
}

func TestMultiChannelEscalator_AllSucceed(t *testing.T) {
	voice := &mockVoicer{}
	chat := &mockMessenger{}
	mail := &mockMailer{}
	e := NewMultiChannelEscalator(voice, chat, mail, "subject")

	if err := e.Escalate(context.Background(), "message"); err != nil {
		t.Fatalf("Escalate() error = %v, want nil", err)
	}
	if voice.calls != 1 || chat.calls != 1 || mail.calls != 1 {
		t.Errorf("calls = voice:%d chat:%d mail:%d, want 1 each", voice.calls, chat.calls, mail.calls)
	}
	if mail.subject != "subject" {
		t.Errorf("mail subject = %q, want %q", mail.subject, "subject")
	}
}

func TestMultiChannelEscalator_PartialFailureStillSucceeds(t *testing.T) {
	voice := &mockVoicer{err: errors.New("twilio down")}
	chat := &mockMessenger{}
	mail := &mockMailer{err: errors.New("smtp down")}
	e := NewMultiChannelEscalator(voice, chat, mail, "subject")

	// Two of three channels fail; the third (chat) succeeds -> overall nil,
	// per the "at least one channel got through" contract.
	if err := e.Escalate(context.Background(), "message"); err != nil {
		t.Fatalf("Escalate() error = %v, want nil (chat succeeded)", err)
	}
	if voice.calls != 1 || chat.calls != 1 || mail.calls != 1 {
		t.Errorf("calls = voice:%d chat:%d mail:%d, want 1 each (every channel attempted regardless of others failing)", voice.calls, chat.calls, mail.calls)
	}
}

func TestMultiChannelEscalator_AllFail(t *testing.T) {
	voice := &mockVoicer{err: errors.New("twilio down")}
	chat := &mockMessenger{err: errors.New("chat down")}
	mail := &mockMailer{err: errors.New("smtp down")}
	e := NewMultiChannelEscalator(voice, chat, mail, "subject")

	err := e.Escalate(context.Background(), "message")
	if err == nil {
		t.Fatal("Escalate() error = nil, want an error when every channel fails")
	}
}

func TestMultiChannelEscalator_NilChannelsAreSkippedNotFailed(t *testing.T) {
	chat := &mockMessenger{}
	// Only chat is configured; twilio and email are nil.
	e := NewMultiChannelEscalator(nil, chat, nil, "subject")

	if err := e.Escalate(context.Background(), "message"); err != nil {
		t.Fatalf("Escalate() error = %v, want nil", err)
	}
	if chat.calls != 1 {
		t.Errorf("chat.calls = %d, want 1", chat.calls)
	}
}

func TestMultiChannelEscalator_NoChannelsConfiguredErrors(t *testing.T) {
	e := NewMultiChannelEscalator(nil, nil, nil, "subject")

	if err := e.Escalate(context.Background(), "message"); err == nil {
		t.Fatal("Escalate() error = nil, want an error when no channel is configured at all")
	}
}
