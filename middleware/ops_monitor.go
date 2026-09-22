package middleware

import (
	"bytes"

	"github.com/QuantumNous/new-api/pkg/opsmonitor"
	"github.com/gin-gonic/gin"
)

// Only error responses are buffered, up to the error-text limit. Successful
// bodies, streams, binary payloads, headers and credentials are never copied.
type opsErrorWriter struct {
	gin.ResponseWriter
	errorBody bytes.Buffer
}

func (w *opsErrorWriter) Write(data []byte) (int, error) {
	if w.Status() >= 400 && w.errorBody.Len() < opsmonitor.MaxErrorBytes+1 {
		_, _ = w.errorBody.Write(data[:min(len(data), opsmonitor.MaxErrorBytes+1-w.errorBody.Len())])
	}
	return w.ResponseWriter.Write(data)
}

func (w *opsErrorWriter) WriteString(data string) (int, error) { return w.Write([]byte(data)) }

// forced is used only by the dedicated Responses response.create engine and
// native plugin submission routes, whose route tag belongs to another engine.
func OpsMonitor(forced bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !opsmonitor.IsGeneration(c) {
			c.Next()
			return
		}
		opsmonitor.Begin(c)
		writer := &opsErrorWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		defer func() {
			recovered := recover()
			if forced || c.GetString(RouteTagKey) == "relay" {
				opsmonitor.Finish(c, writer.errorBody.String(), recovered != nil)
			} else {
				opsmonitor.Discard(c)
			}
			if recovered != nil {
				panic(recovered)
			}
		}()
		c.Next()
	}
}
