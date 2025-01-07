package server

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"time"
)

// StructuredLogger logs a gin HTTP request in JSON format. Allows to set the
// logger for testing purposes.
func StructuredLogger(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		start := time.Now() // Start timer
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Fill the params
		param := gin.LogFormatterParams{}

		param.TimeStamp = time.Now() // Stop timer
		param.Latency = param.TimeStamp.Sub(start)
		if param.Latency > time.Minute {
			param.Latency = param.Latency.Truncate(time.Second)
		}

		param.ClientIP = c.ClientIP()
		param.Method = c.Request.Method
		param.StatusCode = c.Writer.Status()
		param.BodySize = c.Writer.Size()
		if raw != "" {
			path = path + "?" + raw
		}
		param.Path = path

		// Log using the params
		var logEvent *zerolog.Event
		if c.Writer.Status() >= 500 {
			logEvent = logger.Error()
		} else if c.Writer.Status() >= 400 {
			logEvent = logger.Warn()
		} else {
			logEvent = logger.Info()
		}

		logEvent.Str("client_id", param.ClientIP).
			Str("method", param.Method).
			Int("status_code", param.StatusCode).
			Int("body_size", param.BodySize).
			Str("path", param.Path).
			Str("latency", param.Latency.String()).
			Err(c.Errors.Last()).
			Msg("")
	}
}
