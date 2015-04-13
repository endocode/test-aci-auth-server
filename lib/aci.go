package lib

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

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
	manifestStr    = `{"acKind":"ImageManifest","acVersion":"0.5.1+git","name":"testprog","app":{"exec":["/prog"],"user":"0","group":"0"}}`
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
		Dir:    filepath.Join(aciDir, "rootfs"),
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
