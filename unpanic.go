package zhttp

import (
	"fmt"
	"net/http"

	"zgo.at/zlog"
)

// TODO: https://github.com/Teamwork/middleware/blob/master/rescue/rescue.go
func Unpanic(prod bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}

				err, ok := rec.(error)
				if !ok {
					err = fmt.Errorf("Panic: %+v", err)
				}

				zlog.Error(err)

				// TODO: filter stack
				// if prod {
				// 	msg = "Oops :-("
				// }
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}()

			next.ServeHTTP(w, r)
		})
	}
}
