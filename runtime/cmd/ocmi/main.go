package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/edwardoffc/OCMI/runtime/adapters"
	"github.com/edwardoffc/OCMI/runtime/internal"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "help" || os.Args[1] == "call") {
		var response internal.Response
		if os.Args[1] == "call" {
			var asText bool
			var expected string
			response, asText, expected = callFormatted(os.Args[2:])
			if asText {
				if !writeContextText(response, expected) {
					writeContextJSON(response)
				}
			} else {
				write(response)
			}
		} else {
			response = help(os.Args[2:])
			write(response)
		}
		if !response.OK {
			os.Exit(2)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "update" {
		action, home, candidate, source, next, ok := updateFlags(os.Args[2:])
		if !ok {
			write(updateUsage())
			os.Exit(2)
		}
		response := internal.UpdateHome(action, home, candidate, source, next)
		write(response)
		if !response.OK {
			os.Exit(2)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "install" {
		home, inspect, ok := installFlags(os.Args[2:])
		if !ok {
			write(setupUsage())
			os.Exit(2)
		}
		response := internal.InstallHome(home, inspect)
		write(response)
		if !response.OK {
			os.Exit(2)
		}
		return
	}
	if len(os.Args) > 2 && os.Args[1] == "coordinator" {
		home, language, project, source, evidence, ok := coordinatorFlags(os.Args[2:])
		if !ok {
			write(setupUsage())
			os.Exit(2)
		}
		var response internal.Response
		switch os.Args[2] {
		case "prepare":
			response = internal.CoordinatorPrepare(home, language, project)
		case "inspect":
			response = internal.CoordinatorInspect(home)
		case "confirm":
			response = internal.CoordinatorConfirm(home, project, source, evidence)
		}
		write(response)
		if !response.OK {
			os.Exit(2)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		root, rootKind, actor, transferRoot, ok := adapterFlags(os.Args[2:])
		if !ok || adapters.ServeMCP(context.Background(), adapters.Config{ProjectRoot: root, RootKind: rootKind, Actor: actor, TransferRoot: transferRoot}) != nil {
			os.Exit(2)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "hook" {
		root, rootKind, actor, _, ok := adapterFlags(os.Args[2:])
		if !ok || adapters.RunHook(adapters.Config{ProjectRoot: root, RootKind: rootKind, Actor: actor}, os.Stdin, os.Stdout) != nil {
			os.Exit(2)
		}
		return
	}
	response := internal.Response{SchemaVersion: 1, OK: false, Code: "INVALID_REQUEST", Revision: 0, Data: map[string]any{}}
	asText := len(os.Args) == 4 && os.Args[1] == "request" && os.Args[2] == "--format" && os.Args[3] == "text"
	if !(len(os.Args) == 2 && os.Args[1] == "request") && !asText {
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
	if asText && !textRequestOK(request.Action, request.Payload) {
		response.Code = "USAGE"
		write(response)
		os.Exit(2)
	}
	response = internal.Handle(request)
	expected := ""
	if asText {
		var p struct {
			Expected string `json:"expectedBinding"`
		}
		_ = json.Unmarshal(request.Payload, &p)
		expected = p.Expected
	}
	if asText {
		if !writeContextText(response, expected) {
			writeContextJSON(response)
		}
	} else {
		write(response)
	}
	if !response.OK {
		os.Exit(2)
	}
}

func setupUsage() internal.Response {
	return internal.Response{SchemaVersion: 1, OK: false, Code: "USAGE", Data: map[string]any{"reason": "invalid_setup_options", "stage": "cli", "nextAction": "review command options"}}
}

func updateUsage() internal.Response {
	return internal.Response{SchemaVersion: 1, OK: false, Code: "USAGE", Data: map[string]any{"reason": "invalid_update_options", "stage": "cli", "nextAction": "review command options"}}
}

func updateFlags(args []string) (action, home, candidate, source, next string, ok bool) {
	if len(args) == 0 {
		return
	}
	action, args = args[0], args[1:]
	if action != "inspect" && action != "apply" && action != "recover" && action != "rollback" {
		return
	}
	for len(args) > 0 {
		if len(args) < 2 || args[1] == "" {
			return
		}
		key, value := args[0], args[1]
		args = args[2:]
		switch key {
		case "--home":
			if home != "" {
				return
			}
			home = value
		case "--candidate":
			if candidate != "" {
				return
			}
			candidate = value
		case "--expected-source":
			if source != "" {
				return
			}
			source = value
		case "--expected-candidate":
			if next != "" {
				return
			}
			next = value
		default:
			return
		}
	}
	if !filepath.IsAbs(home) {
		return
	}
	switch action {
	case "inspect":
		ok = source == "" && next == "" && (candidate == "" || filepath.IsAbs(candidate))
	case "apply":
		ok = filepath.IsAbs(candidate) && source != "" && next != ""
	case "recover", "rollback":
		ok = candidate == "" && source == "" && next == ""
	}
	return
}

func installFlags(args []string) (home string, inspect, ok bool) {
	for len(args) > 0 {
		if args[0] == "--inspect" && !inspect {
			inspect = true
			args = args[1:]
			continue
		}
		if len(args) < 2 || args[0] != "--home" || home != "" || args[1] == "" {
			return "", false, false
		}
		home = args[1]
		args = args[2:]
	}
	return home, inspect, filepath.IsAbs(home)
}

func coordinatorFlags(args []string) (home, language, project, source, evidence string, ok bool) {
	if len(args) == 0 {
		return
	}
	action := args[0]
	args = args[1:]
	for len(args) > 0 {
		if len(args) < 2 {
			return
		}
		key, value := args[0], args[1]
		args = args[2:]
		switch key {
		case "--home":
			if home != "" || value == "" {
				return
			}
			home = value
		case "--language":
			if language != "" || value == "" {
				return
			}
			language = value
		case "--client-project":
			if project != "" || value == "" {
				return
			}
			project = value
		case "--evidence-source":
			if source != "" || value == "" {
				return
			}
			source = value
		case "--evidence":
			if evidence != "" || value == "" {
				return
			}
			evidence = value
		default:
			return
		}
	}
	if !filepath.IsAbs(home) {
		return
	}
	switch action {
	case "inspect":
		ok = language == "" && project == "" && source == "" && evidence == ""
	case "prepare":
		ok = (language == "en" || language == "ru") && project != "" && source == "" && evidence == ""
	case "confirm":
		ok = project != "" && (source == "user-current" || source == "client-tool") && evidence != "" && language == ""
	}
	return
}

func adapterFlags(args []string) (root, rootKind, actor, transferRoot string, ok bool) {
	for len(args) > 0 {
		if len(args) < 2 {
			return "", "", "", "", false
		}
		switch args[0] {
		case "--project-root":
			root = args[1]
		case "--root-kind":
			rootKind = args[1]
		case "--actor":
			actor = args[1]
		case "--transfer-root":
			transferRoot = args[1]
		default:
			return "", "", "", "", false
		}
		args = args[2:]
	}
	return root, rootKind, actor, transferRoot, root != "" && actor != "" && (rootKind == "" || rootKind == "project" || rootKind == "installation")
}

func write(response internal.Response) {
	encoded, err := json.Marshal(response)
	if err != nil {
		fmt.Fprint(os.Stdout, `{"schemaVersion":1,"ok":false,"code":"ENCODE_ERROR","revision":0,"data":{}}`)
		return
	}
	fmt.Fprint(os.Stdout, string(encoded))
}
