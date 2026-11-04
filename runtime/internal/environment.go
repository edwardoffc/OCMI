package internal

import (
	"path/filepath"
	"sort"
	"strings"
)

type environmentSource struct{ Kind, Path string }
type environmentTool struct{ Kind, Executable string }
type environmentReference struct {
	Kind, Source string
	Line         int
}
type environmentInput struct {
	Sources      []environmentSource
	Tools        []environmentTool
	ScratchRoot  string
	Provisions   []environmentReference
	Continuation *struct {
		Source string
		Line   int
	}
}

func environmentInspect(r Request) Response {
	if r.Actor != "main" || r.ExpectedRevision != 0 {
		return bad("ENVIRONMENT_REFUSED", 0)
	}
	var in environmentInput
	if !checkpointStrictJSON(r.Payload, &in) || len(in.Sources) == 0 || !filepath.IsAbs(in.ScratchRoot) || environmentScratchAllowed(r.ProjectRoot, in.ScratchRoot) != nil {
		return bad("ENVIRONMENT_INVALID", 0)
	}
	set, e := environmentReadSources(r.ProjectRoot, in.Sources)
	if e != nil {
		return bad("ENVIRONMENT_INVALID", 0)
	}
	defer set.Close()
	guides := map[string][]string{}
	for _, f := range set.Files {
		if f.Kind == "guide" {
			guides[f.Path] = strings.Split(string(f.Raw), "\n")
		}
	}
	for _, x := range in.Provisions {
		if (x.Kind != "data" && x.Kind != "secret" && x.Kind != "service" && x.Kind != "access") || x.Line < 1 || x.Line > len(guides[x.Source]) {
			return bad("ENVIRONMENT_INVALID", 0)
		}
	}
	if in.Continuation != nil && (in.Continuation.Line < 1 || in.Continuation.Line > len(guides[in.Continuation.Source])) {
		return bad("ENVIRONMENT_INVALID", 0)
	}
	type bound struct{ p, id, sha string }
	tools := map[string]bound{}
	for _, t := range in.Tools {
		if t.Kind != "go" && t.Kind != "node" && t.Kind != "python" {
			return bad("ENVIRONMENT_INVALID", 0)
		}
		p, id, sha, x := environmentExecutable(r.ProjectRoot, t.Executable)
		if x != nil {
			return bad("ENVIRONMENT_INVALID", 0)
		}
		if _, ok := tools[t.Kind]; ok {
			return bad("ENVIRONMENT_INVALID", 0)
		}
		tools[t.Kind] = bound{p, id, sha}
	}
	// All manifests and conflicts are known before operation effects or probes.
	preDeclared := map[string][]string{}
	for _, f := range set.Files {
		ds, err := environmentParse(f.Kind, f.Raw)
		if err != nil {
			return bad("ENVIRONMENT_INVALID", 0)
		}
		for _, d := range ds {
			if d.Kind != "package-manager" {
				preDeclared[d.Kind] = append(preDeclared[d.Kind], d.Declared)
			}
		}
	}
	preConflict := map[string]bool{}
	for k, ds := range preDeclared {
		preConflict[k] = environmentConflict(ds)
	}
	op, e := newEnvironmentOperation(in.ScratchRoot, r.ProjectRoot)
	if e != nil {
		if op != nil {
			out := bad("ENVIRONMENT_UNRESOLVED", 0)
			out.Data = map[string]any{"ownedStage": op.root, "nextAction": "inspect retained environment operation evidence"}
			return out
		}
		return bad("ENVIRONMENT_INVALID", 0)
	}
	defer op.cancel()
	if set.Recheck() != nil {
		out := bad("ENVIRONMENT_UNRESOLVED", 0)
		out.Data = map[string]any{"ownedStage": op.root, "nextAction": "inspect retained environment operation evidence"}
		return out
	}
	reqs := []map[string]any{}
	declared := map[string][]string{}
	probed := map[string]struct {
		v   string
		err error
	}{}
	for _, f := range set.Files {
		ds, x := environmentParse(f.Kind, f.Raw)
		if x != nil {
			return bad("ENVIRONMENT_INVALID", 0)
		}
		for _, d := range ds {
			q := map[string]any{"kind": d.Kind, "source": f.Path, "declared": d.Declared, "status": "unknown", "observed": ""}
			if d.Kind == "package-manager" {
				reqs = append(reqs, q)
				continue
			}
			declared[d.Kind] = append(declared[d.Kind], d.Declared)
			if preConflict[d.Kind] {
				q["status"] = "conflict"
				reqs = append(reqs, q)
				continue
			}
			if d.Declared != "" && environmentConstraint(d.Declared) {
				if t, ok := tools[d.Kind]; !ok {
					q["status"] = "missing"
				} else if !environmentExecutableUnchanged(r.ProjectRoot, t.p, t.id, t.sha) || set.Recheck() != nil {
					out := bad("ENVIRONMENT_UNRESOLVED", 0)
					out.Data = map[string]any{"ownedStage": op.root, "nextAction": "inspect retained environment operation evidence"}
					return out
				} else if cached, ok := probed[d.Kind]; ok && cached.err == nil {
					q["observed"] = cached.v
					if environmentMatches(cached.v, d.Declared) {
						q["status"] = "match"
					} else {
						q["status"] = "mismatch"
					}
				} else if v, x := op.run(t.p, d.Kind); x == nil {
					probed[d.Kind] = struct {
						v   string
						err error
					}{v: v}
					q["observed"] = v
					if environmentMatches(v, d.Declared) {
						q["status"] = "match"
					} else {
						q["status"] = "mismatch"
					}
				} else {
					out := bad("ENVIRONMENT_UNRESOLVED", 0)
					out.Data = map[string]any{"ownedStage": op.root, "nextAction": "inspect retained environment operation evidence"}
					return out
				}
			}
			reqs = append(reqs, q)
		}
	}
	for k, ds := range declared {
		if environmentConflict(ds) {
			for _, q := range reqs {
				if q["kind"] == k {
					q["status"] = "conflict"
				}
			}
		}
	}
	for _, t := range tools {
		if !environmentExecutableUnchanged(r.ProjectRoot, t.p, t.id, t.sha) {
			out := bad("ENVIRONMENT_UNRESOLVED", 0)
			out.Data = map[string]any{"ownedStage": op.root, "nextAction": "inspect retained environment operation evidence"}
			return out
		}
	}
	if set.Recheck() != nil {
		out := bad("ENVIRONMENT_UNRESOLVED", 0)
		out.Data = map[string]any{"ownedStage": op.root, "nextAction": "inspect retained environment operation evidence"}
		return out
	}
	sources := []map[string]any{}
	for _, f := range set.Files {
		sources = append(sources, map[string]any{"kind": f.Kind, "path": f.Path, "sha256": f.SHA})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i]["path"].(string) < sources[j]["path"].(string) })
	prov := []map[string]any{}
	for _, x := range in.Provisions {
		prov = append(prov, map[string]any{"kind": x.Kind, "source": x.Source, "line": x.Line, "status": "unverified"})
	}
	steps := []map[string]any{}
	for _, q := range reqs {
		if q["status"] == "conflict" || q["status"] == "unknown" {
			steps = append(steps, map[string]any{"kind": "resolve", "source": q["source"], "action": "resolve declared requirement before tool selection"})
		}
	}
	for _, q := range reqs {
		if q["status"] != "match" && q["status"] != "conflict" && q["status"] != "unknown" {
			steps = append(steps, map[string]any{"kind": "tool", "source": q["source"], "action": "select or install declared tool"})
		}
	}
	steps = append(steps, map[string]any{"kind": "dependencies", "source": set.Files[0].Path, "action": "review declared dependencies and setup guidance"})
	for _, p := range prov {
		steps = append(steps, map[string]any{"kind": "provision", "source": p["source"], "line": p["line"], "action": "verify declared provision"})
	}
	if in.Continuation != nil {
		steps = append(steps, map[string]any{"kind": "continuation", "source": in.Continuation.Source, "line": in.Continuation.Line, "action": "confirm selected continuation"})
	}
	if op.finish("complete") != nil {
		out := bad("ENVIRONMENT_CLEANUP_REQUIRED", 0)
		out.Data = map[string]any{"ownedStage": op.root, "nextAction": "inspect retained environment operation evidence"}
		return out
	}
	return good("ENVIRONMENT_INSPECTED", 0, map[string]any{"sources": sources, "requirements": reqs, "provisions": prov, "recoverySteps": steps, "nextAction": steps[0]["action"], "readyToResume": false, "status": "inspection_complete", "ownedStage": op.root, "operationState": "complete"})
}
