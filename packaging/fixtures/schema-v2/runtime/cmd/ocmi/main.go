package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/edwardoffc/OCMI/runtime/adapters"
	"github.com/edwardoffc/OCMI/runtime/internal"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		root, actor, transferRoot, ok := adapterFlags(os.Args[2:])
		if !ok || adapters.ServeMCP(context.Background(), adapters.Config{ProjectRoot: root, Actor: actor, TransferRoot: transferRoot}) != nil {
			os.Exit(2)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "hook" {
		root, actor, _, ok := adapterFlags(os.Args[2:])
		if !ok || adapters.RunHook(adapters.Config{ProjectRoot: root, Actor: actor}, os.Stdin, os.Stdout) != nil {
			os.Exit(2)
		}
		return
	}
	response := internal.Response{SchemaVersion: 1, OK: false, Code: "INVALID_REQUEST", Revision: 0, Data: map[string]any{}}
	if len(os.Args) != 2 || os.Args[1] != "request" {
		response.Code = "USAGE"
		write(response)
		os.Exit(2)
	}
	const maxRequest = 8 << 20
	input, err := io.ReadAll(io.LimitReader(bufio.NewReader(os.Stdin), maxRequest+1))
	if err != nil || len(input) > maxRequest {
		response.Code = "READ_ERROR"
		write(response)
		os.Exit(2)
	}
	var request internal.Request
	if err := json.Unmarshal(input, &request); err != nil {
		response.Code = "MALFORMED_JSON"
		write(response)
		os.Exit(2)
	}
	response = internal.Handle(request)
	write(response)
	if !response.OK {
		os.Exit(2)
	}
}

func adapterFlags(args []string) (root, actor, transferRoot string, ok bool) {
	for len(args) > 0 {
		if len(args) < 2 {
			return "", "", "", false
		}
		switch args[0] {
		case "--project-root":
			root = args[1]
		case "--actor":
			actor = args[1]
		case "--transfer-root":
			transferRoot = args[1]
		default:
			return "", "", "", false
		}
		args = args[2:]
	}
	return root, actor, transferRoot, root != "" && actor != ""
}

func write(response internal.Response) {
	encoded, err := json.Marshal(response)
	if err != nil {
		fmt.Fprint(os.Stdout, `{"schemaVersion":1,"ok":false,"code":"ENCODE_ERROR","revision":0,"data":{}}`)
		return
	}
	fmt.Fprint(os.Stdout, string(encoded))
}
