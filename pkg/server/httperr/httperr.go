package httperr

import (
	"fmt"
	"log/slog"
	"net/http"
)

func Detail(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%+v", err)
}

func Error(w http.ResponseWriter, err error, status int) {
	http.Error(w, Detail(err), status)
}

func Log(log *slog.Logger, msg string, err error) {
	log.Error(msg, "detail", Detail(err), "error", err.Error())
}
