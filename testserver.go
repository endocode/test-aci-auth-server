package main

import (
	"bytes"
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
	dir, err := createTree()
	if dir != "" {
		defer os.RemoveAll(dir)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to build ACI tree: %v", err)
	}
	if err := buildProg(dir); err != nil {
		return nil, fmt.Errorf("failed to build test program: %v", err)
	}
	fn, err := buildACI(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to build ACI: %v", err)
	}
	defer os.Remove(fn)
	contents, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, fmt.Errorf("failed to read ACI to memory: %v", err)
	}
	return contents, nil
}

const (
	manifestStr = `{"acKind":"ImageManifest","acVersion":"0.5.1+git","name":"testprog","app":{"exec":["/prog"],"user":"0","group":"0"}}`
	testProgSrcStr = `
package main

import (
	"fmt"
	"time"
)

func main() {
	for i := 3; i > 0; i -= 1 {
		fmt.Println(i)
		time.Sleep(time.Second)
	}
	fmt.Println("BANG!")
}
`
)

func createTree() (string, error) {
	aciDir := "ACI"
	rootDir := filepath.Join(aciDir, "rootfs")
	manifestFile := filepath.Join(aciDir, "manifest")
	srcFile := filepath.Join(rootDir, "prog.go")
	if err := os.Mkdir(aciDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create ACI directory: %v", err)
	}
	if err := os.Mkdir(rootDir, 0755); err != nil {
		return aciDir, fmt.Errorf("failed to create rootfs directory: %v", err)
	}
	if err := ioutil.WriteFile(manifestFile, []byte(manifestStr), 0644); err != nil {
		return "", fmt.Errorf("failed to write manifest: %v", err)
	}
	if err := ioutil.WriteFile(srcFile, []byte(testProgSrcStr), 0644); err != nil {
		return "", fmt.Errorf("failed to write go source: %v", err)
	}
	return aciDir, nil
}

func buildProg(aciDir string) error {
	compiler, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("failed to find `go`: %v", err)
	}
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd := exec.Cmd{
		Path: compiler,
		Args: []string{
			"go",
			"build",
			"-o",
			"prog",
			"./prog.go",
		},
		Dir: filepath.Join(aciDir, "rootfs"),
		Stdout: outBuf,
		Stderr: errBuf,
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute `go build`: %v\nstdout:\n%v\n\nstderr:\n%v)", err, outBuf.String(), errBuf.String())
	}
	return nil
}

func buildACI(aciDir string) (string, error) {
	actool, err := exec.LookPath("actool")
	if err != nil {
		return "", fmt.Errorf("failed to find `actool`: %v", err)
	}
	timedata, err := time.Now().MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to serialize current date to bytes: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(aciDir, "rootfs", "stamp"), timedata, 0644); err != nil {
		return "", fmt.Errorf("failed to write a stamp: %v", err)
	}
	fn := "prog-build.aci"
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd := exec.Cmd{
		Path: actool,
		Args: []string{
			"actool",
			"build",
			aciDir,
			fn,
		},
		Stdout: outBuf,
		Stderr: errBuf,
	}
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute `actool build`: %v\nstdout:\n%v\n\nstderr:\n%v)", err, outBuf.String(), errBuf.String())
	}
	return fn, nil
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
	cmdsStr := "start, stop"
	if len(os.Args) < 2 {
		fmt.Printf("Error: expected a command - %s\n", cmdsStr)
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "start":
		err = startServer(os.Args[2:])
	case "stop":
		err = stopServer(os.Args[2:])
	default:
		err = fmt.Errorf("wrong command %q, should be %s", os.Args[1], cmdsStr)
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func startServer(args []string) error {
	typesStr := "none, basic, oauth"
	if len(args) < 1 {
		return fmt.Errorf("expected a type - %s", typesStr)
	}
	types := strings.Split(typesStr, ", ")
	auth := ""
	for _, v := range types {
		if v == args[0] {
			auth = v
			break
		}
	}
	if auth == "" {
		return fmt.Errorf("wrong type %q, should, be %s", args[0], typesStr)
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
	parsed, err := url.Parse(ts.URL)
	if err != nil {
		return err
	}
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
	return nil
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

func stopServer(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected a host")
	}
	host := args[0]
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: transport}
	res, err := client.Post(host, "whatever", nil)
	if err != nil {
		return fmt.Errorf("failed to send post to %q: %v", host, err)
	}
	defer res.Body.Close()
	fmt.Printf("Response status: %s\n", res.Status)
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("got a nonsuccess status")
	}
	return nil
}
