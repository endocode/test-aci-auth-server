package main

import (
	"fmt"
	"os"

	"github.com/endocode/test-aci-auth-server/lib"
)

func main() {
	cmdsStr := "start, stop"
	if len(os.Args) < 2 {
		fmt.Printf("Error: expected a command - %s\n", cmdsStr)
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "start":
		err = start(os.Args[2:])
	case "stop":
		err = stop(os.Args[2:])
	default:
		err = fmt.Errorf("wrong command %q, should be %s", os.Args[1], cmdsStr)
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func start(args []string) error {
	typesStr := "none, basic, oauth"
	if len(args) < 1 {
		return fmt.Errorf("expected a type - %s", typesStr)
	}
	types := map[string]lib.Type{
		"none":  lib.None,
		"basic": lib.Basic,
		"oauth": lib.Oauth,
	}
	auth, ok := types[args[0]]
	if !ok {
		return fmt.Errorf("wrong type %q, should, be %s", args[0], typesStr)
	}
	server := lib.StartServer(auth)
	if server.Conf != "" {
		fmt.Printf(server.Conf)
	}
	fmt.Printf("Ready, waiting for connections at %s\n", server.URL)
	loop(server)
	fmt.Println("Byebye")
	return nil
}

func loop(server *lib.Server) {
	for {
		select {
		case <-server.Stop:
			server.Close()
			return
		case msg, ok := <-server.Msg:
			if ok {
				fmt.Println(msg)
			}
		}
	}
}

func stop(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected a host")
	}
	host := args[0]
	res, err := lib.StopServer(host)
	if err != nil {
		return fmt.Errorf("failed to stop server: %v", err)
	}
	defer res.Body.Close()
	fmt.Printf("Response status: %s\n", res.Status)
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("got a nonsuccess status")
	}
	return nil
}
