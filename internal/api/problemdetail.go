package api

import (
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/theme"
	"github.com/tniswong/go.rfcx/rfc7807"
	"net/http"
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
