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

// Package servicebus connects to Azure Service Bus over AMQP on WebSockets, signing in with an Entra ID application.
package servicebus

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/coder/websocket"
)

// Config holds what is needed to reach one Service Bus namespace; shared keys are off, so it signs in with an Entra ID application.
type Config struct {
	Namespace    string
	TenantID     string
	ClientID     string
	ClientSecret string
}

// amqpSubprotocol is the WebSocket subprotocol Service Bus expects for AMQP.
const amqpSubprotocol = "amqp"

// namespaceSuffix completes a bare namespace name into its fully qualified host.
const namespaceSuffix = ".servicebus.windows.net"

// NewClient returns a Service Bus client whose AMQP connection runs over WebSockets on port 443.
func NewClient(cfg Config) (*azservicebus.Client, error) {
	if cfg.Namespace == "" || cfg.TenantID == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("service bus namespace, tenant id, client id and client secret are all required")
	}
	credential, err := azidentity.NewClientSecretCredential(cfg.TenantID, cfg.ClientID, cfg.ClientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("service bus credential: %w", err)
	}
	client, err := azservicebus.NewClient(FullyQualifiedNamespace(cfg.Namespace), credential, &azservicebus.ClientOptions{
		NewWebSocketConn: dialWebSocket,
		RetryOptions: azservicebus.RetryOptions{
			MaxRetries:    5,
			RetryDelay:    time.Second,
			MaxRetryDelay: 30 * time.Second,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("service bus client: %w", err)
	}
	return client, nil
}

// FullyQualifiedNamespace accepts a bare namespace name or its host and returns the host.
func FullyQualifiedNamespace(namespace string) string {
	namespace = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(namespace), "sb://"), "/")
	if strings.Contains(namespace, ".") {
		return namespace
	}
	return namespace + namespaceSuffix
}

// dialWebSocket opens the WebSocket the SDK runs AMQP over, so only outbound HTTPS is needed.
func dialWebSocket(ctx context.Context, args azservicebus.NewWebSocketConnArgs) (net.Conn, error) {
	conn, _, err := websocket.Dial(ctx, args.Host, &websocket.DialOptions{Subprotocols: []string{amqpSubprotocol}})
	if err != nil {
		return nil, fmt.Errorf("service bus websocket: %w", err)
	}
	// The connection outlives the dial, so it is bound to a context that is never cancelled.
	return websocket.NetConn(context.Background(), conn, websocket.MessageBinary), nil
}
