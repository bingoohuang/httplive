package httptee

import (
	"net/http"
)

// Tee duplicates the incoming req (req) and does the req to the
func (h *Handler) Tee(req *http.Request) {
	InsertForwardedHeaders(req)

	for _, alt := range h.Alternatives {
		alterReq := DuplicateRequest(req)
		SetRequestTarget(alterReq, alt)
		alterReq.Host = alt.Host

		h.workers.Run(req.Context(), AlternativeReq{Handler: h, req: alterReq})
	}
}
