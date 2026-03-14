package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type cliSource struct {
	ID                 string `json:"id"`
	Kind               string `json:"kind"`
	Title              string `json:"title"`
	URL                string `json:"url"`
	InputURL           string `json:"inputUrl"`
	Locator            string `json:"locator"`
	PathLabel          string `json:"pathLabel"`
	IntakeStage        string `json:"intakeStage"`
	ValidationSummary  string `json:"validationSummary"`
	NextAction         string `json:"nextAction"`
	PrototypeReadiness string `json:"prototypeReadiness"`
	Status             string `json:"status"`
}

type cliTask struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	EntityID     string `json:"entityId"`
	Status       string `json:"status"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	base := getenv("YTVAULT_API", "http://127.0.0.1:8080")
	switch os.Args[1] {
	case "health":
		get(base + "/healthz")
	case "source":
		handleSource(base, os.Args[2:])
	case "task":
		handleTask(base, os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func handleSource(base string, args []string) {
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		get(base + "/api/v1/sources")
	case "add":
		if len(args) < 3 {
			fmt.Println("usage: yd source add <video|playlist|channel> <url>")
			os.Exit(1)
		}
		post(base+"/api/v1/sources", map[string]string{"kind": args[1], "url": args[2]})
	default:
		usage()
		os.Exit(1)
	}
}

func handleTask(base string, args []string) {
	if len(args) == 0 {
		fmt.Println("usage: yd task <list|run>")
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		get(base + "/api/v1/tasks")
	case "run":
		if len(args) < 2 {
			fmt.Println("usage: yd task run <task_id>")
			os.Exit(1)
		}
		post(base+"/api/v1/tasks/"+args[1]+"/run", map[string]string{})
	default:
		fmt.Println("usage: yd task <list|run>")
		os.Exit(1)
	}
}

func get(url string) {
	resp, err := http.Get(url)
	must(err)
	defer resp.Body.Close()
	copyBody(resp.Body)
}

func post(url string, body any) {
	buf, err := json.Marshal(body)
	must(err)
	resp, err := http.Post(url, "application/json", bytes.NewReader(buf))
	must(err)
	defer resp.Body.Close()
	copyBody(resp.Body)
}

func copyBody(r io.Reader) {
	data, err := io.ReadAll(r)
	must(err)
	if formatted, ok := formatKnownResponse(data); ok {
		_, err = os.Stdout.WriteString(formatted)
		must(err)
		return
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err == nil {
		pretty, err := json.MarshalIndent(parsed, "", "  ")
		must(err)
		_, err = os.Stdout.Write(pretty)
		must(err)
		_, err = os.Stdout.Write([]byte("\n"))
		must(err)
		return
	}
	_, err = os.Stdout.Write(data)
	must(err)
}

func formatKnownResponse(data []byte) (string, bool) {
	var create struct {
		Source cliSource `json:"source"`
		Task   cliTask   `json:"task"`
	}
	if err := json.Unmarshal(data, &create); err == nil && create.Source.ID != "" {
		return renderSource(create.Source) + renderTask(create.Task), true
	}
	var list struct {
		Items []cliSource `json:"items"`
	}
	if err := json.Unmarshal(data, &list); err == nil && list.Items != nil {
		if len(list.Items) == 0 {
			return "No sources yet\n", true
		}
		first := list.Items[0]
		if first.Kind != "" || first.URL != "" || first.Title != "" || first.InputURL != "" || first.PathLabel != "" || first.Locator != "" {
			var b strings.Builder
			for i, item := range list.Items {
				if i > 0 {
					b.WriteString("\n")
				}
				b.WriteString(renderSource(item))
			}
			return b.String(), true
		}
	}
	var runTask struct {
		Task cliTask `json:"task"`
	}
	if err := json.Unmarshal(data, &runTask); err == nil && runTask.Task.ID != "" {
		return renderTask(runTask.Task), true
	}
	var taskList struct {
		Items []cliTask `json:"items"`
	}
	if err := json.Unmarshal(data, &taskList); err == nil && taskList.Items != nil {
		if len(taskList.Items) == 0 {
			return "No tasks yet\n", true
		}
		var b strings.Builder
		for _, item := range taskList.Items {
			b.WriteString(renderTask(item))
		}
		return b.String(), true
	}
	return "", false
}

func renderSource(src cliSource) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Source %s\n", src.ID)
	fmt.Fprintf(&b, "  Title: %s\n", src.Title)
	fmt.Fprintf(&b, "  Kind: %s\n", src.Kind)
	if src.PathLabel != "" {
		fmt.Fprintf(&b, "  Path: %s\n", src.PathLabel)
	}
	if src.Locator != "" {
		fmt.Fprintf(&b, "  Locator: %s\n", src.Locator)
	}
	fmt.Fprintf(&b, "  Normalized: %s\n", src.URL)
	if src.InputURL != "" {
		fmt.Fprintf(&b, "  Original: %s\n", src.InputURL)
	}
	if src.IntakeStage != "" {
		fmt.Fprintf(&b, "  Intake: %s\n", src.IntakeStage)
	}
	if src.ValidationSummary != "" {
		fmt.Fprintf(&b, "  Validation: %s\n", src.ValidationSummary)
	}
	if src.NextAction != "" {
		fmt.Fprintf(&b, "  Next: %s\n", src.NextAction)
	}
	if src.PrototypeReadiness != "" {
		fmt.Fprintf(&b, "  Prototype: %s\n", src.PrototypeReadiness)
	}
	fmt.Fprintf(&b, "  Status: %s\n", src.Status)
	return b.String()
}

func renderTask(task cliTask) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Task %s\n", task.ID)
	fmt.Fprintf(&b, "  Type: %s\n", task.Type)
	fmt.Fprintf(&b, "  Entity: %s\n", task.EntityID)
	fmt.Fprintf(&b, "  Status: %s\n", task.Status)
	if task.ErrorCode != "" {
		fmt.Fprintf(&b, "  ErrorCode: %s\n", task.ErrorCode)
	}
	if task.ErrorMessage != "" {
		fmt.Fprintf(&b, "  ErrorMessage: %s\n", task.ErrorMessage)
	}
	return b.String()
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func usage() {
	fmt.Print(`YtVault CLI

Usage:
  yd health
  yd source list
  yd source add <video|playlist|channel> <url>
  yd task list
  yd task run <task_id>
`)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
