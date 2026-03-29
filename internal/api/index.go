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
			`<label style="display:flex;align-items:center;gap:4px;font-size:12px;color:var(--text-secondary);cursor:pointer">`+
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

var retentionOptions = []struct{ v int; l string }{
	{0, "Default"},
	{30, "30 days"},
	{60, "60 days"},
	{90, "90 days"},
	{180, "180 days"},
	{365, "1 year"},
	{-1, "Keep forever"},
}

func buildRetentionSelect(sourceID string, current int) string {
	sid := html.EscapeString(sourceID)
	var b strings.Builder
	fmt.Fprintf(&b, `<select id="ret-%s" style="width:auto;padding:3px;font-size:11px" onchange="setRetention('%s',this.value)">`, sid, sid)
	for _, o := range retentionOptions {
		sel := ""
		if o.v == current {
			sel = ` selected`
		}
		fmt.Fprintf(&b, `<option value="%d"%s>%s</option>`, o.v, sel, html.EscapeString(o.l))
	}
	b.WriteString(`</select>`)
	return b.String()
}

func buildActionsCell(sourceID string, kind models.SourceKind, hasSchedule bool, retentionDays int, dateAfter string) string {
	sid := html.EscapeString(sourceID)
	if kind == models.SourceKindVideo {
		return fmt.Sprintf(
			`<div style="display:flex;gap:6px;align-items:center;flex-wrap:wrap">`+
				`<button class="btn-sm btn-gray" title="Re-scan this video" onclick="withLoading(this,()=>reindexSource('%s',''))">Reindex</button>`+
				` <button class="btn-sm btn-orange" title="Remove this source and its media" onclick="deleteSource(this,'%s')">Remove</button>`+
				`</div>`,
			sid, sid,
		)
	}
	schedBtn := ""
	if !hasSchedule {
		schedBtn = fmt.Sprintf(
			` <button class="btn-sm btn-green" title="Set up recurring scan every 24h" onclick="withLoading(this,()=>quickSchedule('%s'))">Schedule</button>`,
			sid,
		)
	}
	return fmt.Sprintf(
		`<div style="display:flex;gap:6px;align-items:center;flex-wrap:wrap">`+
			`<span class="src-meta" style="font-size:11px">Scan:</span>`+
			`<select id="scan-date-%s" style="width:auto;padding:3px 6px;font-size:11px" title="Date range for reindexing">%s</select>`+
			`<button class="btn-sm btn-gray" title="Scan for new videos" onclick="withLoading(this,()=>reindexSource('%s',document.getElementById('scan-date-%s').value))">Reindex</button>`+
			`%s`+
			`</div>`+
			`<div style="display:flex;gap:6px;align-items:center;margin-top:6px">`+
			`<span class="src-meta" style="font-size:11px">Retention:</span>`+
			`%s`+
			`<button class="btn-sm btn-gray" title="Delete old downloaded media based on retention" onclick="cleanupSource('%s')">Clean up</button>`+
			` <button class="btn-sm btn-orange" title="Remove this source and all its media" onclick="deleteSource(this,'%s')">Remove</button>`+
			`</div>`,
		sid, buildDateAfterOptions(dateAfter),
		sid, sid,
		schedBtn,
		buildRetentionSelect(sourceID, retentionDays),
		sid, sid,
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
	// Verify downloaded files still exist on disk
	a.store.VerifyMediaFiles()

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

		// Progress percentage
		pct := 0
		if mediaCount > 0 {
			pct = doneCount * 100 / mediaCount
		}

		kindClass := "tag-video"
		if s.Kind == models.SourceKindChannel {
			kindClass = "tag-channel"
		} else if s.Kind == models.SourceKindPlaylist {
			kindClass = "tag-playlist"
		}

		sourceRows = append(sourceRows, fmt.Sprintf(
			`<div class="source-card">`+
				`<div class="source-header">`+
				`<div class="src-title" title="%s">%s</div>`+
				`<span class="tag %s">%s</span>`+
				`<button class="btn-sm btn-blue" onclick="viewMedia('%s')" style="flex-shrink:0">View</button>`+
				`</div>`+
				`<a href="%s" target="_blank" class="source-url" title="%s">%s</a>`+
				`<div style="display:flex;align-items:center;gap:10px;margin-top:4px">`+
				`<div class="progress-bar" style="flex:1"><div class="fill" style="width:%d%%"></div></div>`+
				`<span class="stat-num">%d / %d</span>`+
				`</div>`+
				`<div class="card-actions">%s</div>`+
				`</div>`,
			html.EscapeString(s.Title), html.EscapeString(s.Title),
			kindClass, html.EscapeString(string(s.Kind)),
			html.EscapeString(s.ID),
			html.EscapeString(s.URL), html.EscapeString(s.URL), html.EscapeString(s.URL),
			pct, doneCount, mediaCount,
			buildActionsCell(s.ID, s.Kind,
				func() bool { _, ok := schedBySource[s.ID]; return ok }(),
				s.RetentionDays,
				func() string { if sc, ok := schedBySource[s.ID]; ok { return sc.DateAfter }; return "" }(),
			),
		))
	}
	if len(sourceRows) == 0 {
		sourceRows = []string{`<div class="empty-state"><div class="empty-icon">&#128250;</div><div class="empty-text">No sources yet</div><div class="empty-sub">Click + New Source in the sidebar to get started</div></div>`}
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

		// Pin toggle + action buttons
		mid := html.EscapeString(m.ID)
		pinIcon := "☆"
		pinTitle := "Pin (protect from cleanup)"
		if m.Pinned {
			pinIcon = "★"
			pinTitle = "Unpin"
		}
		pinBtn := fmt.Sprintf(
			`<span style="cursor:pointer;font-size:16px;color:%s" title="%s" onclick="togglePin('%s',this)">%s</span> `,
			func() string { if m.Pinned { return "#fbbf24" }; return "#4b5563" }(),
			pinTitle, mid, pinIcon,
		)
		action := pinBtn
		switch m.Status {
		case models.MediaItemStatusFailed:
			action += fmt.Sprintf(
				`<button class="btn-sm btn-blue" onclick="withLoading(this,()=>retryMedia('%s'))">Retry</button>`+
					` <button class="btn-sm btn-orange" onclick="dismissMedia(this,'%s')">Remove</button>`,
				mid, mid)
		case models.MediaItemStatusPending:
			action += fmt.Sprintf(
				`<button class="btn-sm btn-blue" onclick="withLoading(this,()=>downloadMedia('%s'))">Download</button>`+
					` <button class="btn-sm btn-orange" onclick="dismissMedia(this,'%s')">Remove</button>`, mid, mid)
		case models.MediaItemStatusDone:
			action += fmt.Sprintf(
				`<button class="btn-sm btn-orange" onclick="dismissMedia(this,'%s')">Remove</button>`, mid)
		case models.MediaItemStatusMissing:
			action += fmt.Sprintf(
				`<button class="btn-sm btn-blue" onclick="withLoading(this,()=>downloadMedia('%s'))">Re-download</button>`+
					` <button class="btn-sm btn-orange" onclick="dismissMedia(this,'%s')">Remove</button>`, mid, mid)
		}

		// Status icon for thumbnail overlay
		stClass := "st-pending"
		stIcon := "&#9679;"
		switch m.Status {
		case models.MediaItemStatusDone:
			stClass = "st-done"; stIcon = "&#10003;"
		case models.MediaItemStatusFailed:
			stClass = "st-failed"; stIcon = "&#10007;"
		case models.MediaItemStatusDownloading:
			stClass = "st-downloading"; stIcon = "&#8595;"
		case models.MediaItemStatusMissing:
			stClass = "st-missing"; stIcon = "?"
		}

		// Build metadata line: duration, file size, upload date, download date
		metaParts := []string{}
		if m.Channel != "" {
			metaParts = append(metaParts, m.Channel)
		}
		metaParts = append(metaParts, dur)
		if m.FileSize > 0 {
			if m.FileSize >= 1<<30 {
				metaParts = append(metaParts, fmt.Sprintf("%.1f GB", float64(m.FileSize)/float64(1<<30)))
			} else if m.FileSize >= 1<<20 {
				metaParts = append(metaParts, fmt.Sprintf("%.1f MB", float64(m.FileSize)/float64(1<<20)))
			} else {
				metaParts = append(metaParts, fmt.Sprintf("%.0f KB", float64(m.FileSize)/1024))
			}
		}
		if m.UploadDate != nil {
			metaParts = append(metaParts, "uploaded "+m.UploadDate.Format("2006-01-02"))
		}
		if m.Status == models.MediaItemStatusDone && !m.UpdatedAt.IsZero() {
			metaParts = append(metaParts, "downloaded "+m.UpdatedAt.Format("2006-01-02"))
		}
		metaLine := html.EscapeString(strings.Join(metaParts, " · "))

		// File path tooltip with copy-on-click
		filePathAttr := ""
		if m.FilePath != "" {
			filePathAttr = fmt.Sprintf(` data-filepath="%s" title="%s — click to copy path" onclick="copyPath(this)" style="cursor:pointer"`,
				html.EscapeString(m.FilePath), html.EscapeString(m.FilePath))
		}

		mediaRows = append(mediaRows, fmt.Sprintf(
			`<div class="media-row" data-source="%s" data-status="%s">`+
				`<div class="thumb-wrap"><img src="https://img.youtube.com/vi/%s/mqdefault.jpg" loading="lazy" onerror="this.style.display='none'"><div class="thumb-status %s">%s</div></div>`+
				`<div class="media-info"%s><span class="m-title" title="%s">%s</span><span class="m-sub">%s</span><span class="m-sub">%s</span></div>`+
				`<div class="row-actions">%s</div></div>`,
			html.EscapeString(m.SourceID), html.EscapeString(string(m.Status)),
			html.EscapeString(m.VideoID), stClass, stIcon,
			filePathAttr, html.EscapeString(m.Title), html.EscapeString(m.Title),
			metaLine, statusBadge(string(m.Status)),
			action,
		))
	}
	if len(mediaRows) == 0 {
		mediaRows = []string{`<div class="empty-state"><div class="empty-icon">&#9654;</div><div class="empty-text">No media yet</div><div class="empty-sub">Add a source from + New Source — videos will appear here after scanning</div></div>`}
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
		if t.Status == models.TaskStatusFailed {
			errMsg := html.EscapeString(t.ErrorMessage)
			statusCell += fmt.Sprintf(
				` <span title="%s — click to copy" style="cursor:pointer;font-size:11px;color:var(--danger-text);text-decoration:underline dotted" onclick="copyText(this.title.split(' — ')[0])">details</span>`,
				errMsg,
			)
			if strings.Contains(t.ErrorMessage, "Sign in to confirm") {
				statusCell += ` <a href="#cookies" onclick="showTab('cookies')" style="color:#60a5fa;font-size:11px">→ Cookies</a>`
			}
			if strings.Contains(t.ErrorMessage, "verify your age") || strings.Contains(t.ErrorMessage, "age") {
				statusCell += ` <span class="src-meta" style="font-size:11px" title="Verify your age at myaccount.google.com/birthday, then re-export cookies">age-restricted</span>`
			}
		}
		createdAt := t.CreatedAt.UTC().Format("2006-01-02 15:04")
		taskRows = append(taskRows, fmt.Sprintf(
			`<tr data-status="%s"><td>%s</td><td title="%s">%s</td><td>%s</td><td class="src-meta">%s</td><td class="row-actions">%s</td></tr>`,
			html.EscapeString(string(t.Status)),
			html.EscapeString(typeLabel),
			html.EscapeString(entityName), html.EscapeString(entityName),
			statusCell,
			createdAt,
			action,
		))
	}
	if len(taskRows) == 0 {
		taskRows = []string{`<tr><td colspan="5"><div class="empty-state"><div class="empty-icon">&#9889;</div><div class="empty-text">All systems go</div><div class="empty-sub">No active tasks</div></div></td></tr>`}
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
				` <button class="btn-sm btn-orange" onclick="removeSchedule(this,'%s')">Remove</button>`+
				`</div>`,
			enableClass, sid, !sched.Enabled, enableLabel,
			sid,
		)
		schedRows = append(schedRows, fmt.Sprintf(
			`<tr><td title="%s">%s</td>`+
				`<td><select id="sched-iv-%s" style="width:auto;padding:3px;font-size:12px" onchange="autoSaveSchedule('%s')">%s</select></td>`+
				`<td><select id="sched-da-%s" style="width:auto;padding:3px;font-size:12px" onchange="autoSaveSchedule('%s')">%s</select></td>`+
				`<td><input type="checkbox" id="sched-ad-%s"%s style="width:auto;margin:0" onchange="autoSaveSchedule('%s')"></td>`+
				`<td>%s</td><td class="src-meta">%s</td><td class="src-meta">%s</td><td>%s</td></tr>`,
			html.EscapeString(srcTitle), html.EscapeString(srcTitle),
			sid, sid, buildIntervalOptions(sched.IntervalHours),
			sid, sid, buildDateAfterOptions(sched.DateAfter),
			sid, autoDLChecked, sid,
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
		autoRefreshJS = `(function quietPoll(){
  function poll(){
    const tab=location.hash.slice(1);
    if(tab==='settings'||tab==='cookies'||tab==='new')return setTimeout(poll,5000);
    fetch('/api/v1/tasks').then(r=>r.json()).then(d=>{
      let stillRunning=false;
      (d.items||[]).forEach(t=>{if(t.status==='running'||t.status==='queued')stillRunning=true;});
      if(!stillRunning){location.reload();return;}
      setTimeout(poll,5000);
    }).catch(()=>setTimeout(poll,5000));
  }
  setTimeout(poll,5000);
})();`
	}

	// ── Page template ─────────────────────────────────────────────────────────
	page := `<!doctype html><html lang="en" data-theme="dark">
<head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>YtVault</title>
<link rel="preconnect" href="https://fonts.googleapis.com"><link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Roboto:wght@300;400;500;700&family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
<style>
*{box-sizing:border-box;margin:0;padding:0}
*{-webkit-font-smoothing:antialiased;-moz-osx-font-smoothing:grayscale}
html{-webkit-text-size-adjust:100%%;text-size-adjust:100%%}
input,select,textarea,button{font-family:inherit}
/* ── YouTube 2026 "Nocturn" (default) ── */
:root,[data-theme="youtube-dark"]{
  --bg:#0f0f0f;--surface:#212121;--surface-hover:#2a2a2a;--surface-glass:#212121;
  --border:#303030;--border-subtle:rgba(255,255,255,0.06);--border-hover:rgba(255,0,51,0.3);
  --text:#f1f1f1;--text-secondary:#aaaaaa;--text-muted:#717171;--text-faint:rgba(113,113,113,0.5);
  --primary:#ff0033;--primary-glow:0 0 8px rgba(255,0,51,0.35),0 0 20px rgba(255,0,51,0.12);--primary-bg:rgba(255,0,51,0.1);
  --primary-gradient:linear-gradient(135deg,#ff0033,#ff00d4);
  --success:#2ba640;--success-bg:rgba(43,166,64,0.12);--success-text:#4ade80;
  --danger:#ff4e45;--danger-bg:rgba(255,78,69,0.12);--danger-text:#ff7b74;
  --warning:#f59e0b;--warning-bg:rgba(245,158,11,0.1);--warning-text:#fcd34d;
  --orange:#fb923c;--orange-bg:rgba(249,115,22,0.1);
  --sidebar-bg:#0f0f0f;--sidebar-active:rgba(255,255,255,0.08);
  --card-shadow:0 4px 16px rgba(0,0,0,0.5);--input-bg:#181818;
  --badge-muted-bg:rgba(255,255,255,0.08);
  --tag-ch-bg:rgba(170,130,255,0.12);--tag-ch-border:rgba(170,130,255,0.15);--tag-ch-text:#c4b5fd;
  --font:'Roboto',system-ui,sans-serif;--btn-radius:999px;--card-radius:12px;--input-radius:999px;--select-radius:8px;
}
[data-theme="youtube-light"]{
  --bg:#ffffff;--surface:#f2f2f2;--surface-hover:#e8e8e8;--surface-glass:#f2f2f2;
  --border:#d9d9d9;--border-subtle:rgba(0,0,0,0.06);--border-hover:rgba(255,0,0,0.3);
  --text:#0f0f0f;--text-secondary:#606060;--text-muted:#909090;--text-faint:rgba(96,96,96,0.4);
  --primary:#ff0000;--primary-glow:0 0 6px rgba(255,0,0,0.2),0 0 16px rgba(255,0,0,0.06);--primary-bg:rgba(255,0,0,0.06);
  --primary-gradient:linear-gradient(135deg,#ff0000,#cc0000);
  --success:#1b8c33;--success-bg:rgba(27,140,51,0.08);--success-text:#1b8c33;
  --danger:#cc0000;--danger-bg:rgba(204,0,0,0.06);--danger-text:#cc0000;
  --warning:#b45309;--warning-bg:rgba(180,83,9,0.06);--warning-text:#b45309;
  --orange:#c2410c;--orange-bg:rgba(194,65,12,0.06);
  --sidebar-bg:#ffffff;--sidebar-active:rgba(0,0,0,0.05);
  --card-shadow:0 2px 8px rgba(0,0,0,0.06);--input-bg:#ffffff;
  --badge-muted-bg:rgba(0,0,0,0.05);
  --tag-ch-bg:rgba(120,80,220,0.08);--tag-ch-border:rgba(120,80,220,0.1);--tag-ch-text:#6d28d9;
  --font:'Roboto',system-ui,sans-serif;--btn-radius:999px;--card-radius:12px;--input-radius:999px;--select-radius:8px;
}
/* ── Classic "Obsidian Glass" ── */
[data-theme="classic-dark"]{
  --bg:#020617;--surface:#0f172a;--surface-hover:#1e293b;--surface-glass:rgba(15,23,42,0.7);
  --border:#1e293b;--border-subtle:rgba(255,255,255,0.04);--border-hover:rgba(59,130,246,0.25);
  --text:#f8fafc;--text-secondary:#94a3b8;--text-muted:#475569;--text-faint:rgba(71,85,105,0.4);
  --primary:#3b82f6;--primary-glow:0 0 8px rgba(59,130,246,0.4),0 0 24px rgba(59,130,246,0.15);--primary-bg:rgba(59,130,246,0.08);
  --primary-gradient:linear-gradient(135deg,#3b82f6,#22c55e);
  --success:#10b981;--success-bg:rgba(16,185,129,0.12);--success-text:#6ee7b7;
  --danger:#ef4444;--danger-bg:rgba(239,68,68,0.12);--danger-text:#fca5a5;
  --warning:#f59e0b;--warning-bg:rgba(245,158,11,0.12);--warning-text:#fcd34d;
  --orange:#fb923c;--orange-bg:rgba(249,115,22,0.12);
  --sidebar-bg:rgba(2,6,23,0.95);--sidebar-active:rgba(59,130,246,0.08);
  --card-shadow:0 8px 32px rgba(0,0,0,0.4);--input-bg:rgba(15,23,42,0.8);
  --badge-muted-bg:rgba(30,41,59,0.6);
  --tag-ch-bg:rgba(167,139,250,0.1);--tag-ch-border:rgba(167,139,250,0.15);--tag-ch-text:#a78bfa;
  --font:'Inter',system-ui,sans-serif;--btn-radius:10px;--card-radius:16px;--input-radius:10px;--select-radius:10px;
}
[data-theme="classic-light"]{
  --bg:#f8fafc;--surface:#ffffff;--surface-hover:#f1f5f9;--surface-glass:rgba(255,255,255,0.85);
  --border:#e2e8f0;--border-subtle:rgba(0,0,0,0.04);--border-hover:rgba(59,130,246,0.3);
  --text:#0f172a;--text-secondary:#64748b;--text-muted:#94a3b8;--text-faint:rgba(100,116,139,0.4);
  --primary:#2563eb;--primary-glow:0 0 6px rgba(37,99,235,0.2),0 0 16px rgba(37,99,235,0.08);--primary-bg:rgba(37,99,235,0.06);
  --primary-gradient:linear-gradient(135deg,#2563eb,#059669);
  --success:#059669;--success-bg:rgba(5,150,105,0.08);--success-text:#059669;
  --danger:#dc2626;--danger-bg:rgba(220,38,38,0.06);--danger-text:#dc2626;
  --warning:#d97706;--warning-bg:rgba(217,119,6,0.06);--warning-text:#d97706;
  --orange:#ea580c;--orange-bg:rgba(234,88,12,0.06);
  --sidebar-bg:rgba(255,255,255,0.95);--sidebar-active:rgba(37,99,235,0.05);
  --card-shadow:0 4px 6px -1px rgba(0,0,0,0.05);--input-bg:#f8fafc;
  --badge-muted-bg:rgba(241,245,249,0.8);
  --tag-ch-bg:rgba(139,92,246,0.06);--tag-ch-border:rgba(139,92,246,0.1);--tag-ch-text:#6d28d9;
  --font:'Inter',system-ui,sans-serif;--btn-radius:10px;--card-radius:16px;--input-radius:10px;--select-radius:10px;
}
body{font-family:var(--font);background:var(--bg);color:var(--text);min-height:100vh;transition:background 0.3s,color 0.3s,font-family 0.3s;letter-spacing:-0.01em}
body::before{display:none}
[data-theme="classic-dark"] body::before,[data-theme="classic-light"] body::before{display:block;content:'';position:fixed;inset:0;background:radial-gradient(ellipse at 20%% 0%%,rgba(59,130,246,0.06) 0%%,transparent 50%%),radial-gradient(ellipse at 80%% 100%%,rgba(124,58,237,0.04) 0%%,transparent 50%%);pointer-events:none;z-index:0}
[data-theme="classic-light"] body::before{display:none}
[data-theme="classic-dark"] .card,[data-theme="classic-light"] .card{backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px);background:var(--surface-glass)}
[data-theme="classic-light"] .card{border:none;box-shadow:var(--card-shadow)}
[data-theme="classic-light"] .thumb-wrap{background:#f1f5f9}
[data-theme="youtube-light"] .thumb-wrap{background:#e8e8e8}
/* ── Sidebar ── */
.sidebar{position:fixed;top:0;left:0;width:220px;height:100vh;background:var(--sidebar-bg);border-right:1px solid var(--border-subtle);display:flex;flex-direction:column;z-index:200;padding:0;transition:width 0.2s,background 0.3s}
.sidebar.collapsed{width:56px}
.sidebar.collapsed .nav-label,.sidebar.collapsed .sidebar-label,.sidebar.collapsed .count,.sidebar.collapsed .logo-text,.sidebar.collapsed .theme-toggle,.sidebar.collapsed .status-dot{display:none}
.sidebar.collapsed .sidebar-logo{padding:16px 12px;text-align:center}
.sidebar-logo{padding:20px 16px 16px;font-size:20px;font-weight:700;color:var(--text);letter-spacing:-0.5px;display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid var(--border-subtle);margin-bottom:4px}
.logo-text{white-space:nowrap}
.logo-text b{color:var(--primary);font-weight:700}
.logo-icon{display:inline-flex;align-items:center;justify-content:center;width:28px;height:20px;background:var(--primary);border-radius:6px;color:#fff;font-size:11px;margin-right:6px}
.sidebar-toggle{background:none;border:none;color:var(--text-muted);font-size:14px;cursor:pointer;padding:6px;border-radius:8px;transition:all 0.15s}
.sidebar-toggle:hover{color:var(--text-secondary);background:var(--surface-hover)}
.sidebar-section{padding:0 10px;margin-bottom:4px}
.sidebar-label{font-size:10px;font-weight:600;letter-spacing:1px;text-transform:uppercase;color:var(--text-faint);padding:14px 8px 6px}
.nav-tab{position:relative;display:flex;align-items:center;gap:10px;width:100%%;background:none;border:none;color:var(--text-secondary);font-size:13px;font-weight:500;padding:10px 12px;border-radius:10px;cursor:pointer;transition:all 0.15s;text-align:left}
.nav-tab:hover{color:var(--text);background:var(--surface-hover)}
.nav-tab.active{color:var(--text);background:var(--sidebar-active);font-weight:600}
.nav-tab .icon{font-size:16px;width:20px;text-align:center;flex-shrink:0}
.nav-tab .count{margin-left:auto;font-size:10px;color:var(--text-muted);font-weight:500;background:var(--surface-hover);padding:1px 7px;border-radius:99px}
.status-dot{display:inline-block;width:7px;height:7px;border-radius:50%%;margin-left:auto;flex-shrink:0}
.status-dot.ok{background:#2ba640;box-shadow:0 0 6px rgba(43,166,64,0.5);animation:pulse-gentle 3s infinite}
@keyframes pulse-gentle{0%%,100%%{box-shadow:0 0 6px rgba(43,166,64,0.5)}50%%{box-shadow:0 0 12px rgba(43,166,64,0.7)}}
.status-dot.warn{background:var(--warning);box-shadow:0 0 8px rgba(245,158,11,0.5);animation:pulse 1.5s infinite}
.status-dot.err{background:var(--danger);box-shadow:0 0 8px rgba(239,68,68,0.5)}
.sidebar-footer{margin-top:auto;padding:16px;border-top:1px solid var(--border-subtle)}
/* ── Theme toggle ── */
.theme-toggle{background:none;border:1px solid var(--border-subtle);color:var(--text-secondary);padding:6px 10px;border-radius:8px;font-size:13px;cursor:pointer;transition:all 0.15s;width:100%%}
.theme-toggle:hover{background:var(--surface-hover);color:var(--text)}
/* ── Main area ── */
.main{margin-left:220px;padding:28px 32px;min-height:100vh;position:relative;z-index:1;transition:margin-left 0.2s}
.sidebar.collapsed ~ .main{margin-left:56px}
.page-title{font-size:20px;font-weight:500;color:var(--text);margin-bottom:18px;letter-spacing:-0.02em}
.tab-content{display:none}.tab-content.active{display:block}
/* ── Glass cards ── */
.card{background:var(--surface);border:1px solid var(--border-subtle);border-radius:var(--card-radius);padding:20px;margin-bottom:16px;transition:background 0.3s}
.card-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:16px;flex-wrap:wrap;gap:8px 12px}
.card-title{font-size:15px;font-weight:600;color:var(--text);letter-spacing:-0.2px}
/* ── Forms ── */
.form-row{display:grid;grid-template-columns:140px 1fr;gap:10px;margin-bottom:12px;align-items:end}
.form-row-3{display:grid;grid-template-columns:140px 1fr 160px;gap:10px;margin-bottom:12px;align-items:end}
label{font-size:13px;color:var(--text-secondary);margin-bottom:3px;display:block;font-weight:500}
select,input[type=text],textarea{width:100%%;background:var(--input-bg);border:1px solid var(--border);border-radius:var(--input-radius);padding:8px 16px;color:var(--text);font-size:14px;font-family:inherit;transition:border-color 0.2s,background 0.3s}
textarea{border-radius:12px}
select{border-radius:var(--select-radius);padding:8px 12px}
select:focus,input:focus,textarea:focus{outline:none;border-color:var(--text-muted)}
/* ── Buttons ── */
button{border:none;border-radius:var(--btn-radius);font-weight:500;font-size:13px;cursor:pointer;transition:all 0.15s;font-family:inherit}
.btn-primary{background:var(--primary);color:#fff;padding:9px 22px;letter-spacing:0.02em}
.btn-primary:hover{filter:brightness(1.15);transform:translateY(-1px)}
.btn-sm{padding:5px 14px;font-size:12px;border-radius:var(--btn-radius);transition:all 0.15s}
.btn-sm:hover{filter:brightness(1.1)}
.btn-blue{background:var(--primary-bg);color:var(--primary);border:1px solid rgba(59,130,246,0.2)}
.btn-blue:hover{box-shadow:0 0 12px var(--primary-glow)}
.btn-green{background:var(--success-bg);color:var(--success-text);border:1px solid rgba(16,185,129,0.2)}
.btn-green:hover{box-shadow:0 0 12px rgba(16,185,129,0.15)}
.btn-orange{background:var(--orange-bg);color:var(--orange);border:1px solid rgba(249,115,22,0.2)}
.btn-orange:hover{box-shadow:0 0 12px rgba(249,115,22,0.15)}
.btn-red{background:var(--danger-bg);color:var(--danger-text);border:1px solid rgba(239,68,68,0.2)}
.btn-gray{background:var(--badge-muted-bg);color:var(--text-secondary);border:1px solid var(--border-subtle)}
.btn-gray:hover{background:var(--surface-hover)}
/* ── Tables ── */
.table-wrap{overflow-x:auto}
table{width:100%%;border-collapse:collapse;font-size:13px}
th{color:var(--text-muted);font-weight:500;padding:10px 10px;border-bottom:1px solid var(--border-subtle);text-align:left;white-space:nowrap;font-size:12px}
td{padding:10px 10px;border-bottom:1px solid var(--border-subtle);vertical-align:top;max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;transition:background 0.15s}
tr:last-child td{border-bottom:none}
tr:hover td{background:var(--surface-hover)}
tr:hover .row-actions{opacity:1}
.row-actions{opacity:0.4;transition:opacity 0.2s}
/* ── Badges & tags ── */
.badge{display:inline-flex;align-items:center;gap:5px;border-radius:999px;padding:3px 10px;font-size:11px;font-weight:600;white-space:nowrap}
.badge-queued{background:var(--primary-bg);color:var(--primary)}
.badge-running{background:var(--warning-bg);color:var(--warning)}
.badge-running::before{content:'';width:6px;height:6px;border-radius:50%%;background:var(--primary);animation:pulse 1.5s infinite}
.badge-succeeded{background:var(--success-bg);color:var(--success)}
.badge-failed{background:var(--danger-bg);color:var(--danger-text)}
.badge-pending{background:var(--badge-muted-bg);color:var(--text-secondary)}
.badge-downloading{background:var(--primary-bg);color:var(--primary)}
.badge-downloading::before{content:'';width:6px;height:6px;border-radius:50%%;background:var(--primary);animation:pulse 1.5s infinite}
.badge-done{background:var(--success-bg);color:var(--success)}
.badge-done::before{content:'';width:6px;height:6px;border-radius:50%%;background:var(--success)}
.badge-missing{background:var(--orange-bg);color:var(--orange)}
.badge-enabled{background:var(--success-bg);color:var(--success)}
.badge-enabled::before{content:'';width:6px;height:6px;border-radius:50%%;background:var(--success)}
.badge-disabled{background:var(--badge-muted-bg);color:var(--text-secondary)}
@keyframes pulse{0%%,100%%{opacity:1}50%%{opacity:0.3}}
@keyframes shimmer{0%%{background-position:-200%% 0}100%%{background-position:200%% 0}}
.badge-running,.badge-downloading{position:relative;overflow:hidden}
.badge-running::after,.badge-downloading::after{content:'';position:absolute;inset:0;background:linear-gradient(90deg,transparent,rgba(255,0,51,0.08),transparent);background-size:200%% 100%%;animation:shimmer 2s infinite}
/* ── Empty state ── */
/* ── Skeleton loader ── */
.skeleton{border-radius:8px;background:linear-gradient(90deg,var(--surface) 25%%,var(--surface-hover) 50%%,var(--surface) 75%%);background-size:200%% 100%%;animation:skeleton-shimmer 1.5s infinite}
@keyframes skeleton-shimmer{0%%{background-position:200%% 0}100%%{background-position:-200%% 0}}
.skeleton-row{display:flex;align-items:center;gap:14px;padding:12px 0}
.skeleton-thumb{width:120px;height:68px;border-radius:10px}
.skeleton-text{height:14px;border-radius:4px}
.skeleton-text.w60{width:60%%}
.skeleton-text.w30{width:30%%}
.skeleton-text.w20{width:20%%}
.empty-state{text-align:center;padding:48px 24px}
.empty-state .empty-icon{font-size:40px;margin-bottom:12px;opacity:0.3}
.empty-state .empty-text{color:var(--text-secondary);font-size:14px}
.empty-state .empty-sub{color:var(--text-muted);font-size:12px;margin-top:4px}
.tag{display:inline-flex;align-items:center;background:var(--badge-muted-bg);border:1px solid var(--border-subtle);border-radius:999px;padding:2px 8px;font-size:11px;color:var(--primary);transition:background 0.3s}
.src-title{font-weight:600;color:var(--text);margin-bottom:3px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:14px}
.src-meta{color:var(--text-muted);font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.input-container{max-width:800px}
.tags{display:flex;gap:4px;flex-wrap:wrap;margin:3px 0}
/* ── Source cards ── */
.source-card{background:var(--surface);border:1px solid var(--border-subtle);border-radius:var(--card-radius);padding:18px;margin-bottom:10px;transition:background 0.15s;cursor:default}
.source-card:hover{background:var(--surface-hover)}
.source-card:hover .card-actions{opacity:1;transform:translateY(0)}
.source-card .card-actions{opacity:0;transform:translateY(6px);transition:all 0.25s ease;margin-top:12px;display:flex;gap:6px;flex-wrap:wrap;align-items:center}
.source-card .source-header{display:flex;align-items:center;gap:10px;margin-bottom:6px}
.source-card .source-header .src-title{font-size:15px;flex:1}
.source-url{color:var(--text-muted);font-size:12px;text-decoration:none;transition:color 0.15s}
.source-url:hover{color:var(--primary)}
[data-theme="dark"] .source-url{color:#64748b}
.progress-bar{height:4px;border-radius:2px;background:var(--border-subtle);overflow:hidden;margin:8px 0}
.progress-bar .fill{height:100%%;border-radius:2px;background:var(--primary-gradient);transition:width 0.4s ease}
.stat-num{font-family:'JetBrains Mono',ui-monospace,monospace;font-size:12px;color:var(--text-secondary);letter-spacing:-0.3px}
/* Kind badges with tinted backgrounds */
.tag-video{background:var(--badge-muted-bg);border-color:var(--border-subtle);color:var(--text-secondary)}
.tag-channel{background:var(--tag-ch-bg);border-color:var(--tag-ch-border);color:var(--tag-ch-text)}
.tag-playlist{background:var(--primary-bg);border-color:rgba(59,130,246,0.12);color:var(--primary)}
[data-theme="light"] .thumb-wrap{background:#e8e8e8}
/* More actions menu */
.more-btn{background:none;border:1px solid rgba(255,255,255,0.06);color:#475569;padding:4px 8px;font-size:14px;border-radius:8px;cursor:pointer;transition:all 0.15s;line-height:1}
.more-btn:hover{color:#94a3b8;background:rgba(255,255,255,0.04);border-color:rgba(255,255,255,0.1)}
.actions-popover{display:none;position:absolute;right:0;top:100%%;background:rgba(15,23,42,0.95);backdrop-filter:blur(16px);border:1px solid rgba(255,255,255,0.08);border-radius:12px;padding:8px;min-width:180px;box-shadow:0 12px 40px rgba(0,0,0,0.5);z-index:50}
.actions-popover.open{display:block}
.actions-popover button{width:100%%;text-align:left;padding:7px 10px;border-radius:8px;background:none;color:#94a3b8;font-size:12px;font-weight:500;border:none}
.actions-popover button:hover{background:rgba(255,255,255,0.05);color:#e2e8f0}
/* ── Media thumbnails ── */
.thumb-wrap{position:relative;width:168px;min-width:168px;aspect-ratio:16/9;border-radius:8px;overflow:hidden;background:var(--surface)}
.thumb-wrap img{width:100%%;height:100%%;object-fit:cover;display:block}
.thumb-status{position:absolute;bottom:4px;right:4px;width:18px;height:18px;border-radius:50%%;display:flex;align-items:center;justify-content:center;font-size:10px;backdrop-filter:blur(4px);border:1px solid rgba(255,255,255,0.1)}
.thumb-status.st-done{background:rgba(20,83,45,0.8);color:#86efac}
.thumb-status.st-pending{background:rgba(30,41,59,0.8);color:#94a3b8}
.thumb-status.st-failed{background:rgba(69,10,10,0.8);color:#fca5a5}
.thumb-status.st-downloading{background:rgba(59,29,0,0.8);color:#fb923c}
.thumb-status.st-missing{background:rgba(67,20,7,0.8);color:#fb923c}
.media-info{display:flex;flex-direction:column;gap:3px;min-width:0;flex:1}
.media-info .m-title{font-weight:500;color:var(--text);font-size:14px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;line-height:1.4;letter-spacing:-0.02em}
.media-info .m-sub{font-size:12px;color:var(--text-muted);letter-spacing:0}
.media-row{display:flex;align-items:center;gap:14px;padding:10px 0;border-bottom:1px solid var(--border-subtle);transition:background 0.15s}
.media-row:last-child{border-bottom:none}
.media-row:hover{background:var(--surface-hover)}
.media-row:hover .row-actions{opacity:1}
.media-row .row-actions{opacity:0.3;transition:opacity 0.2s;display:flex;gap:6px;align-items:center;flex-shrink:0}
.filter-bar{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.filter-bar select{width:auto;min-width:0;flex-shrink:1}
/* ── Toast ── */
#toast{position:fixed;bottom:24px;right:24px;z-index:999;display:flex;flex-direction:column;gap:8px;pointer-events:none}
.toast-msg{background:var(--surface);border:1px solid var(--border-subtle);border-radius:var(--card-radius);padding:10px 20px;font-size:13px;max-width:400px;pointer-events:auto;animation:tslide .25s ease;box-shadow:0 4px 24px rgba(0,0,0,0.3);color:var(--text)}
.toast-ok{color:var(--success-text)}
.toast-err{color:var(--danger-text)}
.toast-info{color:var(--primary)}
@keyframes tslide{from{opacity:0;transform:translateY(10px)}to{opacity:1;transform:translateY(0)}}
.empty{color:var(--text-secondary);text-align:center;padding:32px;font-size:13px}
/* ── Pager ── */
.pager{display:flex;align-items:center;gap:8px;padding:10px 0;font-size:12px;color:var(--text-muted)}
.pager select{background:var(--input-bg);border:1px solid var(--border);border-radius:8px;padding:4px 6px;color:var(--text);font-size:11px;width:auto;min-width:0}
.pager button{background:var(--badge-muted-bg);border:1px solid var(--border);border-radius:8px;padding:4px 12px;color:var(--text-secondary);font-size:12px;cursor:pointer}
.pager button:hover:not(:disabled){color:var(--text)}
.pager button:disabled{opacity:0.3;cursor:default}
/* ── Responsive ── */
/* ── Tablet (<=1024px) ── */
@media(max-width:1024px){
  .main{padding:20px 16px}
  .input-container{max-width:100%%}
  .card{padding:16px}
  .source-card{padding:16px}
  td{max-width:200px}
}
/* ── Phone (<=768px) ── */
@media(max-width:768px){
  .sidebar{width:56px}
  .sidebar .nav-label,.sidebar-label,.logo-text,.nav-tab .count,.sidebar-toggle,.status-dot,.theme-toggle{display:none}
  .sidebar-logo{padding:14px 10px;text-align:center}
  .main{margin-left:56px;padding:12px}
  .page-title{font-size:18px;margin-bottom:14px}
  .card{padding:14px;border-radius:12px;margin-bottom:14px}
  .card-header{flex-direction:column;align-items:flex-start;gap:8px}
  .filter-bar{width:100%%;flex-wrap:wrap}
  .filter-bar select{flex:1;min-width:80px}
  .filter-bar .src-meta{font-size:11px}
  .form-row,.form-row-3{grid-template-columns:1fr}
  .source-card{padding:14px}
  .source-card .source-header{flex-wrap:wrap}
  .source-card .card-actions{opacity:1;transform:none}
  .media-row{flex-wrap:wrap;gap:10px}
  .thumb-wrap{width:100%%;min-width:0;max-width:200px}
  .media-row .row-actions{opacity:1;width:100%%}
  .table-wrap{font-size:12px}
  td{max-width:150px;padding:8px 6px}
  th{padding:8px 6px;font-size:10px}
  .pager{flex-wrap:wrap;justify-content:center}
  .btn-primary{padding:8px 14px;font-size:12px}
  .btn-sm{font-size:11px;padding:3px 8px}
}
/* ── Small phone (<=480px) ── */
@media(max-width:480px){
  .sidebar{width:0;border:none}
  .sidebar.show-mobile{width:220px;position:fixed;z-index:300}
  .main{margin-left:0;padding:10px}
  .page-title{font-size:16px}
  .card{padding:12px;border-radius:10px}
  .thumb-wrap{max-width:160px}
  .media-info .m-title{font-size:13px}
}
</style>
</head>
<body>
<aside class="sidebar" id="sidebar">
  <div class="sidebar-logo"><span class="logo-text"><span class="logo-icon">&#9654;</span>Yt<b>Vault</b></span><button class="sidebar-toggle" onclick="toggleSidebar()" title="Toggle sidebar">&#9776;</button></div>
  <div class="sidebar-section">
    <div class="sidebar-label">Main</div>
    <button class="nav-tab" id="tab-btn-sources" onclick="showTab('sources')"><span class="icon">&#9881;</span> <span class="nav-label">Sources</span> <span class="count">%d</span></button>
    <button class="nav-tab" id="tab-btn-media" onclick="showTab('media')"><span class="icon">&#9654;</span> <span class="nav-label">Media</span> <span class="count">%d</span></button>
    <button class="nav-tab" id="tab-btn-tasks" onclick="showTab('tasks')"><span class="icon">&#9889;</span> <span class="nav-label">Tasks</span> <span class="count">%d</span></button>
    <button class="nav-tab" id="tab-btn-schedules" onclick="showTab('schedules')"><span class="icon">&#128337;</span> <span class="nav-label">Schedules</span> <span class="count">%d</span></button>
  </div>
  <div class="sidebar-section">
    <div class="sidebar-label">System</div>
    <button class="nav-tab" id="tab-btn-cookies" onclick="showTab('cookies')"><span class="icon">&#128274;</span> <span class="nav-label">Cookies</span> %s</button>
    <button class="nav-tab" id="tab-btn-settings" onclick="showTab('settings')"><span class="icon">&#9881;</span> <span class="nav-label">Settings</span></button>
  </div>
  <div class="sidebar-footer">
    <button class="theme-toggle" onclick="toggleDarkLight()" id="theme-btn">&#9790; Dark</button>
  </div>
</aside>
<div id="toast"></div>
<div class="main">

<span id="tab-btn-new" style="display:none"></span>
<div id="tab-new" class="tab-content">
  <div style="display:flex;align-items:center;gap:12px;margin-bottom:18px">
    <button class="btn-sm btn-gray" onclick="showTab('sources')" title="Back to Sources">&#8592; Back</button>
    <h1 class="page-title" style="margin-bottom:0">Add New Source</h1>
  </div>
  <div class="card">
    <div class="card-header"><span class="card-title">Search YouTube</span></div>
    <div class="input-container" style="display:flex;gap:8px;margin-bottom:12px">
      <input type="text" id="search-q" placeholder="Search YouTube by keywords…" style="flex:1;backdrop-filter:blur(10px)" onkeydown="if(event.key==='Enter')searchYT()">
      <button class="btn-primary" onclick="withLoading(this,searchYT)">Search</button>
    </div>
    <div id="search-results" style="display:none">
      <div id="search-tbody"></div>
    </div>
  </div>
  <div class="card">
    <div class="card-header"><span class="card-title">Add Source</span></div>
    <input type="hidden" id="kind" value="video">
    <div class="form-row input-container">
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
      <label style="display:flex;align-items:center;gap:4px;font-size:12px;color:var(--text-secondary);cursor:pointer">
        <input type="checkbox" id="auto-download" style="width:auto;margin:0" checked> Auto-download after scan
      </label>
    </div>
    <div id="new-sched-wrap" style="display:none;margin-bottom:12px">
      <div style="font-size:13px;font-weight:600;color:var(--text);margin-bottom:8px">Options</div>
      <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap;margin-bottom:8px">
        <label style="display:flex;align-items:center;gap:4px;font-size:12px;color:var(--text-secondary);cursor:pointer">
          <input type="checkbox" id="new-sched-enabled" style="width:auto;margin:0"> Enable recurring scan
        </label>
        <select id="new-sched-interval" style="width:auto;padding:4px 8px;font-size:12px" title="How often to scan for new videos">
          <option value="6">Every 6h</option><option value="12">Every 12h</option>
          <option value="24" selected>Every 24h</option><option value="168">Every week</option>
        </select>
      </div>
      <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
        <span class="src-meta" style="font-size:12px">Retention:</span>
        <select id="new-retention" style="width:auto;padding:4px 8px;font-size:12px" title="Auto-delete downloaded media older than this">
          <option value="0">Default</option><option value="30">30 days</option><option value="60">60 days</option>
          <option value="90">90 days</option><option value="180">180 days</option><option value="365">1 year</option>
          <option value="-1">Keep forever</option>
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
  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:18px">
    <h1 class="page-title" style="margin-bottom:0">Sources <span style="font-size:14px;color:var(--text-muted);font-weight:400">(%d)</span></h1>
    <button class="btn-primary" onclick="showTab('new')">+ Create</button>
  </div>
  <div id="sources-list">%s</div>
  <div class="pager" id="pager-sources"></div>
</div>

<div id="tab-media" class="tab-content">
  <h1 class="page-title">Media Library</h1>
  <div class="card">
    <div class="card-header">
      <span class="card-title">Media Items (%d)</span>
      <div class="filter-bar">
        <button class="btn-sm btn-green" title="Download all filtered media (pending, missing, and failed)" onclick="withLoading(this,downloadAll)">Download All</button>
        <span class="src-meta">Source:</span>
        <select id="media-filter" onchange="filterMedia(this.value)">
          <option value="">All sources</option>%s
        </select>
        <span class="src-meta">Status:</span>
        <select id="media-status-filter" onchange="filterMediaStatus(this.value)">
          <option value="">All</option><option value="pending">Pending</option><option value="downloading">Downloading</option>
          <option value="done">Done</option><option value="failed">Failed</option><option value="missing">Missing</option>
        </select>
      </div>
    </div>
    <div id="media-tbody">%s</div>
    <div class="pager" id="pager-media"></div>
  </div>
</div>

<div id="tab-tasks" class="tab-content">
  <h1 class="page-title">Tasks</h1>
  <div class="card">
    <div class="card-header"><span class="card-title">Tasks (%d)</span><div style="display:flex;gap:6px;flex-wrap:wrap"><select id="task-status-filter" onchange="filterTaskStatus(this.value)" style="font-size:12px;padding:3px 6px;width:auto"><option value="">All</option><option value="queued">Queued</option><option value="running">Running</option><option value="succeeded">Succeeded</option><option value="failed">Failed</option></select> <button class="btn-sm btn-orange" title="Cancel all running tasks" onclick="withLoading(this,stopRunningTasks)">Stop running</button><button class="btn-sm btn-blue" title="Retry all failed download tasks" onclick="withLoading(this,retryAllFailed)">Retry failed</button><button class="btn-sm btn-gray" title="Remove all succeeded and failed tasks" onclick="withLoading(this,clearCompletedTasks)">Clear completed</button></div></div>
    <div class="table-wrap"><table>
      <thead><tr><th>Type</th><th>Entity</th><th>Status</th><th>Created</th><th></th></tr></thead>
      <tbody id="tasks-tbody">%s</tbody>
    </table></div>
    <div class="pager" id="pager-tasks"></div>
  </div>
</div>

<div id="tab-schedules" class="tab-content">
  <h1 class="page-title">Schedules</h1>
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
  <h1 class="page-title">Cookie Management</h1>
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
      <label style="display:inline-block;padding:6px 14px;background:var(--badge-muted-bg);border:1px solid var(--border);border-radius:8px;cursor:pointer;font-size:13px;color:var(--primary)">
        Choose File <input type="file" id="ck-file" accept=".txt" style="display:none" onchange="document.getElementById('ck-filename').textContent=this.files[0]?.name||''">
      </label>
      <span id="ck-filename" class="src-meta" style="margin-left:8px"></span>
    </div>
    <div style="margin-bottom:10px">
      <label>Or paste content:</label>
      <textarea id="ck-paste" rows="6" style="width:100%%;background:var(--bg-input);border:1px solid var(--border);border-radius:8px;padding:8px;color:var(--text);font-family:monospace;font-size:12px;resize:vertical" placeholder="# Netscape HTTP Cookie File&#10;.youtube.com	TRUE	/	TRUE	..."></textarea>
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
          <li>Install yt-dlp locally: <code style="background:var(--badge-muted-bg);padding:2px 6px;border-radius:4px">brew install yt-dlp</code></li>
          <li>Run: <code style="background:var(--badge-muted-bg);padding:2px 6px;border-radius:4px">yt-dlp --cookies-from-browser safari --cookies cookies.txt --skip-download https://www.youtube.com</code></li>
          <li>Upload the generated <code>cookies.txt</code> file above</li>
        </ol>
      </details>
      <p class="src-meta" style="margin-top:8px">For more details see the <a href="https://github.com/yt-dlp/yt-dlp/wiki/FAQ#how-do-i-pass-cookies-to-yt-dlp" target="_blank" style="color:#60a5fa">yt-dlp cookie guide</a>.</p>
      <p class="src-meta" style="margin-top:4px">Cookies expire over time (usually weeks to months). If downloads start failing, re-export a fresh cookies file.</p>
    </div>
  </div>
</div>

<div id="tab-settings" class="tab-content">
  <h1 class="page-title">Settings</h1>
  <div class="card">
    <div class="card-header"><span class="card-title">Theme</span></div>
    <div class="form-row">
      <label>Design theme</label>
      <select id="cfg-design-theme" onchange="setDesignTheme(this.value)">
        <option value="youtube">YouTube 2026</option>
        <option value="classic">Classic (Obsidian Glass)</option>
      </select>
    </div>
    <p class="src-meta" style="margin-top:4px">Use the moon/sun button in the sidebar to toggle dark/light mode.</p>
  </div>
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
    <div style="margin:10px 0;padding:12px;background:var(--bg-surface);border:1px solid var(--border);border-radius:10px">
      <div style="font-size:13px;font-weight:600;color:var(--text);margin-bottom:8px">Playlist overrides</div>
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:var(--text-secondary);cursor:pointer;margin-bottom:6px">
        <input type="checkbox" id="cfg-pl-folder" style="width:auto;margin:0" checked> Use playlist name as folder for playlist sources
      </label>
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:var(--text-secondary);cursor:pointer">
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
    <div class="form-row">
      <label>Default retention</label>
      <select id="cfg-retention">
        <option value="0">Keep forever</option>
        <option value="30">30 days</option>
        <option value="60">60 days</option>
        <option value="90">90 days</option>
        <option value="180">180 days</option>
        <option value="365">1 year</option>
      </select>
    </div>
    <p class="src-meta" style="margin:0 0 10px 0">Retention deletes downloaded media older than N days (by upload date). Pinned items are always preserved. Each source can override this default.</p>
    <div style="margin-top:12px">
      <button class="btn-primary" onclick="withLoading(this,saveConfig)">Save Settings</button>
      <span id="cfg-status" class="src-meta" style="margin-left:8px"></span>
    </div>
  </div>
</div>

</div>
<script>
function getThemeKey(){
  const design=localStorage.getItem('designTheme')||'youtube';
  const mode=localStorage.getItem('darkMode')==='light'?'light':'dark';
  return design+'-'+mode;
}
function applyTheme(){
  const key=getThemeKey();
  document.documentElement.dataset.theme=key;
  const isLight=key.endsWith('-light');
  const btn=document.getElementById('theme-btn');
  if(btn)btn.innerHTML=isLight?'&#9790; Dark':'&#9788; Light';
}
function toggleDarkLight(){
  const cur=localStorage.getItem('darkMode')==='light'?'light':'dark';
  localStorage.setItem('darkMode',cur==='dark'?'light':'dark');
  applyTheme();
}
function setDesignTheme(design){
  localStorage.setItem('designTheme',design);
  applyTheme();
}
function toggleSidebar(){
  const s=document.getElementById('sidebar');
  s.classList.toggle('collapsed');
  sessionStorage.setItem('sidebarCollapsed',s.classList.contains('collapsed')?'1':'');
}
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
  const tbody=document.getElementById('search-tbody');
  const wrap=document.getElementById('search-results');
  // Show skeleton loaders
  wrap.style.display='';
  tbody.innerHTML='';
  for(let i=0;i<5;i++){
    tbody.innerHTML+='<div class="skeleton-row"><div class="skeleton skeleton-thumb"></div><div style="flex:1;display:flex;flex-direction:column;gap:8px"><div class="skeleton skeleton-text w60"></div><div class="skeleton skeleton-text w30"></div></div><div class="skeleton skeleton-text w20" style="width:60px;height:28px"></div></div>';
  }
  const res=await fetch('/api/v1/youtube/search?q='+encodeURIComponent(q));
  const data=await res.json();
  tbody.innerHTML='';
  if(!res.ok){toast('Search failed: '+(data.error?.message||''),'err');wrap.style.display='none';return;}
  if(!data.results||data.results.length===0){
    tbody.innerHTML='<div class="empty-state"><div class="empty-text">No results found</div></div>';
    return;
  }
  data.results.forEach(r=>{
    const dur=r.duration>0?(Math.floor(r.duration/60)+'m'+(r.duration%%60)+'s'):'—';
    const d=document.createElement('div');
    d.className='media-row';
    d.innerHTML='<div class="thumb-wrap"><img src="https://img.youtube.com/vi/'+r.id+'/mqdefault.jpg" loading="lazy" onerror="this.style.display=\'none\'"></div>'+
      '<div class="media-info"><span class="m-title" title="'+r.title.replace(/"/g,'&amp;quot;')+'">'+r.title+'</span><span class="m-sub">'+(r.channel||'—')+' &middot; '+dur+'</span></div>'+
      '<button class="btn-sm btn-blue" onclick="selectResult(\''+r.url.replace(/'/g,"\\'")+'\')">Select</button>';
    tbody.appendChild(d);
  });
}
function copyPath(el){
  const fp=el.dataset.filepath;
  if(!fp)return;
  navigator.clipboard.writeText(fp).then(()=>toast('Path copied','ok')).catch(()=>{
    const ta=document.createElement('textarea');ta.value=fp;document.body.appendChild(ta);ta.select();document.execCommand('copy');ta.remove();
    toast('Path copied','ok');
  });
}
function copyText(text){
  navigator.clipboard.writeText(text).then(()=>toast('Copied','ok')).catch(()=>{
    const ta=document.createElement('textarea');ta.value=text;document.body.appendChild(ta);ta.select();document.execCommand('copy');ta.remove();
    toast('Copied','ok');
  });
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
      // Set retention and create schedule if enabled (channel/playlist only)
      if(kind!=='video'&&data.source?.id){
        const retEl=document.getElementById('new-retention');
        const retDays=retEl?parseInt(retEl.value):0;
        if(retDays!==0){
          await fetch('/api/v1/sources/'+data.source.id+'/retention',{
            method:'PUT',headers:{'Content-Type':'application/json'},
            body:JSON.stringify({retentionDays:retDays})
          }).catch(()=>{});
        }
        const schedEnabled=document.getElementById('new-sched-enabled');
        if(schedEnabled&&schedEnabled.checked){
          const interval=parseInt(document.getElementById('new-sched-interval').value)||24;
          await fetch('/api/v1/sources/'+data.source.id+'/schedule',{
            method:'POST',headers:{'Content-Type':'application/json'},
            body:JSON.stringify({intervalHours:interval,enabled:true,autoDownload:true,dateAfter:dateAfter})
          }).catch(()=>{});
        }
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
async function autoSaveSchedule(sourceId){
  const iv=document.getElementById('sched-iv-'+sourceId);
  const da=document.getElementById('sched-da-'+sourceId);
  const ad=document.getElementById('sched-ad-'+sourceId);
  const body={intervalHours:parseInt(iv?.value||24),enabled:true,autoDownload:ad?.checked||false,dateAfter:da?.value||''};
  const res=await fetch('/api/v1/sources/'+sourceId+'/schedule',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  if(res.ok){toast('Schedule updated','ok');}
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
  applyMediaFilters();
}
function filterMediaStatus(status){
  sessionStorage.setItem('mediaStatusFilter',status);
  applyMediaFilters();
}
function applyMediaFilters(){
  const src=sessionStorage.getItem('mediaFilter')||'';
  const st=sessionStorage.getItem('mediaStatusFilter')||'';
  if(window.pagers&&window.pagers['media-tbody']){
    window.pagers['media-tbody'].setFilter(el=>(src===''||el.dataset.source===src)&&(st===''||el.dataset.status===st));
  }else{
    document.querySelectorAll('#media-tbody [data-source]').forEach(el=>{
      el.style.display=((src===''||el.dataset.source===src)&&(st===''||el.dataset.status===st))?'':'none';
    });
  }
}
function filterTaskStatus(status){
  sessionStorage.setItem('taskStatusFilter',status);
  if(window.pagers&&window.pagers['tasks-tbody']){
    window.pagers['tasks-tbody'].setFilter(tr=>status===''||tr.dataset.status===status);
  }else{
    document.querySelectorAll('#tasks-tbody tr[data-status]').forEach(tr=>{
      tr.style.display=(status===''||tr.dataset.status===status)?'':'none';
    });
  }
}
function viewMedia(sourceId){
  showTab('media');
  const f=document.getElementById('media-filter');
  if(f){f.value=sourceId;filterMedia(sourceId);}
}
async function cleanupFiltered(){
  const filter=sessionStorage.getItem('mediaFilter')||'';
  if(!filter){toast('Select a source filter first','info');return;}
  await cleanupSource(filter);
}
async function togglePin(mediaId,el){
  const res=await fetch('/api/v1/media/'+mediaId+'/pin',{method:'POST'});
  if(res.ok){
    const data=await res.json();
    el.textContent=data.pinned?'★':'☆';
    el.style.color=data.pinned?'#fbbf24':'#4b5563';
    el.title=data.pinned?'Unpin':'Pin (protect from cleanup)';
  }
}
async function cleanupSource(sourceId){
  const prev=await fetch('/api/v1/sources/'+sourceId+'/cleanup-preview').then(r=>r.json());
  if(prev.eligible===0){toast('Nothing to clean up'+(prev.retentionDays===0?' (no retention policy)':''),'info');return;}
  if(!confirm('Remove '+prev.eligible+' item(s) older than '+prev.retentionDays+' days? ('+prev.pinned+' pinned preserved)'))return;
  const res=await fetch('/api/v1/sources/'+sourceId+'/cleanup',{method:'POST'});
  const data=await res.json();
  toast('Removed '+(data.removed||0)+' item(s)','ok');
  setTimeout(()=>location.reload(),400);
}
async function downloadMedia(mediaId){
  const res=await fetch('/api/v1/media/'+mediaId+'/download',{method:'POST'});
  if(res.ok||res.status===202){toast('Download started','ok');setTimeout(()=>location.reload(),400);}
  else{const d=await res.json();toast('Failed: '+(d.error?.message||''),'err');}
}
async function downloadAll(){
  const filter=sessionStorage.getItem('mediaFilter')||'';
  const qs=filter?'?source='+encodeURIComponent(filter):'';
  const res=await fetch('/api/v1/media/download-all-pending'+qs,{method:'POST'});
  const data=await res.json();
  if(res.ok||res.status===202){
    toast('Started '+(data.started||0)+' of '+(data.total||0)+' download(s)','ok');
    if(data.total>0)setTimeout(()=>location.reload(),1000);
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
    if(res.ok||res.status===204){const row=btn.closest('.media-row')||btn.closest('tr');if(row)row.remove();toast('Removed','ok');}
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
    if(res.ok||res.status===204){const row=btn.closest('.media-row')||btn.closest('tr');if(row)row.remove();toast('Removed','ok');}
    else{btn.disabled=false;btn.textContent='Remove';btn.className='btn-sm btn-orange';btn.dataset.confirm='0';toast('Failed','err');}
  }).catch(()=>{btn.disabled=false;});
}
async function setRetention(sourceId,value){
  const days=parseInt(value);
  const res=await fetch('/api/v1/sources/'+sourceId+'/retention',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({retentionDays:days})});
  if(res.ok){
    const label=days===0?'default':days===-1?'keep forever':days+' days';
    toast('Retention set: '+label,'ok');
  }else{const d=await res.json();toast('Failed: '+(d.error?.message||''),'err');}
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
  const tag=tbody.tagName==='TBODY'?'tr':':scope > *';
  function getRows(){return Array.from(tbody.querySelectorAll(tag)).filter(r=>r.style.display!=='none'||r.dataset.source!==undefined);}
  function allRows(){return Array.from(tbody.querySelectorAll(tag));}
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
    // Restore design theme selector
    const dt=document.getElementById('cfg-design-theme');
    if(dt)dt.value=localStorage.getItem('designTheme')||'youtube';
    const res=await fetch('/api/v1/config');
    const data=await res.json();
    document.getElementById('cfg-dldir').textContent=data.downloadDir||'—';
    const c=data.config||{};
    const setSelect=(id,val)=>{const el=document.getElementById(id);for(let i=0;i<el.options.length;i++){if(el.options[i].value===val){el.selectedIndex=i;break;}}};
    setSelect('cfg-folder',c.folderFormat||'channel');
    setSelect('cfg-file',c.fileFormat||'date_title');
    setSelect('cfg-format',c.videoFormat||'');
    setSelect('cfg-concurrent',String(c.maxConcurrent||1));
    setSelect('cfg-retention',String(c.defaultRetentionDays||0));
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
    maxConcurrent:parseInt(document.getElementById('cfg-concurrent').value)||1,
    defaultRetentionDays:parseInt(document.getElementById('cfg-retention').value)||0
  };
  const res=await fetch('/api/v1/config',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  if(res.ok){toast('Settings saved','ok');}else{toast('Failed to save','err');}
}
window.onload=function(){
  // Restore theme
  applyTheme();
  if(sessionStorage.getItem('sidebarCollapsed')==='1')document.getElementById('sidebar').classList.add('collapsed');
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
  initPager('sources-list','pager-sources',20);
  initPager('media-tbody','pager-media',20);
  initPager('tasks-tbody','pager-tasks',20);
  initPager('schedules-tbody','pager-schedules',20);
  // Restore filters
  const mf=sessionStorage.getItem('mediaFilter');
  if(mf){const el=document.getElementById('media-filter');if(el){el.value=mf;}}
  const msf=sessionStorage.getItem('mediaStatusFilter');
  if(msf){const el=document.getElementById('media-status-filter');if(el){el.value=msf;}}
  if(mf||msf)applyMediaFilters();
  const tsf=sessionStorage.getItem('taskStatusFilter');
  if(tsf){const el=document.getElementById('task-status-filter');if(el){el.value=tsf;filterTaskStatus(tsf);}}
  %s
};
window.onbeforeunload=function(){sessionStorage.setItem('scrollY',window.scrollY);};
</script>
</body></html>`

	addDateAfterOpts := buildDateAfterOptions("")

	// Cookie status for nav badge and cookies tab
	ckExists, ckSize, ckMtime, ckStatus, ckClass := a.cookieStatusForUI()
	// Status dot for sidebar: ok=green, warn/pending=amber, failed=red
	dotClass := "err"
	if ckClass == "badge-succeeded" {
		dotClass = "ok"
	} else if ckClass == "badge-pending" {
		dotClass = "warn"
	}
	ckNavBadge := fmt.Sprintf(`<span class="status-dot %s" title="%s"></span>`, dotClass, html.EscapeString(ckStatus))
	ckPathDisplay := html.EscapeString(a.cookiesPath)
	ckStatusBadge := fmt.Sprintf(`<span class="badge %s">%s</span>`, ckClass, html.EscapeString(ckStatus))
	if !ckExists {
		ckSize = "—"
		ckMtime = "—"
	}

	_, _ = w.Write([]byte(fmt.Sprintf(page,
		len(sources), len(allMedia), len(tasks), len(schedules),         // sidebar counts
		ckNavBadge,                                                       // cookies badge in sidebar
		addDateAfterOpts,                                                // add-source date filter options
		len(sources), strings.Join(sourceRows, ""),                      // sources tab: title, rows
		len(allMedia), mediaFilterOptions, strings.Join(mediaRows, ""), // media tab
		len(tasks), strings.Join(taskRows, ""),                         // tasks tab
		len(schedules), strings.Join(schedRows, ""),                    // schedules tab
		ckStatusBadge, ckPathDisplay, ckSize, ckMtime,                  // cookies tab: status, path, size, mtime
		autoRefreshJS,                                                   // auto-refresh if running tasks
	)))
}
