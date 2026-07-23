package main

const (
	errBadRequest        = "Bad Request: invalid request"
	errTooManyRequests   = "Too Many Requests: rate limit exceeded, please retry later"
	errServiceUnavailable = "Service Unavailable"
	errNotFound          = "Not Found"
	errMethodNotAllowed  = "Method Not Allowed"
	errURLTooLong        = "Bad Request: URL too long"
	errBodyNotAllowed    = "Bad Request: request body not allowed"
	errForbidden         = "Forbidden"
)

func statusText(code int) string {
	switch code {
	case 400:
		return errBadRequest
	case 403:
		return errForbidden
	case 404:
		return errNotFound
	case 405:
		return errMethodNotAllowed
	case 414:
		return errURLTooLong
	case 429:
		return errTooManyRequests
	case 503:
		return errServiceUnavailable
	default:
		return ""
	}
}
