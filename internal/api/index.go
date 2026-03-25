package api

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"ytvault/internal/models"
)

// ── Badge / tag helpers ───────────────────────────────────────────────────────

func statusBadge(status string) string {
	return fmt.Sprintf(`<span class="badge badge-%s">%s</span>`,
		html.EscapeString(status), html.EscapeString(status))
}

func tag(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return fmt.Sprintf(`<span class="tag">%s</span>`, html.EscapeString(text))
}

// ── Date-filter helpers ───────────────────────────────────────────────────────

// dateAfterPresets lists the yt-dlp --dateafter presets shown in the UI.
var dateAfterPresets = []struct{ v, l string }{
	{"", "Lifetime"},
	{"now-1days", "Last day"},
	{"now-3days", "Last 3 days"},
	{"now-7days", "Last week"},
	{"now-14days", "Last 2 weeks"},
	{"now-30days", "Last 30 days"},
	{"now-90days", "Last 3 months"},
	{"now-365days", "Last year"},
	{"now-730days", "Last 2 years"},
}

func buildDateAfterOptions(selected string) string {
	var b strings.Builder
	for _, p := range dateAfterPresets {
		sel := ""
		if p.v == selected {
			sel = ` selected`
		}
		fmt.Fprintf(&b, `<option value="%s"%s>%s</option>`,
			html.EscapeString(p.v), sel, html.EscapeString(p.l))
	}
	return b.String()
}

// ── Schedule cell (channel/playlist only) ────────────────────────────────────

func buildIntervalOptions(selected int) string {
	opts := []struct {
		v int
		l string
	}{
		{1, "1h"}, {6, "6h"}, {12, "12h"}, {24, "24h"}, {168, "1 week"},
	}
	var b strings.Builder
	for _, o := range opts {
		sel := ""
		if o.v == selected {
			sel = ` selected`
		}
		fmt.Fprintf(&b, `<option value="%d"%s>Every %s</option>`, o.v, sel, o.l)
	}
	return b.String()
}

func buildScheduleCell(sourceID string, kind models.SourceKind, schedBySource map[string]models.Schedule) string {
	// Schedules only make sense for channel/playlist (not a single video).
	if kind == models.SourceKindVideo {
		return `<span class="src-meta">N/A for videos</span>`
	}

	defaultInterval := 24
	autoDLChecked := ""
	defaultDateAfter := ""
	schedStatus := `<span class="src-meta">No schedule</span>`
	removeBtn := ""

	if sched, ok := schedBySource[sourceID]; ok {
		defaultInterval = sched.IntervalHours
		defaultDateAfter = sched.DateAfter
		if sched.AutoDownload {
			autoDLChecked = " checked"
		}
		state := "disabled"
		if sched.Enabled {
			state = "enabled"
		}
		autoLabel := ""
		if sched.AutoDownload {
			autoLabel = " + auto-dl"
		}
		nextRun := sched.NextRunAt.UTC().Format("2006-01-02 15:04 UTC")
		schedStatus = fmt.Sprintf(
			`<span class="src-meta">Every %dh%s · %s · next %s</span>`,
			sched.IntervalHours, autoLabel, statusBadge(state), nextRun,
		)
		removeBtn = fmt.Sprintf(
			` <button class="btn-sm btn-orange" onclick="removeSchedule(this,'%s')">Remove</button>`,
			html.EscapeString(sourceID),
		)
	}

	sid := html.EscapeString(sourceID)
	return fmt.Sprintf(
		`<div>%s</div>`+
			`<div style="display:flex;gap:6px;align-items:center;flex-wrap:wrap;margin-top:6px">`+
			`<select id="sched-interval-%s" style="width:auto;padding:4px 8px;font-size:12px">%s</select>`+
			`<select id="sched-dateafter-%s" style="width:auto;padding:4px 8px;font-size:12px">%s</select>`+
			`<label style="display:flex;align-items:center;gap:4px;font-size:12px;color:#94a3b8;cursor:pointer">`+
			`<input type="checkbox" id="sched-autodl-%s"%s style="width:auto;margin:0"> Auto-dl</label>`+
			`<button class="btn-sm btn-blue" onclick="setSchedule('%s')">Set</button>%s`+
			`</div>`,
		schedStatus,
		sid, buildIntervalOptions(defaultInterval),
		sid, buildDateAfterOptions(defaultDateAfter),
		sid, autoDLChecked,
		sid, removeBtn,
	)
}

// ── Actions cell ──────────────────────────────────────────────────────────────

func buildActionsCell(sourceID string, kind models.SourceKind, hasSchedule bool) string {
	sid := html.EscapeString(sourceID)
	deleteBtn := fmt.Sprintf(
		` <button class="btn-sm btn-orange" onclick="deleteSource(this,'%s')">Remove</button>`,
		sid,
	)
	schedBtn := ""
	if kind != models.SourceKindVideo && !hasSchedule {
		schedBtn = fmt.Sprintf(
			` <button class="btn-sm btn-green" onclick="withLoading(this,()=>quickSchedule('%s'))">Schedule</button>`,
			sid,
		)
	}
	if kind == models.SourceKindVideo {
		return fmt.Sprintf(
			`<div style="display:flex;gap:6px;align-items:center;flex-wrap:wrap">`+
				`<button class="btn-sm btn-gray" onclick="withLoading(this,()=>reindexSource('%s',''))">Reindex</button>%s`+
				`</div>`,
			sid, deleteBtn,
		)
	}
	return fmt.Sprintf(
		`<div style="display:flex;gap:6px;align-items:center;flex-wrap:wrap">`+
			`<select id="scan-date-%s" style="width:auto;padding:4px 8px;font-size:12px">%s</select>`+
			`<button class="btn-sm btn-gray" onclick="withLoading(this,()=>reindexSource('%s',document.getElementById('scan-date-%s').value))">Reindex</button>%s%s`+
			`</div>`,
		sid, buildDateAfterOptions(""),
		sid, sid, schedBtn, deleteBtn,
	)
}

// ── Main handler ──────────────────────────────────────────────────────────────

func (a *App) cookieStatusForUI() (exists bool, sizeStr string, modTimeStr string, validStatus string, validClass string) {
	ex, size, modTime := a.cookiesFileInfo()
	if !ex {
		return false, "", "", "Missing", "badge-failed"
	}
	sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
	modTimeStr = modTime.UTC().Format("2006-01-02 15:04 UTC")

	a.cookiesMu.Lock()
	v := a.cookiesValid
	msg := a.cookiesValidMsg
	_ = msg
	a.cookiesMu.Unlock()

	if v == nil {
		return true, sizeStr, modTimeStr, "Not tested", "badge-pending"
	}
	if *v {
		return true, sizeStr, modTimeStr, "Valid", "badge-succeeded"
	}
	return true, sizeStr, modTimeStr, "Expired / Invalid", "badge-failed"
}

func (a *App) handleIndex(w http.ResponseWriter, _ *http.Request) {
	sources := a.store.ListSources()
	tasks := a.store.ListTasks()
	schedules := a.store.ListSchedules()

	schedBySource := map[string]models.Schedule{}
	for _, s := range schedules {
		schedBySource[s.SourceID] = s
	}

	var allMedia []models.MediaItem
	for _, src := range sources {
		allMedia = append(allMedia, a.store.ListMediaItemsBySource(src.ID)...)
	}

	// ── Source rows ───────────────────────────────────────────────────────────
	var sourceRows []string
	for _, s := range sources {
		mediaItems := a.store.ListMediaItemsBySource(s.ID)
		mediaCount := len(mediaItems)
		doneCount := 0
		for _, m := range mediaItems {
			if m.Status == models.MediaItemStatusDone {
				doneCount++
			}
		}

		// Detail cell — show only title, kind badge, and URL as clickable link
		detail := fmt.Sprintf(
			`<div class="src-title" title="%s">%s</div>`+
				`<div class="tags">%s</div>`+
				`<div class="src-meta" title="%s"><a href="%s" target="_blank" style="color:#60a5fa">%s</a></div>`,
			html.EscapeString(s.Title), html.EscapeString(s.Title),
			tag(string(s.Kind)),
			html.EscapeString(s.URL), html.EscapeString(s.URL), html.EscapeString(s.URL),
		)

		// Media cell
		mediaInfo := fmt.Sprintf(`<span class="src-meta">%d indexed`, mediaCount)
		if doneCount > 0 {
			mediaInfo += fmt.Sprintf(`, %d downloaded`, doneCount)
		}
		mediaInfo += `</span>`
		viewBtn := fmt.Sprintf(
			`<br><button class="btn-sm btn-blue" style="margin-top:4px" onclick="viewMedia('%s')">View media</button>`,
			html.EscapeString(s.ID),
		)
		mediaCell := mediaInfo + viewBtn

		sourceRows = append(sourceRows, fmt.Sprintf(
			"<tr><td>%s</td><td>%s</td><td>%s</td></tr>",
			detail,
			mediaCell,
			buildActionsCell(s.ID, s.Kind, func() bool { _, ok := schedBySource[s.ID]; return ok }()),
		))
	}
	if len(sourceRows) == 0 {
		sourceRows = []string{`<tr><td colspan="3" class="empty">No sources yet.<br><span class="src-meta">Use the + New tab to add a YouTube source.</span></td></tr>`}
	}

	// ── Media filter options ──────────────────────────────────────────────────
	var filterOpts []string
	for _, src := range sources {
		title := src.Title
		if title == "" {
			title = src.ID
		}
		filterOpts = append(filterOpts, fmt.Sprintf(
			`<option value="%s">%s</option>`,
			html.EscapeString(src.ID), html.EscapeString(title),
		))
	}
	mediaFilterOptions := strings.Join(filterOpts, "")

	// ── Media rows ────────────────────────────────────────────────────────────
	var mediaRows []string
	for _, m := range allMedia {
		dur := "—"
		if m.Duration > 0 {
			h, min, sec := m.Duration/3600, (m.Duration%3600)/60, m.Duration%60
			if h > 0 {
				dur = fmt.Sprintf("%dh%dm%ds", h, min, sec)
			} else {
				dur = fmt.Sprintf("%dm%ds", min, sec)
			}
		}

		// Action buttons based on status
		action := ""
		mid := html.EscapeString(m.ID)
		switch m.Status {
		case models.MediaItemStatusFailed:
			action = fmt.Sprintf(
				`<button class="btn-sm btn-blue" onclick="withLoading(this,()=>retryMedia('%s'))">Retry</button>`+
					` <button class="btn-sm btn-orange" onclick="dismissMedia(this,'%s')">Remove</button>`,
				mid, mid)
		case models.MediaItemStatusPending:
			action = fmt.Sprintf(
				`<button class="btn-sm btn-blue" onclick="withLoading(this,()=>downloadMedia('%s'))">Download</button>`+
					` <button class="btn-sm btn-orange" onclick="dismissMedia(this,'%s')">Remove</button>`, mid, mid)
		case models.MediaItemStatusDone:
			action = fmt.Sprintf(
				`<button class="btn-sm btn-orange" onclick="dismissMedia(this,'%s')">Remove</button>`, mid)
		}

		mediaRows = append(mediaRows, fmt.Sprintf(
			`<tr data-source="%s"><td title="%s">%s</td><td class="src-meta">%s</td><td>%s</td><td>%s</td></tr>`,
			html.EscapeString(m.SourceID),
			html.EscapeString(m.Title), html.EscapeString(m.Title),
			html.EscapeString(dur),
			statusBadge(string(m.Status)),
			action,
		))
	}
	if len(mediaRows) == 0 {
		mediaRows = []string{`<tr><td colspan="4" class="empty">No media yet.<br><span class="src-meta">Add a source from the Sources tab — videos will appear here after scanning.</span></td></tr>`}
	}

	// ── Task rows ─────────────────────────────────────────────────────────────
	// Build lookup maps for human-readable entity names
	sourceNames := map[string]string{}
	for _, src := range sources {
		sourceNames[src.ID] = src.Title
	}
	mediaNames := map[string]string{}
	for _, m := range allMedia {
		mediaNames[m.ID] = m.Title
	}

	var taskRows []string
	for _, t := range tasks {
		// Human-readable type label
		typeLabel := "Index"
		if t.Type == "download" {
			typeLabel = "Download"
		}

		// Resolve entity to human name
		entityName := t.EntityID
		if t.EntityType == "source" {
			if name, ok := sourceNames[t.EntityID]; ok && name != "" {
				entityName = name
			}
		} else if t.EntityType == "media" {
			if name, ok := mediaNames[t.EntityID]; ok && name != "" {
				entityName = name
			}
		}

		// Action buttons
		action := ""
		if t.Status == models.TaskStatusQueued {
			action = fmt.Sprintf(
				`<button class="btn-sm btn-blue" onclick="withLoading(this,()=>startTask('%s'))">Start</button>`,
				html.EscapeString(t.ID),
			)
		}
		if t.Status == models.TaskStatusRunning {
			action = fmt.Sprintf(
				`<button class="btn-sm btn-orange" onclick="withLoading(this,()=>cancelTask('%s'))">Stop</button>`,
				html.EscapeString(t.ID),
			)
		}
		if t.Status == models.TaskStatusFailed {
			action += fmt.Sprintf(
				`<button class="btn-sm btn-blue" onclick="withLoading(this,()=>retryTask('%s'))">Retry</button> `,
				html.EscapeString(t.ID),
			)
		}
		if t.Status == models.TaskStatusSucceeded || t.Status == models.TaskStatusFailed {
			action += fmt.Sprintf(
				`<button class="btn-sm btn-orange" onclick="dismissTask(this,'%s')">Remove</button>`,
				html.EscapeString(t.ID),
			)
		}

		statusCell := statusBadge(string(t.Status))
		if t.Status == models.TaskStatusFailed && strings.Contains(t.ErrorMessage, "Sign in to confirm") {
			statusCell += ` <a href="#cookies" onclick="showTab('cookies')" style="color:#60a5fa;font-size:11px">→ Check Cookies</a>`
		}
		createdAt := t.CreatedAt.UTC().Format("2006-01-02 15:04")
		taskRows = append(taskRows, fmt.Sprintf(
			`<tr><td>%s</td><td title="%s">%s</td><td>%s</td><td class="src-meta">%s</td><td>%s</td></tr>`,
			html.EscapeString(typeLabel),
			html.EscapeString(entityName), html.EscapeString(entityName),
			statusCell,
			createdAt,
			action,
		))
	}
	if len(taskRows) == 0 {
		taskRows = []string{`<tr><td colspan="5" class="empty">No tasks yet</td></tr>`}
	}

	// ── Schedule rows ─────────────────────────────────────────────────────────
	var schedRows []string
	for _, sched := range schedules {
		src, _ := a.store.GetSource(sched.SourceID)
		srcTitle := src.Title
		if srcTitle == "" {
			srcTitle = sched.SourceID
		}
		state := statusBadge("disabled")
		if sched.Enabled {
			state = statusBadge("enabled")
		}
		dateLabel := "all time"
		if sched.DateAfter != "" {
			// Map the raw yt-dlp value to a human label
			for _, p := range dateAfterPresets {
				if p.v == sched.DateAfter {
					dateLabel = p.l
					break
				}
			}
			if dateLabel == "all time" {
				dateLabel = sched.DateAfter
			}
		}
		lastRun := "never"
		if sched.LastRunAt != nil {
			lastRun = sched.LastRunAt.UTC().Format("2006-01-02 15:04")
		}
		nextRun := sched.NextRunAt.UTC().Format("2006-01-02 15:04")
		sid := html.EscapeString(sched.SourceID)
		enableLabel := "Pause"
		enableClass := "btn-gray"
		if !sched.Enabled {
			enableLabel = "Start"
			enableClass = "btn-green"
		}
		autoDLChecked := ""
		if sched.AutoDownload {
			autoDLChecked = " checked"
		}
		actions := fmt.Sprintf(
			`<div style="display:flex;gap:4px;flex-wrap:wrap">`+
				`<button class="btn-sm %s" onclick="withLoading(this,()=>toggleSchedule('%s',%v))">%s</button>`+
				` <button class="btn-sm btn-blue" onclick="withLoading(this,()=>saveSchedule('%s'))">Save</button>`+
				` <button class="btn-sm btn-orange" onclick="removeSchedule(this,'%s')">Remove</button>`+
				`</div>`,
			enableClass, sid, !sched.Enabled, enableLabel,
			sid,
			sid,
		)
		schedRows = append(schedRows, fmt.Sprintf(
			`<tr><td title="%s">%s</td>`+
				`<td><select id="sched-iv-%s" style="width:auto;padding:3px;font-size:12px">%s</select></td>`+
				`<td><select id="sched-da-%s" style="width:auto;padding:3px;font-size:12px">%s</select></td>`+
				`<td><input type="checkbox" id="sched-ad-%s"%s style="width:auto;margin:0"></td>`+
				`<td>%s</td><td class="src-meta">%s</td><td class="src-meta">%s</td><td>%s</td></tr>`,
			html.EscapeString(srcTitle), html.EscapeString(srcTitle),
			sid, buildIntervalOptions(sched.IntervalHours),
			sid, buildDateAfterOptions(sched.DateAfter),
			sid, autoDLChecked,
			state, lastRun, nextRun, actions,
		))
	}
	if len(schedRows) == 0 {
		schedRows = []string{`<tr><td colspan="8" class="empty">No schedules — use the "Schedule" button on a channel or playlist source</td></tr>`}
	}

	// Check if any task is running (for auto-refresh)
	hasRunning := false
	for _, t := range tasks {
		if t.Status == models.TaskStatusRunning {
			hasRunning = true
			break
		}
	}
	autoRefreshJS := ""
	if hasRunning {
		autoRefreshJS = `(function poll(){
  const tab=location.hash.slice(1);
  if(tab==='settings'||tab==='cookies'||tab==='new')return;
  setTimeout(()=>location.reload(),5000);
})();`
	}

	// ── Page template ─────────────────────────────────────────────────────────
	page := `<!doctype html><html lang="en">
<head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>YtVault</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:Inter,system-ui,sans-serif;background:#0b1220;color:#e5e7eb;min-height:100vh}
nav{display:flex;align-items:center;gap:12px;padding:0 20px;height:52px;background:#0f172a;border-bottom:1px solid #1e293b;position:sticky;top:0;z-index:100}
.logo{font-size:17px;font-weight:700;color:#f1f5f9;margin-right:4px}
.nav-tabs{display:flex;gap:2px}
.nav-tab{background:none;border:none;color:#94a3b8;font-size:14px;font-weight:500;padding:5px 12px;border-radius:8px;cursor:pointer}
.nav-tab:hover{color:#e5e7eb;background:#1e293b}
.nav-tab.active{color:#60a5fa;background:#172033}
.nav-stats{margin-left:auto;display:flex;gap:6px}
.stat-pill{background:#172033;border:1px solid #1e3a5f;color:#93c5fd;font-size:11px;padding:2px 9px;border-radius:999px;cursor:pointer}.stat-pill:hover{background:#1e3a5f}
.main{padding:20px;max-width:1300px;margin:0 auto}
.tab-content{display:none}.tab-content.active{display:block}
.card{background:#121a2b;border:1px solid #1e293b;border-radius:12px;padding:18px;margin-bottom:18px}
.card-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:14px;flex-wrap:wrap;gap:8px}
.card-title{font-size:15px;font-weight:600;color:#f1f5f9}
.form-row{display:grid;grid-template-columns:140px 1fr;gap:10px;margin-bottom:10px;align-items:end}
.form-row-3{display:grid;grid-template-columns:140px 1fr 160px;gap:10px;margin-bottom:10px;align-items:end}
label{font-size:13px;color:#94a3b8;margin-bottom:3px;display:block}
select,input[type=text]{width:100%%;background:#0f172a;border:1px solid #334155;border-radius:8px;padding:7px 10px;color:#e5e7eb;font-size:13px}
select:focus,input:focus{outline:none;border-color:#3b82f6}
button{border:none;border-radius:8px;font-weight:600;font-size:13px;cursor:pointer}
.btn-primary{background:#2563eb;color:#fff;padding:8px 18px}
.btn-primary:hover{background:#1d4ed8}
.btn-sm{padding:3px 9px;font-size:12px}
.btn-blue{background:#1e3a5f;color:#93c5fd;border:1px solid #2d5a8e}
.btn-blue:hover{background:#1a3356}
.btn-green{background:#14532d;color:#86efac;border:1px solid #166534}
.btn-green:hover{background:#115225}
.btn-orange{background:#431407;color:#fb923c;border:1px solid #7c2d12}
.btn-orange:hover{background:#3a1006}
.btn-red{background:#450a0a;color:#fca5a5;border:1px solid #7f1d1d}
.btn-gray{background:#1e293b;color:#94a3b8;border:1px solid #334155}
.btn-gray:hover{background:#243044}
.table-wrap{overflow-x:auto}
table{width:100%%;border-collapse:collapse;font-size:13px}
th{color:#94a3b8;font-weight:500;padding:9px 8px;border-bottom:1px solid #1e293b;text-align:left;white-space:nowrap}
td{padding:9px 8px;border-bottom:1px solid #111827;vertical-align:top;max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
tr:last-child td{border-bottom:none}
tr:hover td{background:rgba(255,255,255,.02)}
.badge{display:inline-flex;align-items:center;border-radius:999px;padding:2px 8px;font-size:11px;font-weight:600;white-space:nowrap}
.badge-queued{background:#1e3a5f;color:#93c5fd}
.badge-running{background:#3b1d00;color:#fb923c}
.badge-succeeded{background:#14532d;color:#86efac}
.badge-failed{background:#3b0f0f;color:#fca5a5}
.badge-pending{background:#1e293b;color:#94a3b8}
.badge-downloading{background:#3b1d00;color:#fb923c}
.badge-done{background:#14532d;color:#86efac}
.badge-enabled{background:#14532d;color:#86efac}
.badge-disabled{background:#1e293b;color:#94a3b8}
.tag{display:inline-flex;align-items:center;background:#172033;border:1px solid #31415f;border-radius:999px;padding:2px 7px;font-size:11px;color:#bfdbfe}
.src-title{font-weight:600;color:#f1f5f9;margin-bottom:3px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.src-meta{color:#94a3b8;font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.tags{display:flex;gap:4px;flex-wrap:wrap;margin:3px 0}
.filter-bar{display:flex;align-items:center;gap:8px}
.filter-bar select{width:auto}
#toast{position:fixed;bottom:20px;right:20px;z-index:999;display:flex;flex-direction:column;gap:6px;pointer-events:none}
.toast-msg{background:#1e293b;border-radius:10px;padding:10px 14px;font-size:13px;max-width:340px;border-left:3px solid #3b82f6;pointer-events:auto;animation:tslide .2s ease}
.toast-ok{border-color:#22c55e;color:#86efac}
.toast-err{border-color:#ef4444;color:#fca5a5}
.toast-info{border-color:#3b82f6;color:#93c5fd}
@keyframes tslide{from{opacity:0;transform:translateY(6px)}to{opacity:1;transform:translateY(0)}}
.empty{color:#4b5563;text-align:center;padding:28px;font-size:13px}
.pager{display:flex;align-items:center;gap:8px;padding:8px 0;font-size:12px;color:#94a3b8}
.pager select{background:#0f172a;border:1px solid #334155;border-radius:6px;padding:3px 4px;color:#e5e7eb;font-size:11px;width:auto;min-width:0}
.pager button{background:#1e293b;border:1px solid #334155;border-radius:6px;padding:3px 10px;color:#94a3b8;font-size:12px;cursor:pointer}
.pager button:hover:not(:disabled){background:#243044;color:#e5e7eb}
.pager button:disabled{opacity:0.4;cursor:default}
@media(max-width:768px){.form-row,.form-row-3{grid-template-columns:1fr}.nav-stats{display:none}}
</style>
</head>
<body>
<nav>
  <span class="logo">YtVault</span>
  <div class="nav-tabs">
    <button class="nav-tab" id="tab-btn-new" onclick="showTab('new')">+ New</button>
    <button class="nav-tab" id="tab-btn-sources" onclick="showTab('sources')">Sources</button>
    <button class="nav-tab" id="tab-btn-media" onclick="showTab('media')">Media</button>
    <button class="nav-tab" id="tab-btn-tasks" onclick="showTab('tasks')">Tasks</button>
    <button class="nav-tab" id="tab-btn-schedules" onclick="showTab('schedules')">Schedules</button>
    <button class="nav-tab" id="tab-btn-cookies" onclick="showTab('cookies')">Cookies %s</button>
    <button class="nav-tab" id="tab-btn-settings" onclick="showTab('settings')">Settings</button>
  </div>
  <div class="nav-stats">
    <span class="stat-pill" onclick="showTab('sources')">%d sources</span>
    <span class="stat-pill" onclick="showTab('media')">%d media</span>
    <span class="stat-pill" onclick="showTab('tasks')">%d tasks</span>
    <span class="stat-pill" onclick="showTab('schedules')">%d schedules</span>
  </div>
</nav>
<div id="toast"></div>
<div class="main">

<div id="tab-new" class="tab-content">
  <div class="card">
    <div class="card-header"><span class="card-title">Search YouTube</span></div>
    <div style="display:flex;gap:8px;margin-bottom:10px">
      <input type="text" id="search-q" placeholder="Search by keywords…" style="flex:1" onkeydown="if(event.key==='Enter')searchYT()">
      <button class="btn-primary" onclick="withLoading(this,searchYT)">Search</button>
    </div>
    <div id="search-results" style="display:none">
      <div class="table-wrap"><table>
        <thead><tr><th>Title</th><th>Channel</th><th>Duration</th><th></th></tr></thead>
        <tbody id="search-tbody"></tbody>
      </table></div>
    </div>
  </div>
  <div class="card">
    <div class="card-header"><span class="card-title">Add Source</span></div>
    <input type="hidden" id="kind" value="video">
    <div class="form-row">
      <label>YouTube URL</label>
      <div style="display:flex;gap:8px;align-items:center">
        <input type="text" id="url" placeholder="https://www.youtube.com/..." style="flex:1" onkeydown="if(event.key==='Enter')submitForm()" oninput="autoDetectKind()">
        <span id="detected-kind" class="badge badge-pending" style="white-space:nowrap"></span>
      </div>
    </div>
    <div id="add-dateafter-wrap" style="display:none" class="form-row">
      <label>Date filter</label>
      <select id="add-dateafter">%s</select>
    </div>
    <div style="display:flex;align-items:center;gap:12px;flex-wrap:wrap;margin-bottom:12px">
      <label style="display:flex;align-items:center;gap:4px;font-size:12px;color:#94a3b8;cursor:pointer">
        <input type="checkbox" id="auto-download" style="width:auto;margin:0" checked> Auto-download after scan
      </label>
    </div>
    <div id="new-sched-wrap" style="display:none;margin-bottom:12px">
      <div style="font-size:13px;font-weight:600;color:#f1f5f9;margin-bottom:8px">Schedule (optional)</div>
      <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
        <label style="display:flex;align-items:center;gap:4px;font-size:12px;color:#94a3b8;cursor:pointer">
          <input type="checkbox" id="new-sched-enabled" style="width:auto;margin:0"> Enable recurring scan
        </label>
        <select id="new-sched-interval" style="width:auto;padding:4px 8px;font-size:12px">
          <option value="6">Every 6h</option><option value="12">Every 12h</option>
          <option value="24" selected>Every 24h</option><option value="168">Every week</option>
        </select>
      </div>
    </div>
    <div style="display:flex;align-items:center;gap:12px">
      <button class="btn-primary" onclick="withLoading(this,submitForm)">Add Source</button>
      <span id="add-status" class="src-meta"></span>
    </div>
  </div>
</div>

<div id="tab-sources" class="tab-content">
  <div class="card">
    <div class="card-header"><span class="card-title">Sources (%d)</span></div>
    <div class="table-wrap"><table>
      <thead><tr><th>Source</th><th>Media</th><th>Actions</th></tr></thead>
      <tbody id="sources-tbody">%s</tbody>
    </table></div>
    <div class="pager" id="pager-sources"></div>
  </div>
</div>

<div id="tab-media" class="tab-content">
  <div class="card">
    <div class="card-header">
      <span class="card-title">Media Items (%d)</span>
      <div class="filter-bar">
        <button class="btn-sm btn-green" onclick="withLoading(this,downloadAllPending)">Download All Pending</button>
        <span class="src-meta">Filter:</span>
        <select id="media-filter" onchange="filterMedia(this.value)">
          <option value="">All sources</option>%s
        </select>
      </div>
    </div>
    <div class="table-wrap"><table>
      <thead><tr><th>Title</th><th>Duration</th><th>Status</th><th>Actions</th></tr></thead>
      <tbody id="media-tbody">%s</tbody>
    </table></div>
    <div class="pager" id="pager-media"></div>
  </div>
</div>

<div id="tab-tasks" class="tab-content">
  <div class="card">
    <div class="card-header"><span class="card-title">Tasks (%d)</span><div style="display:flex;gap:6px"><button class="btn-sm btn-orange" onclick="withLoading(this,stopRunningTasks)">Stop running</button><button class="btn-sm btn-blue" onclick="withLoading(this,retryAllFailed)">Retry failed</button><button class="btn-sm btn-gray" onclick="withLoading(this,clearCompletedTasks)">Clear completed</button></div></div>
    <div class="table-wrap"><table>
      <thead><tr><th>Type</th><th>Entity</th><th>Status</th><th>Created</th><th></th></tr></thead>
      <tbody id="tasks-tbody">%s</tbody>
    </table></div>
    <div class="pager" id="pager-tasks"></div>
  </div>
</div>

<div id="tab-schedules" class="tab-content">
  <div class="card">
    <div class="card-header"><span class="card-title">Schedules (%d)</span></div>
    <div class="table-wrap"><table>
      <thead><tr><th>Source</th><th>Interval</th><th>Date filter</th><th>Auto-DL</th><th>State</th><th>Last run</th><th>Next run</th><th></th></tr></thead>
      <tbody id="schedules-tbody">%s</tbody>
    </table></div>
    <div class="pager" id="pager-schedules"></div>
  </div>
</div>

<div id="tab-cookies" class="tab-content">
  <div class="card">
    <div class="card-header"><span class="card-title">Cookie Status</span></div>
    <div style="display:grid;grid-template-columns:140px 1fr;gap:8px 16px;font-size:13px;margin-bottom:14px">
      <span class="src-meta">Status</span><span id="ck-status">%s</span>
      <span class="src-meta">File path</span><span class="src-meta">%s</span>
      <span class="src-meta">File size</span><span class="src-meta" id="ck-size">%s</span>
      <span class="src-meta">Last modified</span><span class="src-meta" id="ck-mtime">%s</span>
    </div>
    <button class="btn-sm btn-blue" onclick="validateCookies()" id="ck-test-btn">Test Now</button>
    <span id="ck-test-result" class="src-meta" style="margin-left:8px"></span>
    <p style="color:#64748b;font-size:11px;margin-top:10px">Cookies contain your YouTube session. Only upload on a trusted network.</p>
  </div>

  <div class="card">
    <div class="card-header"><span class="card-title">Upload Cookies</span></div>
    <p class="src-meta" style="margin-bottom:10px">Upload a <code>cookies.txt</code> file (Netscape format) or paste cookie content below.</p>
    <div style="margin-bottom:10px">
      <label style="display:inline-block;padding:6px 14px;background:#1e293b;border:1px solid #334155;border-radius:8px;cursor:pointer;font-size:13px;color:#93c5fd">
        Choose File <input type="file" id="ck-file" accept=".txt" style="display:none" onchange="document.getElementById('ck-filename').textContent=this.files[0]?.name||''">
      </label>
      <span id="ck-filename" class="src-meta" style="margin-left:8px"></span>
    </div>
    <div style="margin-bottom:10px">
      <label>Or paste content:</label>
      <textarea id="ck-paste" rows="6" style="width:100%%;background:#0f172a;border:1px solid #334155;border-radius:8px;padding:8px;color:#e5e7eb;font-family:monospace;font-size:12px;resize:vertical" placeholder="# Netscape HTTP Cookie File&#10;.youtube.com	TRUE	/	TRUE	..."></textarea>
    </div>
    <div style="display:flex;gap:8px;align-items:center">
      <button class="btn-primary" onclick="uploadCookies()">Save Cookies</button>
      <button class="btn-sm btn-orange" onclick="deleteCookies(this)">Delete Cookies</button>
      <span id="ck-upload-status" class="src-meta"></span>
    </div>
  </div>

  <div class="card">
    <div class="card-header"><span class="card-title">How to Get Cookies</span></div>
    <div style="font-size:13px;line-height:1.7">
      <details style="margin-bottom:10px">
        <summary style="cursor:pointer;color:#93c5fd;font-weight:600">Chrome / Edge / Brave / Arc (Chromium browsers)</summary>
        <ol style="padding-left:20px;margin-top:6px;color:#cbd5e1">
          <li>Install <a href="https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc" target="_blank" style="color:#60a5fa">Get cookies.txt LOCALLY</a> extension</li>
          <li>Go to <a href="https://www.youtube.com" target="_blank" style="color:#60a5fa">youtube.com</a> and make sure you are logged in</li>
          <li>Click the extension icon → Export → save or copy the content</li>
          <li>Upload the file or paste the content above</li>
        </ol>
      </details>
      <details style="margin-bottom:10px">
        <summary style="cursor:pointer;color:#93c5fd;font-weight:600">Firefox</summary>
        <ol style="padding-left:20px;margin-top:6px;color:#cbd5e1">
          <li>Install <a href="https://addons.mozilla.org/en-US/firefox/addon/cookies-txt/" target="_blank" style="color:#60a5fa">cookies.txt</a> add-on</li>
          <li>Go to <a href="https://www.youtube.com" target="_blank" style="color:#60a5fa">youtube.com</a> and make sure you are logged in</li>
          <li>Click the add-on → "Current Site" → save or copy the content</li>
          <li>Upload the file or paste the content above</li>
        </ol>
      </details>
      <details style="margin-bottom:10px">
        <summary style="cursor:pointer;color:#93c5fd;font-weight:600">Safari (macOS)</summary>
        <ol style="padding-left:20px;margin-top:6px;color:#cbd5e1">
          <li>Install yt-dlp locally: <code style="background:#1e293b;padding:2px 6px;border-radius:4px">brew install yt-dlp</code></li>
          <li>Run: <code style="background:#1e293b;padding:2px 6px;border-radius:4px">yt-dlp --cookies-from-browser safari --cookies cookies.txt --skip-download https://www.youtube.com</code></li>
          <li>Upload the generated <code>cookies.txt</code> file above</li>
        </ol>
      </details>
      <p class="src-meta" style="margin-top:8px">For more details see the <a href="https://github.com/yt-dlp/yt-dlp/wiki/FAQ#how-do-i-pass-cookies-to-yt-dlp" target="_blank" style="color:#60a5fa">yt-dlp cookie guide</a>.</p>
      <p class="src-meta" style="margin-top:4px">Cookies expire over time (usually weeks to months). If downloads start failing, re-export a fresh cookies file.</p>
    </div>
  </div>
</div>

<div id="tab-settings" class="tab-content">
  <div class="card">
    <div class="card-header"><span class="card-title">Download Settings</span></div>
    <div class="form-row">
      <label>Download directory</label>
      <span class="src-meta" id="cfg-dldir" style="padding:7px 0">Loading…</span>
    </div>
    <div class="form-row">
      <label>Folder name</label>
      <select id="cfg-folder">
        <option value="channel">Channel name</option>
        <option value="date">Release date (yyyy-mm-dd)</option>
        <option value="channel_year">Channel / Year (yyyy)</option>
        <option value="channel_month">Channel / Month (yyyy-mm)</option>
        <option value="channel_day">Channel / Date (yyyy-mm-dd)</option>
      </select>
    </div>
    <div class="form-row">
      <label>File name</label>
      <select id="cfg-file">
        <option value="date_title">Date + Title (yyyy-mm-dd Title.ext)</option>
        <option value="title">Title only (Title.ext)</option>
      </select>
    </div>
    <div style="margin:10px 0;padding:10px;background:#0f172a;border:1px solid #1e293b;border-radius:8px">
      <div style="font-size:13px;font-weight:600;color:#f1f5f9;margin-bottom:8px">Playlist overrides</div>
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:#94a3b8;cursor:pointer;margin-bottom:6px">
        <input type="checkbox" id="cfg-pl-folder" style="width:auto;margin:0" checked> Use playlist name as folder for playlist sources
      </label>
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:#94a3b8;cursor:pointer">
        <input type="checkbox" id="cfg-pl-file" style="width:auto;margin:0" checked> Use playlist index + title as filename for playlist sources
      </label>
    </div>
    <div class="form-row">
      <label>Video quality</label>
      <select id="cfg-format">
        <option value="bestvideo[ext=mp4]+bestaudio[ext=m4a]/bestvideo+bestaudio/best">Best (MP4 preferred)</option>
        <option value="bestvideo+bestaudio/best">Best (any format)</option>
        <option value="bestvideo[height<=1080]+bestaudio/best">Up to 1080p</option>
        <option value="bestvideo[height<=720]+bestaudio/best">Up to 720p</option>
        <option value="bestaudio/best">Audio only</option>
      </select>
    </div>
    <div class="form-row">
      <label>Max concurrent downloads</label>
      <select id="cfg-concurrent">
        <option value="1">1</option>
        <option value="2">2</option>
        <option value="3">3</option>
        <option value="5">5</option>
      </select>
    </div>
    <div style="margin-top:12px">
      <button class="btn-primary" onclick="withLoading(this,saveConfig)">Save Settings</button>
      <span id="cfg-status" class="src-meta" style="margin-left:8px"></span>
    </div>
  </div>
</div>

</div>
<script>
function showTab(name){
  document.querySelectorAll('.tab-content').forEach(el=>el.classList.remove('active'));
  document.querySelectorAll('.nav-tab').forEach(el=>el.classList.remove('active'));
  const c=document.getElementById('tab-'+name),b=document.getElementById('tab-btn-'+name);
  if(c)c.classList.add('active');
  if(b)b.classList.add('active');
  location.hash=name;
  if(name==='settings')loadConfig();
}
function toast(msg,type){
  const d=document.createElement('div');
  d.className='toast-msg toast-'+(type||'ok');
  d.textContent=msg;
  document.getElementById('toast').appendChild(d);
  setTimeout(()=>d.remove(),4000);
}
async function withLoading(btn,fn){
  const orig=btn.textContent;
  btn.disabled=true;btn.style.opacity='0.6';btn.textContent=orig+'…';
  try{await fn();}finally{btn.disabled=false;btn.style.opacity='';btn.textContent=orig;}
}
function onKindChange(kind){
  const isMulti=(kind==='channel'||kind==='playlist');
  document.getElementById('add-dateafter-wrap').style.display=isMulti?'':'none';
  const sw=document.getElementById('new-sched-wrap');
  if(sw)sw.style.display=isMulti?'':'none';
  const badge=document.getElementById('detected-kind');
  if(badge){
    badge.textContent=kind;
    badge.className='badge badge-'+(kind==='video'?'pending':kind==='channel'?'queued':'succeeded');
  }
}
function autoDetectKind(){
  const url=document.getElementById('url').value.trim().toLowerCase();
  const kind=document.getElementById('kind');
  if(url.includes('/playlist')||url.includes('list='))kind.value='playlist';
  else if(url.includes('/@')||url.includes('/channel/')||url.includes('/c/')||url.includes('/user/'))kind.value='channel';
  else kind.value='video';
  onKindChange(kind.value);
}
async function searchYT(){
  const q=document.getElementById('search-q').value.trim();
  if(!q){toast('Enter search keywords','info');return;}
  const res=await fetch('/api/v1/youtube/search?q='+encodeURIComponent(q));
  const data=await res.json();
  if(!res.ok){toast('Search failed: '+(data.error?.message||''),'err');return;}
  const tbody=document.getElementById('search-tbody');
  const wrap=document.getElementById('search-results');
  tbody.innerHTML='';
  if(!data.results||data.results.length===0){
    tbody.innerHTML='<tr><td colspan="4" class="empty">No results found</td></tr>';
    wrap.style.display='';return;
  }
  data.results.forEach(r=>{
    const dur=r.duration>0?(Math.floor(r.duration/60)+'m'+(r.duration%%60)+'s'):'—';
    const tr=document.createElement('tr');
    tr.innerHTML='<td title="'+r.title.replace(/"/g,'&quot;')+'">'+r.title+'</td>'+
      '<td class="src-meta" title="'+(r.channel||'').replace(/"/g,'&quot;')+'">'+(r.channel||'—')+'</td>'+
      '<td class="src-meta">'+dur+'</td>'+
      '<td><button class="btn-sm btn-blue" onclick="selectResult(\''+r.url.replace(/'/g,"\\'")+'\')">Select</button></td>';
    tbody.appendChild(tr);
  });
  wrap.style.display='';
}
function selectResult(url){
  document.getElementById('url').value=url;
  autoDetectKind();
  toast('URL selected — review options and click Add Source','ok');
}
async function submitForm(){
  const kind=document.getElementById('kind').value;
  const url=document.getElementById('url').value.trim();
  const dateAfterEl=document.getElementById('add-dateafter');
  const dateAfter=(kind!=='video'&&dateAfterEl)?dateAfterEl.value:'';
  const st=document.getElementById('add-status');
  if(!url){st.textContent='Please enter a URL';st.style.color='#fca5a5';return;}
  st.textContent='Adding...';st.style.color='#94a3b8';
  try{
    const autoDownload=document.getElementById('auto-download').checked;
    const body={kind,url,autoDownload};
    if(dateAfter)body.dateAfter=dateAfter;
    const res=await fetch('/api/v1/sources',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const data=await res.json();
    if(res.ok){
      // Auto-start the initial index task
      if(data.task?.id){
        await fetch('/api/v1/tasks/'+data.task.id+'/start',{method:'POST'}).catch(()=>{});
      }
      // Create schedule if enabled (channel/playlist only)
      const schedEnabled=document.getElementById('new-sched-enabled');
      if(schedEnabled&&schedEnabled.checked&&kind!=='video'&&data.source?.id){
        const interval=parseInt(document.getElementById('new-sched-interval').value)||24;
        await fetch('/api/v1/sources/'+data.source.id+'/schedule',{
          method:'POST',headers:{'Content-Type':'application/json'},
          body:JSON.stringify({intervalHours:interval,enabled:true,autoDownload:true,dateAfter:dateAfter})
        }).catch(()=>{});
      }
      toast('Source added — indexing now…','ok');
      setTimeout(()=>{location.hash='sources';location.reload();},400);
    }else{
      const m=data.error?.message||JSON.stringify(data);
      st.textContent=m;st.style.color='#fca5a5';
      toast('Failed: '+m,'err');
    }
  }catch(e){st.textContent='Network error';st.style.color='#fca5a5';toast('Network error','err');}
}
async function reindexSource(sourceId,dateAfter){
  try{
    const body=dateAfter?{dateAfter}:{};
    const res=await fetch('/api/v1/sources/'+sourceId+'/reindex',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const data=await res.json();
    if(res.ok||res.status===202){
      toast(dateAfter?'Reindexing ('+dateAfter+')…':'Reindexing…','ok');
      setTimeout(()=>location.reload(),300);
    }else{
      toast('Scan failed: '+(data.error?.message||JSON.stringify(data)),'err');
    }
  }catch(e){toast('Network error','err');}
}
async function startTask(id){
  try{
    const res=await fetch('/api/v1/tasks/'+id+'/start',{method:'POST'});
    const data=await res.json();
    if(res.ok||res.status===202){
      toast('Task started','ok');
      setTimeout(()=>location.reload(),300);
    }else{
      toast('Failed: '+(data.error?.message||JSON.stringify(data)),'err');
    }
  }catch(e){toast('Network error','err');}
}
async function downloadAll(sourceId){
  try{
    const res=await fetch('/api/v1/sources/'+sourceId+'/download-all',{method:'POST'});
    const data=await res.json();
    if(res.ok||res.status===202){
      const c=data.count||0,s=data.started||0;
      if(c===0){
        toast('No pending media to download','info');
      }else{
        toast('Started '+s+' of '+c+' download(s)','ok');
        if(data.startErrors?.length)toast(data.startErrors.length+' failed to start','err');
        setTimeout(()=>location.reload(),400);
      }
    }else{
      toast('Error: '+(data.error?.message||JSON.stringify(data)),'err');
    }
  }catch(e){toast('Network error','err');}
}
async function toggleSchedule(sourceId,enabled){
  const iv=document.getElementById('sched-iv-'+sourceId);
  const da=document.getElementById('sched-da-'+sourceId);
  const ad=document.getElementById('sched-ad-'+sourceId);
  const body={intervalHours:parseInt(iv?.value||24),enabled:enabled,autoDownload:ad?.checked||false,dateAfter:da?.value||''};
  const res=await fetch('/api/v1/sources/'+sourceId+'/schedule',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  if(res.ok){toast(enabled?'Schedule started':'Schedule paused','ok');setTimeout(()=>location.reload(),300);}
  else{const d=await res.json();toast('Failed: '+(d.error?.message||''),'err');}
}
async function saveSchedule(sourceId){
  const iv=document.getElementById('sched-iv-'+sourceId);
  const da=document.getElementById('sched-da-'+sourceId);
  const ad=document.getElementById('sched-ad-'+sourceId);
  const body={intervalHours:parseInt(iv?.value||24),enabled:true,autoDownload:ad?.checked||false,dateAfter:da?.value||''};
  const res=await fetch('/api/v1/sources/'+sourceId+'/schedule',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  if(res.ok){toast('Schedule saved','ok');setTimeout(()=>location.reload(),300);}
  else{const d=await res.json();toast('Failed: '+(d.error?.message||''),'err');}
}
async function setSchedule(sourceId){
  const iv=document.getElementById('sched-interval-'+sourceId);
  const ad=document.getElementById('sched-autodl-'+sourceId);
  const da=document.getElementById('sched-dateafter-'+sourceId);
  const body={intervalHours:parseInt(iv.value),enabled:true,autoDownload:ad.checked,dateAfter:da?da.value:''};
  try{
    const res=await fetch('/api/v1/sources/'+sourceId+'/schedule',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const data=await res.json();
    if(res.ok){
      toast('Schedule set: every '+data.schedule.intervalHours+'h','ok');
      setTimeout(()=>location.reload(),300);
    }else{
      toast('Failed: '+(data.error?.message||JSON.stringify(data)),'err');
    }
  }catch(e){toast('Network error','err');}
}
function removeSchedule(btn,sourceId){
  if(btn.dataset.confirm!=='1'){
    btn.dataset.confirm='1';
    btn.textContent='Confirm?';
    btn.className='btn-sm btn-red';
    setTimeout(()=>{
      if(btn.dataset.confirm==='1'){
        btn.dataset.confirm='0';
        btn.textContent='Remove';
        btn.className='btn-sm btn-orange';
      }
    },3000);
    return;
  }
  btn.disabled=true;btn.textContent='Removing...';
  fetch('/api/v1/sources/'+sourceId+'/schedule',{method:'DELETE'}).then(res=>{
    if(res.ok||res.status===204){
      toast('Schedule removed','ok');
      setTimeout(()=>location.reload(),300);
    }else{
      res.json().then(d=>toast('Error: '+(d.error?.message||JSON.stringify(d)),'err'));
      btn.disabled=false;btn.textContent='Remove';btn.dataset.confirm='0';
    }
  }).catch(e=>{toast('Network error','err');btn.disabled=false;});
}
function filterMedia(sourceId){
  sessionStorage.setItem('mediaFilter',sourceId);
  if(window.pagers&&window.pagers['media-tbody']){
    window.pagers['media-tbody'].setFilter(tr=>sourceId===''||tr.dataset.source===sourceId);
  }else{
    document.querySelectorAll('#media-tbody tr[data-source]').forEach(tr=>{
      tr.style.display=(sourceId===''||tr.dataset.source===sourceId)?'':'none';
    });
  }
}
function viewMedia(sourceId){
  showTab('media');
  const f=document.getElementById('media-filter');
  if(f){f.value=sourceId;filterMedia(sourceId);}
}
async function downloadMedia(mediaId){
  const res=await fetch('/api/v1/media/'+mediaId+'/download',{method:'POST'});
  if(res.ok||res.status===202){toast('Download started','ok');setTimeout(()=>location.reload(),400);}
  else{const d=await res.json();toast('Failed: '+(d.error?.message||''),'err');}
}
async function downloadAllPending(){
  const filter=sessionStorage.getItem('mediaFilter')||'';
  const qs=filter?'?source='+encodeURIComponent(filter):'';
  const res=await fetch('/api/v1/media/download-all-pending'+qs,{method:'POST'});
  const data=await res.json();
  if(res.ok||res.status===202){
    toast('Started '+(data.started||0)+' of '+(data.total||0)+' download(s)','ok');
    if(data.total>0)setTimeout(()=>location.reload(),400);
  }else{toast('Failed','err');}
}
async function retryTask(taskId){
  const res=await fetch('/api/v1/tasks/'+taskId+'/retry',{method:'POST'});
  if(res.ok||res.status===202){toast('Retrying…','ok');setTimeout(()=>location.reload(),400);}
  else{const d=await res.json();toast('Failed: '+(d.error?.message||''),'err');}
}
async function cancelTask(taskId){
  const res=await fetch('/api/v1/tasks/'+taskId+'/cancel',{method:'POST'});
  if(res.ok){toast('Stopping task…','ok');setTimeout(()=>location.reload(),1000);}
  else{const d=await res.json();toast('Failed: '+(d.error?.message||''),'err');}
}
async function retryMedia(mediaId){
  const res=await fetch('/api/v1/media/'+mediaId+'/retry',{method:'POST'});
  if(res.ok||res.status===202){toast('Retrying download…','ok');setTimeout(()=>location.reload(),400);}
  else{const d=await res.json();toast('Retry failed: '+(d.error?.message||''),'err');}
}
function dismissMedia(btn,mediaId){
  if(btn.dataset.confirm!=='1'){
    btn.dataset.confirm='1';btn.textContent='Confirm?';btn.className='btn-sm btn-red';
    setTimeout(()=>{if(btn.dataset.confirm==='1'){btn.dataset.confirm='0';btn.textContent='Remove';btn.className='btn-sm btn-orange';}},3000);
    return;
  }
  btn.disabled=true;btn.textContent='Removing…';
  fetch('/api/v1/media/'+mediaId,{method:'DELETE'}).then(res=>{
    if(res.ok||res.status===204){btn.closest('tr').remove();toast('Removed','ok');}
    else{btn.disabled=false;btn.textContent='Remove';btn.className='btn-sm btn-orange';btn.dataset.confirm='0';toast('Failed','err');}
  }).catch(()=>{btn.disabled=false;});
}
function deleteSource(btn,sourceId){
  if(btn.dataset.confirm!=='1'){
    btn.dataset.confirm='1';btn.textContent='Confirm?';btn.className='btn-sm btn-red';
    setTimeout(()=>{if(btn.dataset.confirm==='1'){btn.dataset.confirm='0';btn.textContent='Remove';btn.className='btn-sm btn-orange';}},3000);
    return;
  }
  btn.disabled=true;btn.textContent='Removing…';
  fetch('/api/v1/sources/'+sourceId,{method:'DELETE'}).then(res=>{
    if(res.ok||res.status===204){toast('Source removed','ok');setTimeout(()=>location.reload(),300);}
    else{res.json().then(d=>toast('Error: '+(d.error?.message||''),'err'));btn.disabled=false;btn.textContent='Remove';btn.dataset.confirm='0';}
  }).catch(()=>{toast('Network error','err');btn.disabled=false;});
}
function dismissTask(btn,taskId){
  if(btn.dataset.confirm!=='1'){
    btn.dataset.confirm='1';btn.textContent='Confirm?';btn.className='btn-sm btn-red';
    setTimeout(()=>{if(btn.dataset.confirm==='1'){btn.dataset.confirm='0';btn.textContent='Remove';btn.className='btn-sm btn-orange';}},3000);
    return;
  }
  btn.disabled=true;btn.textContent='Removing…';
  fetch('/api/v1/tasks/'+taskId,{method:'DELETE'}).then(res=>{
    if(res.ok||res.status===204){btn.closest('tr').remove();toast('Removed','ok');}
    else{btn.disabled=false;btn.textContent='Remove';btn.className='btn-sm btn-orange';btn.dataset.confirm='0';toast('Failed','err');}
  }).catch(()=>{btn.disabled=false;});
}
async function quickSchedule(sourceId){
  const body={intervalHours:24,enabled:true,autoDownload:true,dateAfter:''};
  const res=await fetch('/api/v1/sources/'+sourceId+'/schedule',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  if(res.ok){toast('Scheduled: every 24h with auto-download','ok');setTimeout(()=>location.reload(),300);}
  else{const d=await res.json();toast('Failed: '+(d.error?.message||''),'err');}
}
async function stopRunningTasks(){
  const res=await fetch('/api/v1/tasks/stop-all',{method:'POST'});
  const data=await res.json();
  toast('Stopped '+(data.stopped||0)+' task(s)','ok');
  setTimeout(()=>location.reload(),1000);
}
async function retryAllFailed(){
  const res=await fetch('/api/v1/tasks/retry-failed',{method:'POST'});
  const data=await res.json();
  toast('Retried '+(data.retried||0)+' task(s)','ok');
  if(data.retried>0)setTimeout(()=>location.reload(),400);
}
async function clearCompletedTasks(){
  const res=await fetch('/api/v1/tasks',{method:'DELETE'});
  const data=await res.json();
  toast('Cleared '+(data.removed||0)+' completed task(s)','ok');
  setTimeout(()=>location.reload(),300);
}
async function validateCookies(){
  const btn=document.getElementById('ck-test-btn');
  const res_el=document.getElementById('ck-test-result');
  btn.disabled=true;btn.textContent='Testing...';res_el.textContent='';
  try{
    const res=await fetch('/api/v1/cookies/validate',{method:'POST'});
    const data=await res.json();
    if(data.valid){
      res_el.textContent='Valid!';res_el.style.color='#86efac';
      toast('Cookies are valid','ok');
    }else{
      res_el.textContent='Invalid: '+(data.message||'unknown');res_el.style.color='#fca5a5';
      toast('Cookies invalid','err');
    }
    setTimeout(()=>location.reload(),2000);
  }catch(e){res_el.textContent='Network error';res_el.style.color='#fca5a5';}
  btn.disabled=false;btn.textContent='Test Now';
}
async function uploadCookies(){
  const fileInput=document.getElementById('ck-file');
  const paste=document.getElementById('ck-paste').value.trim();
  const st=document.getElementById('ck-upload-status');
  let body,ct;
  if(fileInput.files.length>0){
    body=new FormData();body.append('file',fileInput.files[0]);ct=undefined;
  }else if(paste){
    body=paste;ct='text/plain';
  }else{
    st.textContent='Choose a file or paste content';st.style.color='#fca5a5';return;
  }
  st.textContent='Uploading...';st.style.color='#94a3b8';
  try{
    const opts={method:'POST',body};
    if(ct)opts.headers={'Content-Type':ct};
    const res=await fetch('/api/v1/cookies',opts);
    const data=await res.json();
    if(res.ok){
      toast('Cookies saved — validating…','ok');
      fetch('/api/v1/cookies/validate',{method:'POST'}).then(r=>r.json()).then(d=>{
        if(d.valid)toast('Cookies are valid!','ok');
        else toast('Cookies may be expired: '+(d.message||''),'err');
      }).catch(()=>{});
      setTimeout(()=>location.reload(),2000);
    }else{
      st.textContent=data.error?.message||'Upload failed';st.style.color='#fca5a5';
      toast('Failed: '+(data.error?.message||'unknown'),'err');
    }
  }catch(e){st.textContent='Network error';st.style.color='#fca5a5';toast('Network error','err');}
}
function deleteCookies(btn){
  if(btn.dataset.confirm!=='1'){
    btn.dataset.confirm='1';btn.textContent='Confirm?';btn.className='btn-sm btn-red';
    setTimeout(()=>{if(btn.dataset.confirm==='1'){btn.dataset.confirm='0';btn.textContent='Delete Cookies';btn.className='btn-sm btn-orange';}},3000);
    return;
  }
  fetch('/api/v1/cookies',{method:'DELETE'}).then(res=>{
    if(res.ok||res.status===204){toast('Cookies deleted','ok');setTimeout(()=>location.reload(),300);}
    else{toast('Delete failed','err');}
  }).catch(()=>toast('Network error','err'));
}
function initPager(tbodyId,pagerId,defaultSize){
  const tbody=document.getElementById(tbodyId);
  const pager=document.getElementById(pagerId);
  if(!tbody||!pager)return;
  let page=0,size=defaultSize||20;
  function getRows(){return Array.from(tbody.querySelectorAll('tr')).filter(r=>r.style.display!=='none'||r.dataset.source!==undefined);}
  function allRows(){return Array.from(tbody.querySelectorAll('tr'));}
  function render(){
    const rows=allRows();
    const visibleRows=rows.filter(r=>!r.dataset.hidden);
    const total=visibleRows.length;
    const pages=Math.max(1,Math.ceil(total/size));
    if(page>=pages)page=pages-1;
    let vi=0;
    rows.forEach(r=>{
      if(r.dataset.hidden==='1'){r.style.display='none';return;}
      if(vi>=page*size&&vi<(page+1)*size){r.style.display='';}
      else{r.style.display='none';}
      vi++;
    });
    pager.innerHTML='<button onclick="pagers[\''+tbodyId+'\'].prev()">Prev</button>'+
      '<span>Page '+(page+1)+' of '+pages+'</span>'+
      '<button onclick="pagers[\''+tbodyId+'\'].next()">Next</button>'+
      '<select onchange="pagers[\''+tbodyId+'\'].setSize(+this.value)">'+
      [5,10,20,50,100].map(n=>'<option value="'+n+'"'+(n===size?' selected':'')+'>'+n+'</option>').join('')+
      '</select>';
    pager.querySelector('button').disabled=(page===0);
    pager.querySelectorAll('button')[1].disabled=(page>=pages-1);
  }
  const api={
    prev(){if(page>0){page--;render();}},
    next(){page++;render();},
    setSize(n){size=n;page=0;render();},
    refresh(){render();},
    setFilter(fn){allRows().forEach(r=>{r.dataset.hidden=fn(r)?'':'1';});page=0;render();}
  };
  if(!window.pagers)window.pagers={};
  window.pagers[tbodyId]=api;
  render();
  return api;
}
async function loadConfig(){
  try{
    const res=await fetch('/api/v1/config');
    const data=await res.json();
    document.getElementById('cfg-dldir').textContent=data.downloadDir||'—';
    const c=data.config||{};
    const setSelect=(id,val)=>{const el=document.getElementById(id);for(let i=0;i<el.options.length;i++){if(el.options[i].value===val){el.selectedIndex=i;break;}}};
    setSelect('cfg-folder',c.folderFormat||'channel');
    setSelect('cfg-file',c.fileFormat||'date_title');
    setSelect('cfg-format',c.videoFormat||'');
    setSelect('cfg-concurrent',String(c.maxConcurrent||1));
    document.getElementById('cfg-pl-folder').checked=c.playlistFolderOverride!==false;
    document.getElementById('cfg-pl-file').checked=c.playlistFileOverride!==false;
  }catch(e){}
}
async function saveConfig(){
  const body={
    folderFormat:document.getElementById('cfg-folder').value,
    fileFormat:document.getElementById('cfg-file').value,
    playlistFolderOverride:document.getElementById('cfg-pl-folder').checked,
    playlistFileOverride:document.getElementById('cfg-pl-file').checked,
    videoFormat:document.getElementById('cfg-format').value,
    maxConcurrent:parseInt(document.getElementById('cfg-concurrent').value)||1
  };
  const res=await fetch('/api/v1/config',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  if(res.ok){toast('Settings saved','ok');}else{toast('Failed to save','err');}
}
window.onload=function(){
  const h=location.hash.slice(1);
  showTab(['new','sources','media','tasks','schedules','cookies','settings'].includes(h)?h:'sources');
  // Restore scroll position after reload
  const sy=sessionStorage.getItem('scrollY');
  if(sy){window.scrollTo(0,parseInt(sy));sessionStorage.removeItem('scrollY');}
  // Auto-validate cookies if not recently tested
  fetch('/api/v1/cookies/status').then(r=>r.json()).then(d=>{
    if(d.exists&&!d.valid&&!d.validatedAt){
      fetch('/api/v1/cookies/validate',{method:'POST'}).catch(()=>{});
    }
  }).catch(()=>{});
  // Init pagers
  initPager('sources-tbody','pager-sources',20);
  initPager('media-tbody','pager-media',20);
  initPager('tasks-tbody','pager-tasks',20);
  initPager('schedules-tbody','pager-schedules',20);
  // Restore media filter
  const mf=sessionStorage.getItem('mediaFilter');
  if(mf){const el=document.getElementById('media-filter');if(el){el.value=mf;filterMedia(mf);}}
  %s
};
window.onbeforeunload=function(){sessionStorage.setItem('scrollY',window.scrollY);};
</script>
</body></html>`

	addDateAfterOpts := buildDateAfterOptions("")

	// Cookie status for nav badge and cookies tab
	ckExists, ckSize, ckMtime, ckStatus, ckClass := a.cookieStatusForUI()
	ckNavBadge := fmt.Sprintf(`<span class="badge %s" style="margin-left:4px;font-size:9px">%s</span>`, ckClass, html.EscapeString(ckStatus))
	ckPathDisplay := html.EscapeString(a.cookiesPath)
	ckStatusBadge := fmt.Sprintf(`<span class="badge %s">%s</span>`, ckClass, html.EscapeString(ckStatus))
	if !ckExists {
		ckSize = "—"
		ckMtime = "—"
	}

	_, _ = w.Write([]byte(fmt.Sprintf(page,
		ckNavBadge,                                                       // cookies nav badge
		len(sources), len(allMedia), len(tasks), len(schedules),         // nav stats
		addDateAfterOpts,                                                // add-source date filter options
		len(sources), strings.Join(sourceRows, ""),                      // sources tab: title, rows
		len(allMedia), mediaFilterOptions, strings.Join(mediaRows, ""), // media tab
		len(tasks), strings.Join(taskRows, ""),                         // tasks tab
		len(schedules), strings.Join(schedRows, ""),                    // schedules tab
		ckStatusBadge, ckPathDisplay, ckSize, ckMtime,                  // cookies tab: status, path, size, mtime
		autoRefreshJS,                                                   // auto-refresh if running tasks
	)))
}
