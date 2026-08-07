package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/theme"
	"github.com/tniswong/go.rfcx/rfc7807"
)

func ProblemError(e error) rfc7807.Problem {
	return rfc7807.Problem{
		Type:   "https://sablierapp.dev/#/errors?id=internal-error",
		Title:  http.StatusText(http.StatusInternalServerError),
		Status: http.StatusInternalServerError,
		Detail: e.Error(),
	}
}

func ProblemValidation(e error) rfc7807.Problem {
	return rfc7807.Problem{
		Type:   "https://sablierapp.dev/#/errors?id=validation-error",
		Title:  "Validation Failed",
		Status: http.StatusBadRequest,
		Detail: e.Error(),
	}
}

func ProblemGroupNotFound(e sablier.ErrGroupNotFound) rfc7807.Problem {
	pb := rfc7807.Problem{
		Type:   "https://sablierapp.dev/#/errors?id=group-not-found",
		Title:  "Group not found",
		Status: http.StatusNotFound,
		Detail: "The group you requested does not exist. It is possible that the group has not been scanned yet.",
	}
	_ = pb.Extend("availableGroups", e.AvailableGroups)
	_ = pb.Extend("requestGroup", e.Group)
	_ = pb.Extend("error", e.Error())
	return pb
}

func ProblemThemeNotFound(e theme.ErrThemeNotFound) rfc7807.Problem {
	pb := rfc7807.Problem{
		Type:   "https://sablierapp.dev/#/errors?id=theme-not-found",
		Title:  "Theme not found",
		Status: http.StatusNotFound,
		Detail: "The theme you requested does not exist among the default themes and the custom themes (if any).",
	}
	_ = pb.Extend("availableTheme", e.AvailableThemes)
	_ = pb.Extend("requestTheme", e.Theme)
	_ = pb.Extend("error", e.Error())
	return pb
}

func ProblemTimeout(e sablier.ErrTimeout) rfc7807.Problem {
	detail := fmt.Sprintf("session was not ready after %s", e.Duration)
	if reasons := e.InstanceReasons(); len(reasons) > 0 {
		detail = fmt.Sprintf("%s: %s", detail, strings.Join(reasons, "; "))
	}

	pb := rfc7807.Problem{
		Type:   "https://sablierapp.dev/#/errors?id=timeout",
		Title:  http.StatusText(http.StatusGatewayTimeout),
		Status: http.StatusGatewayTimeout,
		Detail: detail,
	}

	if len(e.Instances) > 0 {
		type instanceDetail struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message,omitempty"`
			Error   string `json:"error,omitempty"`
		}
		details := make([]instanceDetail, 0, len(e.Instances))
		for _, i := range e.Instances {
			d := instanceDetail{
				Name:    i.Instance.Name,
				Status:  string(i.Instance.Status),
				Message: i.Instance.Message,
			}
			if i.Error != nil {
				d.Error = i.Error.Error()
			}
			details = append(details, d)
		}
		_ = pb.Extend("instances", details)
	}

	return pb
}

func ProblemInstanceNotManaged(e sablier.ErrInstanceNotManaged) rfc7807.Problem {
	pb := rfc7807.Problem{
		Type:   "https://sablierapp.dev/#/errors?id=instance-not-managed",
		Title:  "Instance not managed",
		Status: http.StatusNotFound,
		Detail: fmt.Sprintf("instance %q is not managed by Sablier; add the sablier.enable label to the instance", e.Name),
	}
	_ = pb.Extend("instance", e.Name)
	_ = pb.Extend("error", e.Error())
	return pb
}

func ProblemRequestCancelled() rfc7807.Problem {
	return rfc7807.Problem{
		Type:   "https://sablierapp.dev/#/errors?id=request-cancelled",
		Title:  "Request Cancelled",
		Status: http.StatusServiceUnavailable,
		Detail: "the request was cancelled before the session became ready",
	}
}
