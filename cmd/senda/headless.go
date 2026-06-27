// runHeadless runs a collection (or one folder of it) headlessly — same
// pipeline as the desktop app: scripts, variables, secrets, asserts, cookie jar.
// Exit code 0 = every request passed; 1 = at least one failure.
//
//	senda run -collection ./my-api [-folder auth] [-env dev] [-q]
//	senda run -collection ./my-api --report junit [-o report.xml]
//	senda run -collection ./my-api --docs [-o docs/api.md] [--docs-format html|md]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"senda/internal/docgen"
	"senda/internal/flow"
	"senda/internal/model"
	"senda/internal/pipeline"
	"senda/internal/runner"
	"senda/internal/store"
)

func runHeadless(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	collPath := fs.String("collection", ".", "collection root directory")
	folder := fs.String("folder", "", "subfolder to run (default: whole collection)")
	env := fs.String("env", "", "environment name")
	quiet := fs.Bool("q", false, "only print the summary line")
	docs := fs.Bool("docs", false, "generate API documentation instead of running requests")
	docsOutput := fs.String("o", "", "output file for docs (default: stdout)")
	docsFormat := fs.String("docs-format", "md", "docs output format: md or html")
	dataFile := fs.String("data", "", "CSV or JSON data file for data-driven runs")
	report := fs.String("report", "", "machine-readable run report instead of text: json or junit (to -o file or stdout)")
	flowName := fs.String("flow", "", "run a flow (name under .senda/flows or a path) instead of a folder")
	_ = fs.Parse(args)

	root, err := filepath.Abs(*collPath)
	if err != nil {
		fatal(err)
	}

	if *docs {
		runDocs(root, *folder, *docsOutput, *docsFormat)
		return
	}

	if *flowName != "" {
		runFlow(root, *flowName, *env, *quiet, *report, *docsOutput)
		return
	}

	target := root
	if *folder != "" {
		target = filepath.Join(root, *folder)
	}
	paths, err := store.ListRequests(target)
	if err != nil {
		fatal(err)
	}
	if len(paths) == 0 {
		fatal(fmt.Errorf("no requests under %s", target))
	}

	// Data-driven: load rows if --data is set.
	var dataRows []map[string]string
	if *dataFile != "" {
		var err error
		dataRows, err = runner.LoadDataFile(*dataFile)
		if err != nil {
			fatal(err)
		}
	}

	session := pipeline.NewSession()
	makeSend := func(extra map[string]string) runner.Send {
		return func(ctx context.Context, path string) (model.Request, model.Response, error) {
			req, err := store.ReadRequest(path)
			if err != nil {
				return req, model.Response{}, err
			}
			resp, appliedURL := session.SendWithExtra(ctx, req, root, path, *env, extra)
			req.URL = appliedURL // report the interpolated URL, not the raw template
			return req, resp, nil
		}
	}
	send := makeSend(nil)

	// Report mode streams nothing — only the final report goes to stdout.
	silent := *quiet || *report != ""
	onResult := func(r model.RunResult) {
		if silent {
			return
		}
		fmt.Println(formatResult(r))
	}

	var results []model.RunResult
	if len(dataRows) > 0 {
		ri := 0
		results = runner.RunFolderWithData(context.Background(), paths, dataRows,
			makeSend,
			func(rowIdx int, r model.RunResult) {
				ri++
				if !silent {
					fmt.Printf("[row %d] ", rowIdx+1)
				}
				onResult(r)
			})
		_ = ri
	} else {
		results = runner.RunFolder(context.Background(), paths, send, onResult)
	}

	passed := 0
	for _, r := range results {
		if r.OK {
			passed++
		}
	}

	if *report != "" {
		out, err := renderReport(*report, results)
		if err != nil {
			fatal(err)
		}
		if *docsOutput == "" {
			fmt.Println(string(out))
		} else if err := os.WriteFile(*docsOutput, out, 0o644); err != nil {
			fatal(err)
		} else {
			fmt.Fprintf(os.Stderr, "report written to %s\n", *docsOutput)
		}
	} else {
		fmt.Printf("\n%d/%d passed\n", passed, len(results))
	}
	if passed != len(results) {
		os.Exit(1)
	}
}

// formatResult renders one run result as the single status line `senda run`
// prints per request: a pass/fail mark, the name, method, status, duration, and
// (when present) the assertion tally and any error.
func formatResult(r model.RunResult) string {
	mark := "✓"
	if !r.OK {
		mark = "✗"
	}
	line := fmt.Sprintf("%s %-40s %s %d (%dms)", mark, r.Name, r.Method, r.Status, r.DurationMs)
	if r.AssertPass+r.AssertFail > 0 {
		line += fmt.Sprintf("  asserts %d/%d", r.AssertPass, r.AssertPass+r.AssertFail)
	}
	if r.Error != "" {
		line += "  " + r.Error
	}
	return line
}

// runFlow executes a *.flow.yaml graph headlessly, reusing the same session and
// send path as a folder run. Request steps feed the existing report renderers;
// non-request steps print in text mode only.
func runFlow(root, nameOrPath, env string, quiet bool, report, outFile string) {
	fpath, err := store.ResolveFlow(root, nameOrPath)
	if err != nil {
		fatal(fmt.Errorf("flow %q: %w", nameOrPath, err))
	}
	fl, err := store.ReadFlow(fpath)
	if err != nil {
		fatal(err)
	}

	session := pipeline.NewSession()
	makeSend := func(extra map[string]string) runner.Send {
		return func(ctx context.Context, path string) (model.Request, model.Response, error) {
			abs := path
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(root, path)
			}
			req, err := store.ReadRequest(abs)
			if err != nil {
				return req, model.Response{}, err
			}
			resp, appliedURL := session.SendWithExtra(ctx, req, root, abs, env, extra)
			req.URL = appliedURL // report the interpolated URL, not the raw template
			return req, resp, nil
		}
	}

	silent := quiet || report != ""
	steps, runErr := flow.Run(context.Background(), fl, flow.Runner{
		MakeSend: makeSend,
		Resolve:  func(s string) string { return session.Resolve(root, "", env, s) },
		Data:     func(p string) ([]map[string]string, error) { return runner.LoadDataFile(resolveUnder(root, p)) },
		SetVar:   session.SetVar,
		OnStep: func(sr flow.StepResult) {
			if !silent {
				fmt.Println(formatStep(sr))
			}
		},
	})

	var results []model.RunResult
	for _, s := range steps {
		if s.Result != nil {
			results = append(results, *s.Result)
		}
	}
	passed := 0
	for _, r := range results {
		if r.OK {
			passed++
		}
	}

	if report != "" {
		out, err := renderReport(report, results)
		if err != nil {
			fatal(err)
		}
		if outFile == "" {
			fmt.Println(string(out))
		} else if err := os.WriteFile(outFile, out, 0o644); err != nil {
			fatal(err)
		} else {
			fmt.Fprintf(os.Stderr, "report written to %s\n", outFile)
		}
	} else {
		fmt.Printf("\n%d/%d passed\n", passed, len(results))
	}
	if runErr != nil {
		fatal(runErr)
	}
	if passed != len(results) {
		os.Exit(1)
	}
}

// resolveUnder joins a collection-relative path against root, leaving absolute
// paths untouched — used so a flow's request/data paths resolve against the
// collection, not the process working directory.
func resolveUnder(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

// formatStep renders one flow step: request steps reuse the run-result line;
// branch/setvar/delay/loop/parallel steps print a node marker.
func formatStep(s flow.StepResult) string {
	if s.Result != nil {
		return formatResult(*s.Result)
	}
	line := fmt.Sprintf("• %-20s %s", s.NodeID, s.Type)
	if s.Branch != "" {
		line += " → " + s.Branch
	}
	if s.Err != "" {
		line += "  " + s.Err
	}
	return line
}

func runDocs(collPath, subFolder, outFile, format string) {
	subPath := ""
	if subFolder != "" {
		subPath = filepath.Join(collPath, subFolder)
	}

	var (
		content string
		err     error
	)
	switch docgen.Format(format) {
	case docgen.FormatHTML:
		content, err = docgen.GenerateHTML(collPath, subPath)
	default:
		content, err = docgen.GenerateMarkdown(collPath, subPath)
	}
	if err != nil {
		fatal(err)
	}

	if outFile == "" {
		fmt.Print(content)
		return
	}
	if err := os.WriteFile(outFile, []byte(content), 0o644); err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "docs written to %s\n", outFile)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "senda:", err)
	os.Exit(1)
}
