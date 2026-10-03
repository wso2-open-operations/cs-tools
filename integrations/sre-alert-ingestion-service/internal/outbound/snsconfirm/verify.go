// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
package snsconfirm

import (
	"crypto"
	"crypto/rsa"
	_ "crypto/sha1" // SignatureVersion 1
	_ "crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// message holds the SNS fields a subscription confirmation is signed over.
type message struct {
	Type             string `json:"Type"`
	MessageID        string `json:"MessageId"`
	Token            string `json:"Token"`
	TopicArn         string `json:"TopicArn"`
	Message          string `json:"Message"`
	SubscribeURL     string `json:"SubscribeURL"`
	Timestamp        string `json:"Timestamp"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertURL"`
}

// stringToSign is AWS's canonical form for SubscriptionConfirmation and UnsubscribeConfirmation.
func (m message) stringToSign() string {
	var b strings.Builder
	for _, kv := range [][2]string{
		{"Message", m.Message}, {"MessageId", m.MessageID}, {"SubscribeURL", m.SubscribeURL},
		{"Timestamp", m.Timestamp}, {"Token", m.Token}, {"TopicArn", m.TopicArn}, {"Type", m.Type},
	} {
		b.WriteString(kv[0] + "\n" + kv[1] + "\n")
	}
	return b.String()
}

// verifier checks SNS signatures against AWS's signing certificate.
type verifier struct {
	http     *http.Client
	roots    *x509.CertPool // nil means the system roots
	allowURL func(*url.URL) bool

	mu    sync.Mutex
	certs map[string]*x509.Certificate
}

func (v *verifier) verify(m message) error {
	var hash crypto.Hash
	switch m.SignatureVersion {
	case "1":
		hash = crypto.SHA1
	case "2":
		hash = crypto.SHA256
	default:
		return fmt.Errorf("unsupported SignatureVersion %q", m.SignatureVersion)
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || len(sig) == 0 {
		return errors.New("missing or invalid Signature")
	}
	cert, err := v.cert(m.SigningCertURL)
	if err != nil {
		return err
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("signing certificate is not RSA")
	}
	h := hash.New()
	h.Write([]byte(m.stringToSign()))
	if err := rsa.VerifyPKCS1v15(pub, hash, h.Sum(nil), sig); err != nil {
		return errors.New("signature does not match")
	}
	return nil
}

// cert fetches (once) the signing certificate, which must be an SNS-hosted .pem that chains to a trusted root.
func (v *verifier) cert(raw string) (*x509.Certificate, error) {
	u, err := url.Parse(raw)
	if err != nil || !v.allowURL(u) || !strings.HasSuffix(u.Path, ".pem") {
		return nil, fmt.Errorf("SigningCertURL %q is not an AWS SNS certificate URL", raw)
	}
	v.mu.Lock()
	cached := v.certs[raw]
	v.mu.Unlock()
	if cached != nil {
		return cached, nil
	}

	resp, err := v.http.Get(raw)
	if err != nil {
		return nil, fmt.Errorf("fetch signing certificate: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("fetch signing certificate: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	var chain []*x509.Certificate
	for block, rest := pem.Decode(body); block != nil; block, rest = pem.Decode(rest) {
		if c, err := x509.ParseCertificate(block.Bytes); err == nil {
			chain = append(chain, c)
		}
	}
	if len(chain) == 0 {
		return nil, errors.New("no certificate in SigningCertURL")
	}
	intermediates := x509.NewCertPool()
	for _, c := range chain[1:] {
		intermediates.AddCert(c)
	}
	if _, err := chain[0].Verify(x509.VerifyOptions{Roots: v.roots, Intermediates: intermediates}); err != nil {
		return nil, fmt.Errorf("signing certificate not trusted: %w", err)
	}
	v.mu.Lock()
	if v.certs == nil {
		v.certs = map[string]*x509.Certificate{}
	}
	v.certs[raw] = chain[0]
	v.mu.Unlock()
	return chain[0], nil
}
