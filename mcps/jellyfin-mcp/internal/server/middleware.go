package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// timingAndLoggingMiddleware logs the duration of every tools/call to stderr
// and, when a session is available, sends structured MCP log notifications.
func timingAndLoggingMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			start := time.Now()
			result, err := next(ctx, method, req)
			duration := time.Since(start)

			// Only log tool calls (not list operations, pings, etc.)
			if method != "tools/call" {
				return result, err
			}

			// Extract tool name from request params
			toolName := "unknown"
			if p, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok {
				toolName = p.Name
			}

			// Always log to stderr for local debugging
			log.Printf("%s(%s): %s", method, toolName, duration)

			// Send structured MCP log to connected client if session available
			if session, ok := req.GetSession().(*mcp.ServerSession); ok {
				level := mcp.LoggingLevel("info")

				// Log errors at warning level
				if tr, ok := result.(*mcp.CallToolResult); ok && tr != nil && tr.IsError {
					level = "warning"
				}

				if err := session.Log(ctx, &mcp.LoggingMessageParams{
					Level:  level,
					Logger: "jellyfin",
					Data:   fmt.Sprintf("%s(%s) completed in %s", method, toolName, duration),
				}); err != nil {
					log.Printf("session.Log: %v", err)
				}
			}

			return result, err
		}
	}
}
