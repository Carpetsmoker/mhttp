package zhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"zgo.at/zlog"
)

// HandlerRobots writes a simple robots.txt.
func HandlerRobots(rules [][]string) func(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	for _, r := range rules {
		buf.WriteString(fmt.Sprintf("%s\n%s\n\n", r[0], r[1]))
	}
	text := buf.Bytes()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Cache-Control", "public,max-age=31536000")
		w.WriteHeader(200)
		w.Write(text)
	}
}

// CSP errors.
type (
	CSPError struct {
		Report Report `json:"csp-report"`
	}

	Report struct {
		BlockedURI   string `json:"blocked-uri"`
		ColumnNumber int    `json:"column-number"`
		DocumentURI  string `json:"document-uri"`
		LineNumber   int    `json:"line-number"`
		Referrer     string `json:"referrer"`
		SourceFile   string `json:"source-file"`
		Violated     string `json:"violated-directive"`
	}
)

// HandlerCSP handles CSP errors.
func HandlerCSP() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		d, _ := ioutil.ReadAll(r.Body)
		var csp CSPError
		err := json.Unmarshal(d, &csp)

		// Probably an extension or something.
		if err == nil && !noise(csp.Report) {
			zlog.Fields(zlog.F{
				"BlockedURI":   csp.Report.BlockedURI,
				"ColumnNumber": csp.Report.ColumnNumber,
				"DocumentURI":  csp.Report.DocumentURI,
				"LineNumber":   csp.Report.LineNumber,
				"Referrer":     csp.Report.Referrer,
				"SourceFile":   csp.Report.SourceFile,
				"Violated":     csp.Report.Violated,
			}).Errorf("CSP error")
		}

		w.WriteHeader(202)
	}
}

func noise(r Report) bool {
	// Probably some extension or whatnot that injected a script.
	if r.ColumnNumber == 1 && r.LineNumber == 1 &&
		r.Violated == "script-src" &&
		(r.BlockedURI == "inline" || r.BlockedURI == "eval") {
		return true
	}

	if strings.HasPrefix(r.SourceFile, "safari-extension://") || strings.HasPrefix(r.SourceFile, "moz-extension://") {
		return true
	}

	return false
}
