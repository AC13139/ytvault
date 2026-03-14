package api

import (
	"fmt"
	"html"
	"net/http"
	"strings"
)

func sourceInfoHTML(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return fmt.Sprintf("<div><strong>%s:</strong> %s</div>", html.EscapeString(label), html.EscapeString(value))
}

func sourceBadgeHTML(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return fmt.Sprintf("<span class=\"source-badge\">%s</span>", html.EscapeString(text))
}

func (a *App) handleIndex(w http.ResponseWriter, _ *http.Request) {
	sources := a.store.ListSources()
	tasks := a.store.ListTasks()
	var sourceRows []string
	for _, s := range sources {
		badges := strings.Join([]string{
			sourceBadgeHTML(string(s.Kind)),
			sourceBadgeHTML(s.PathLabel),
			sourceBadgeHTML(s.IntakeStage),
		}, " ")
		details := strings.Join([]string{
			fmt.Sprintf("<div class=\"source-title\">%s</div>", html.EscapeString(s.Title)),
			fmt.Sprintf("<div class=\"source-badges\">%s</div>", badges),
			sourceInfoHTML("Locator", s.Locator),
			sourceInfoHTML("Normalized", s.URL),
			sourceInfoHTML("Original", s.InputURL),
			sourceInfoHTML("Validation", s.ValidationSummary),
			sourceInfoHTML("Next", s.NextAction),
			sourceInfoHTML("Prototype", s.PrototypeReadiness),
		}, "")
		sourceRows = append(sourceRows, fmt.Sprintf("<tr><td>%s</td><td>%s</td><td class=\"details\">%s</td><td>%s</td></tr>", html.EscapeString(s.ID), html.EscapeString(string(s.Kind)), details, html.EscapeString(string(s.Status))))
	}
	if len(sourceRows) == 0 {
		sourceRows = []string{"<tr><td colspan='4'>No sources yet</td></tr>"}
	}
	var taskRows []string
	for _, t := range tasks {
		taskRows = append(taskRows, fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", html.EscapeString(t.ID), html.EscapeString(t.Type), html.EscapeString(t.EntityID), html.EscapeString(string(t.Status))))
	}
	if len(taskRows) == 0 {
		taskRows = []string{"<tr><td colspan='4'>No tasks yet</td></tr>"}
	}
	page := `<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>YtVault</title>
<style>
body{font-family:Inter,system-ui,sans-serif;background:#0b1220;color:#e5e7eb;margin:0;padding:24px}
.container{max-width:1100px;margin:0 auto}.grid{display:grid;grid-template-columns:1.1fr .9fr;gap:20px}
.card{background:#121a2b;border:1px solid #24304a;border-radius:14px;padding:18px}
h1,h2{margin:0 0 12px}p{color:#9ca3af}label{display:block;font-size:14px;margin:10px 0 6px}input,select,button{width:100%%;border-radius:10px;border:1px solid #334155;padding:10px;background:#0f172a;color:#e5e7eb}button{background:#2563eb;border:none;font-weight:600;cursor:pointer;margin-top:12px}table{width:100%%;border-collapse:collapse;font-size:14px}th,td{padding:10px;border-top:1px solid #24304a;text-align:left;vertical-align:top}a{color:#93c5fd}.muted{color:#94a3b8;font-size:13px}.stat{display:flex;gap:16px;margin-bottom:14px}.pill{background:#0f172a;border:1px solid #24304a;padding:10px 12px;border-radius:999px}.details div{margin-bottom:6px;word-break:break-word}.source-title{font-size:16px;font-weight:700;margin-bottom:8px}.source-badges{display:flex;gap:8px;flex-wrap:wrap;margin-bottom:10px}.source-badge{display:inline-flex;align-items:center;background:#172033;border:1px solid #31415f;border-radius:999px;padding:4px 10px;font-size:12px;color:#bfdbfe}
@media (max-width: 900px){.grid{grid-template-columns:1fr}}
</style></head>
<body><div class="container">
<h1>YtVault</h1><p>Phase 1 control surface for YouTube channel / playlist / single video sources.</p>
<div class="stat"><div class="pill">Sources: %d</div><div class="pill">Tasks: %d</div><div class="pill">Mode: validated intake prototype</div></div>
<div class="grid">
<div class="card"><h2>Add source</h2><p class="muted">This prototype validates intake, normalizes the source URL, records the source profile, and queues an index task stub. Live indexing/download is not wired yet.</p>
<form method="post" action="/api/v1/sources" onsubmit="return submitForm(event)">
<label>Kind</label><select name="kind" id="kind"><option value="video">video</option><option value="playlist">playlist</option><option value="channel">channel</option></select>
<label>YouTube URL</label><input id="url" name="url" placeholder="https://www.youtube.com/watch?v=...">
<button type="submit">Create source</button></form><pre id="result" class="muted"></pre></div>
<div class="card"><h2>Recent tasks</h2><table><thead><tr><th>ID</th><th>Type</th><th>Entity</th><th>Status</th></tr></thead><tbody>%s</tbody></table></div>
</div>
<div class="card" style="margin-top:20px"><h2>Sources</h2><table><thead><tr><th>ID</th><th>Kind</th><th>Source profile</th><th>Status</th></tr></thead><tbody>%s</tbody></table></div>
</div>
<script>
async function submitForm(event){event.preventDefault();const body={kind:document.getElementById('kind').value,url:document.getElementById('url').value};const res=await fetch('/api/v1/sources',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});const text=await res.text();document.getElementById('result').textContent=text;if(res.ok){window.location.reload()}return false}
</script></body></html>`
	_, _ = w.Write([]byte(fmt.Sprintf(page, len(sources), len(tasks), strings.Join(taskRows, ""), strings.Join(sourceRows, ""))))
}
