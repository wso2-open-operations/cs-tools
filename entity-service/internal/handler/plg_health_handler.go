package handler

import "net/http"

// Health answers the liveness probe on GET /plg/health.
//
// Prefixed like everything else, so the slice can be probed independently while
// it runs as its own service. entity-service has its own GET /health, and after
// the merge this one is redundant — it is listed in the merge checklist for
// deletion rather than carried.
func Health(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]string{"status": "ok"})
}
