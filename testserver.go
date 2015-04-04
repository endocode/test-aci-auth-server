package main

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type httpError struct {
	code    int
	message string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("%d: %s", e.code, e.message)
}

type serverHandler struct {
	auth string
	stop chan<- struct{}
	msg  chan<- string
}

func (h *serverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		w.WriteHeader(http.StatusOK)
		h.stop <- struct{}{}
		return
	case "GET":
		// handled later
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch h.auth {
	case "none":
		// no auth to do.
	case "basic":
		payload, httpErr := getAuthPayload(r, "Basic")
		if httpErr != nil {
			w.WriteHeader(httpErr.code)
			h.msg <- fmt.Sprintf(`No "Authorization" header: %v`, httpErr.message)
			return
		}
		creds, err := base64.StdEncoding.DecodeString(string(payload))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			h.msg <- fmt.Sprintf(`Badly formed "Authorization" header`)
			return
		}
		parts := strings.Split(string(creds), ":")
		if len(parts) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			h.msg <- fmt.Sprintf(`Badly formed "Authorization" header (2)`)
			return
		}
		user := parts[0]
		password := parts[1]
		if user != "bar" || password != "baz" {
			w.WriteHeader(http.StatusUnauthorized)
			h.msg <- fmt.Sprintf("Bad credentials: %q", string(creds))
			return
		}
	case "oauth":
		payload, httpErr := getAuthPayload(r, "Bearer")
		if httpErr != nil {
			w.WriteHeader(httpErr.code)
			h.msg <- fmt.Sprintf(`No "Authorization" header: %v`, httpErr.message)
			return
		}
		if payload != "sometoken" {
			w.WriteHeader(http.StatusUnauthorized)
			h.msg <- fmt.Sprintf(`Bad token: %q`, payload)
			return
		}
	default:
		panic("Woe is me!")
	}
	h.msg <- fmt.Sprintf("Trying to serve %q", r.URL.String())
	switch filepath.Base(r.URL.Path) {
	case "prog.aci":
		h.msg <- fmt.Sprintf("  serving")
		if data, err := prepareACI(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			h.msg <- fmt.Sprintf("    failed (%v)", err)
		} else {
			w.Write(data)
			h.msg <- fmt.Sprintf("    done.")
		}
	default:
		h.msg <- fmt.Sprintf("  not found.")
		w.WriteHeader(http.StatusNotFound)
	}
}

func prepareACI() ([]byte, error) {
	if err := buildProg(); err != nil {
		return nil, err
	}
	fn, err := buildACI();
	if err != nil {
		return nil, err
	}
	defer os.Remove(fn)
	return ioutil.ReadFile(fn)
}

func buildProg() error {
	compiler, err := exec.LookPath("go")
	if err != nil {
		return err
	}
	cmd := exec.Cmd{
		Path: compiler,
		Args: []string{
			"go",
			"build",
			"-o",
			"prog",
			"./prog.go",
		},
		Dir: "ACI/rootfs",
	}
	return cmd.Run()
}

func buildACI() (string, error) {
	actool, err := exec.LookPath("actool")
	if err != nil {
		return "", err
	}
	timedata, err := time.Now().MarshalBinary()
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile("ACI/rootfs/stamp", timedata, 0644); err != nil {
		return "", err
	}
	fn := "prog-build.aci"
	cmd := exec.Cmd{
		Path: actool,
		Args: []string{
			"actool",
			"build",
			"ACI",
			fn,
		},
	}
	return fn, cmd.Run()
}

func getAuthPayload(r *http.Request, authType string) (string, *httpError) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		err := &httpError{
			code:    http.StatusUnauthorized,
			message: "No auth",
		}
		return "", err
	}
	parts := strings.Split(auth, " ")
	if len(parts) != 2 {
		err := &httpError{
			code:    http.StatusBadRequest,
			message: "Malformed auth",
		}
		return "", err
	}
	if parts[0] != authType {
		err := &httpError{
			code:    http.StatusUnauthorized,
			message: "Wrong auth",
		}
		return "", err
	}
	return parts[1], nil
}

func main() {
	typesStr := "none, basic, oauth"
	types := strings.Split(typesStr, ", ")
	if len(os.Args) < 2 {
		fmt.Printf("Expected a type - %s\n", typesStr)
		os.Exit(1)
	}
	auth := ""
	for _, v := range types {
		if v == os.Args[1] {
			auth = v
			break
		}
	}
	if auth == "" {
		fmt.Printf("Wrong type %q, should, be %s\n", os.Args[1], typesStr)
		os.Exit(1)
	}
	stop := make(chan struct{})
	msg := make(chan string)
	server := &serverHandler{
		auth: auth,
		stop: stop,
		msg:  msg,
	}
	ts := httptest.NewUnstartedServer(server)
	ts.TLS = &tls.Config{InsecureSkipVerify: true}
	ts.StartTLS()
	defer ts.Close()
	parsed, _ := url.Parse(ts.URL)
	switch auth {
	case "none":
		// nothing to do
	case "basic":
		creds := `"user": "bar",
		"password": "baz"`
		printCreds(parsed.Host, auth, creds)
	case "oauth":
		creds := `"token": "sometoken"`
		printCreds(parsed.Host, auth, creds)
	default:
		panic("Woe is me!")
	}
	fmt.Printf("Ready, waiting for connections at %s\n", ts.URL)
	loop(stop, msg)
	fmt.Println("Byebye")
}

func loop(stopChan <-chan struct{}, msgChan <-chan string) {
	for {
		var msg string
		select {
		case <-stopChan:
			return
		case msg = <-msgChan:
			fmt.Println(msg)
		}
	}
}

func printCreds(host, auth, creds string) {
	fmt.Printf(`
{
	"rktKind": "auth",
	"rktVersion": "v1",
	"domains": ["%s"],
	"type": "%s",
	"credentials":
	{
		%s
	}
}

`, host, auth, creds)
}
