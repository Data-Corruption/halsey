package disc

import (
	"errors"
	"net/http"

	"github.com/disgoorg/disgo/rest"
)

func IsUnknownMessage(err error) bool {
	var re rest.Error
	if errors.As(err, &re) {
		if re.Code == rest.JSONErrorCode(10008) { // "Unknown Message"
			return true
		}
		if re.Response != nil && re.Response.StatusCode == http.StatusNotFound {
			return true
		}
	}
	return false
}
