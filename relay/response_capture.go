package relay

import (
	"bytes"
	"io"
	"net/http"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

const maxResponseBodyCaptureSize = 1 << 20 // 1MB

// captureWriter wraps gin.ResponseWriter and captures written bytes into a buffer.
type captureWriter struct {
	gin.ResponseWriter
	buf  bytes.Buffer
	mu   sync.Mutex
	done bool
}

func (w *captureWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.capture(data)
	return n, err
}

func (w *captureWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.capture([]byte(s))
	return n, err
}

func (w *captureWriter) capture(data []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.done && w.buf.Len()+len(data) <= maxResponseBodyCaptureSize {
		w.buf.Write(data)
	}
}

func (w *captureWriter) getCaptured() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.done = true
	return w.buf.Bytes()
}

// PrepareResponseCapture sets up response capturing before DoResponse is called.
// - Captures upstream response headers into gin context.
// - For non-streaming: pre-reads resp.Body and replaces with a reader.
// - For streaming: wraps c.Writer with a captureWriter.
func PrepareResponseCapture(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) {
	if !common.LogRecordResponseEnabled && !common.LogRecordOnErrorEnabled {
		return
	}
	if resp == nil {
		return
	}

	// Capture response headers
	if len(resp.Header) > 0 {
		headers := make(map[string]string, len(resp.Header))
		for k := range resp.Header {
			headers[k] = resp.Header.Get(k)
		}
		common.SetContextKey(c, constant.ContextKeyResponseHeaders, headers)
	}

	if info.IsStream {
		// Streaming: wrap the ResponseWriter to capture SSE data
		cw := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = cw
	} else {
		// Non-streaming: pre-read response body and replace
		if resp.Body != nil {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err == nil {
				common.SetContextKey(c, constant.ContextKeyResponseBody, string(bodyBytes))
				resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}
	}
}

// FinalizeResponseCapture reads captured streaming data from the writer wrapper
// and stores it in the gin context.
func FinalizeResponseCapture(c *gin.Context, info *relaycommon.RelayInfo) {
	if !common.LogRecordResponseEnabled && !common.LogRecordOnErrorEnabled {
		return
	}
	if !info.IsStream {
		return
	}
	cw, ok := c.Writer.(*captureWriter)
	if !ok {
		return
	}
	captured := cw.getCaptured()
	if len(captured) > 0 {
		common.SetContextKey(c, constant.ContextKeyResponseBody, string(captured))
	}
	// Restore original writer
	c.Writer = cw.ResponseWriter
}
