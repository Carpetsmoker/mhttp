package zhttp

import (
	"database/sql"
	"encoding/json"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"

	"zgo.at/zlog"
)

type (
	coder interface {
		Code() int
		Error() string
	}
	errorJSON interface {
		ErrorJSON() ([]byte, error)
	}
)

var ErrPage = DefaultErrPage

// TODO: make it easy to hide errors on production.
func DefaultErrPage(w http.ResponseWriter, r *http.Request, code int, reported error) {
	w.WriteHeader(code)

	if code >= 500 {
		zlog.Field("code", ErrorCode(reported)).FieldsRequest(r).Error(reported)
	}

	ct := strings.ToLower(r.Header.Get("Content-Type"))
	switch {
	case strings.HasPrefix(ct, "application/json"):
		var (
			j   []byte
			err error
		)
		if jErr, ok := reported.(json.Marshaler); ok {
			j, err = jErr.MarshalJSON()
		} else if jErr, ok := reported.(errorJSON); ok {
			j, err = jErr.ErrorJSON()
		} else {
			j, err = json.Marshal(map[string]string{"error": reported.Error()})
		}
		if err != nil {
			zlog.FieldsRequest(r).Error(err)
		}
		w.Write(j)

	case ct == "application/x-www-form-urlencoded" || strings.HasPrefix(ct, "multipart/"):
		fallthrough

	default:
		if tpl == nil {
			return
		}

		err := tpl.ExecuteTemplate(w, "error.gohtml", struct {
			Code  int
			Error error
		}{code, reported})
		if err != nil {
			zlog.FieldsRequest(r).Error(err)
		}
	}
}

// ErrorCode gets a hash based on the error value.
func ErrorCode(err error) string {
	if err == nil {
		return ""
	}

	h := fnv.New32()
	_, _ = h.Write([]byte(err.Error()))
	return strconv.FormatInt(int64(h.Sum32()), 36)
}

// HandlerFunc function.
type HandlerFunc func(http.ResponseWriter, *http.Request) error

// Wrap a http.HandlerFunc
func Wrap(handler HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := handler(w, r)
		if err == nil {
			return
		}

		// A github.com/teamwork/guru error with an embeded status code.
		if stErr, ok := err.(coder); ok {
			ErrPage(w, r, stErr.Code(), stErr)
			return
		}

		switch err {
		case sql.ErrNoRows:
			ErrPage(w, r, 404, err)
			return
		}

		// switch out := err.(type) {
		// }

		ErrPage(w, r, 500, err)
	}
}

func Bytes(w http.ResponseWriter, b []byte) error {
	w.WriteHeader(200)
	w.Write(b)
	return nil
}

func String(w http.ResponseWriter, s string) error {
	w.WriteHeader(200)
	w.Write([]byte(s))
	return nil
}

func JSON(w http.ResponseWriter, i interface{}) error {
	j, err := json.Marshal(i)
	if err != nil {
		return err
	}

	w.WriteHeader(200)
	w.Write(j)
	return nil
}

func Template(w http.ResponseWriter, name string, data interface{}) error {
	return tpl.ExecuteTemplate(w, name, data)
}

// MovedPermanently redirects to the given URL.
func MovedPermanently(w http.ResponseWriter, url string) error {
	w.Header().Set("Location", url)
	w.WriteHeader(301)
	return nil
}

// SeeOther redirects to the given URL.
//
// "A 303 response to a GET request indicates that the origin server does not
// have a representation of the target resource that can be transferred by the
// server over HTTP. However, the Location field value refers to a resource that
// is descriptive of the target resource, such that making a retrieval request
// on that other resource might result in a representation that is useful to
// recipients without implying that it represents the original target resource."
func SeeOther(w http.ResponseWriter, url string) error {
	w.Header().Set("Location", url)
	w.WriteHeader(303)
	return nil
}
