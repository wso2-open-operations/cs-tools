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

package middleware

import "net/http"

// securityHeaders are set on every response. This service only ever returns
// JSON or file downloads, never a page to render, so the content policy
// allows nothing to load and nothing to frame it; responses carry per-user
// data, so nothing may cache them or send the URL on as a referrer.
var securityHeaders = [][2]string{
	{"X-Content-Type-Options", "nosniff"},
	{"Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'"},
	{"Strict-Transport-Security", "max-age=31536000; includeSubDomains"},
	{"Cache-Control", "no-store"},
	{"Referrer-Policy", "no-referrer"},
	{"X-Frame-Options", "DENY"},
}

// setSecurityHeaders writes securityHeaders onto h. A handler that needs a
// different value for one of them can still Set it afterwards; download
// routes keep their own Content-Type, Content-Disposition and nosniff.
func setSecurityHeaders(h http.Header) {
	for _, kv := range securityHeaders {
		h.Set(kv[0], kv[1])
	}
}

// SecurityHeaders sets security-related response headers on every response.
// It is the outermost middleware, so they are present on every response,
// including CORS preflights and authentication failures.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w.Header())
		next.ServeHTTP(w, r)
	})
}
