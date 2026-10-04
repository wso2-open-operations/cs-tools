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

package eventbus

import (
	"testing"
	"time"
)

// TestNewProducer_FlushesEachPublishImmediately pins the writer settings
// that keep a lone synchronous Publish from waiting out kafka-go's default
// one-second batch timer, and that it stays synchronous.
func TestNewProducer_FlushesEachPublishImmediately(t *testing.T) {
	p := NewProducer(Config{Broker: "127.0.0.1:1", ConnectionString: "x", Topic: "t"})
	defer p.Close()
	if p.writer.BatchSize != 1 {
		t.Errorf("BatchSize = %d, want 1", p.writer.BatchSize)
	}
	if p.writer.BatchTimeout <= 0 || p.writer.BatchTimeout > 50*time.Millisecond {
		t.Errorf("BatchTimeout = %v, want a short explicit timeout", p.writer.BatchTimeout)
	}
	if p.writer.Async {
		t.Error("Async = true, want synchronous publishes: every caller acts on the result")
	}
}
