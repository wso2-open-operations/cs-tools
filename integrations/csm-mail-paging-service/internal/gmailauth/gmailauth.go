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

// Package gmailauth runs the one-time OAuth consent for the mailbox user and
// prints a refresh token. It is run by hand through the server's gmail-auth
// subcommand; the service itself never logs in.
package gmailauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-mail-paging-service/internal/gmail"
)

// Run performs the consent flow and prints the refresh token to stdout.
func Run() error {
	id, secret := os.Getenv("GMAIL_OAUTH_CLIENT_ID"), os.Getenv("GMAIL_OAUTH_CLIENT_SECRET")
	if id == "" || secret == "" {
		return errors.New("GMAIL_OAUTH_CLIENT_ID and GMAIL_OAUTH_CLIENT_SECRET are required")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	oc := &oauth2.Config{
		ClientID:     id,
		ClientSecret: secret,
		Scopes:       []string{gmail.ScopeModify},
		Endpoint:     oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: gmail.OAuthEndpoint},
		RedirectURL:  fmt.Sprintf("http://%s/callback", ln.Addr()),
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	state := hex.EncodeToString(b)

	codes := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state || r.URL.Query().Get("code") == "" {
			http.Error(w, "invalid callback", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "Done. You can close this tab.")
		select {
		case codes <- r.URL.Query().Get("code"):
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	// access_type=offline + prompt=consent make Google issue a refresh token.
	fmt.Println("Open this URL and log in as the mailbox user:")
	fmt.Println()
	// PKCE binds the returned code to this process.
	verifier := oauth2.GenerateVerifier()
	fmt.Println(oc.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"), oauth2.S256ChallengeOption(verifier)))

	select {
	case code := <-codes:
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		tok, err := oc.Exchange(ctx, code, oauth2.VerifierOption(verifier))
		if err != nil {
			return err
		}
		if tok.RefreshToken == "" {
			return errors.New("no refresh token returned; revoke the app's access in the account and retry")
		}
		fmt.Println("\nGMAIL_OAUTH_REFRESH_TOKEN=" + tok.RefreshToken)
		return nil
	case <-time.After(5 * time.Minute):
		return errors.New("timed out waiting for consent")
	}
}
