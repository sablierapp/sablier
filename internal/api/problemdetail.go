package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type ProblemDetail struct {
	// Type is a unique error code
	Type string `json:"type,omitempty"`
	// Title is a human-readable error message
	Title string `json:"title,omitempty"`
	// Status is the HTTP Status code
	Status int `json:"status,omitempty"`
	// Detail is a human-readable error description
	Detail string `json:"detail,omitempty"`
	error  error
}

func ValidationError(err error) ProblemDetail {
	return ProblemDetail{
		Type:   "validation-error",
		Title:  "Bad Request",
		Status: http.StatusBadRequest,
		Detail: err.Error(),
		error:  err,
	}
}

func AbortWithProblemDetail(c *gin.Context, p ProblemDetail) {
	_ = c.Error(p.error)
	c.IndentedJSON(p.Status, p)
}
