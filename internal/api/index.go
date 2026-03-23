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

// ── Schedule cell helpers ─────────────────────────────────────────────────────

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

func buildScheduleCell(sourceID string, schedBySource map[string]models.Schedule) string {
	defaultInterval := 24
	autoDLChecked := ""
	schedStatus := `<span class="src-meta">No schedule</span>`
	removeBtn := ""

	if sched, ok := schedBySource[sourceID]; ok {
		defaultInterval = sched.IntervalHours
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

	return fmt.Sprintf(
		`<div>%s</div>`+
			`<div style="display:flex;gap:6px;align-items:center;flex-wrap:wrap;margin-top:6px">`+
			`<select id="sched-interval-%s" style="width:auto;padding:4px 8px;font-size:12px">%s</select>`+
			`<label style="display:flex;align-items:center;gap:4px;font-size:12px;color:#94a3b8;cursor:pointer">`+
			`<input type="checkbox" id="sched-autodl-%s"%s style="width:auto;margin:0"> Auto-dl</label>`+
			`<button class="btn-sm btn-blue" onclick="setSchedule('%s')">Set</button>%s`+
			`</div>`,
		schedStatus,
		html.EscapeString(sourceID),
		buildIntervalOptions(defaultInterval),
		html.EscapeString(sourceID),
		autoDLChecked,
		html.EscapeString(sourceID),
		removeBtn,
	)
}

// ── Main handler ──────────────────────────────────────────────────────────────

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

		// Detail cell: title, kind/path/stage tags, URLs, summary, next action
		detail := fmt.Sprintf(
			`<div class="src-title">%s</div>`+
				`<div class="tags">%s%s%s</div>`+
				`<div class="src-meta">%s</div>`+
				`<div class="src-meta">%s</div>`+
				`<div class="src-meta">%s</div>`+
				`<div class="src-meta">%s</div>`,
			html.EscapeString(s.Title),
			tag(string(s.Kind)), tag(s.PathLabel), tag(s.IntakeStage),
			html.EscapeString(s.URL),
			html.EscapeString(s.InputURL),
			html.EscapeString(s.ValidationSummary),
			html.EscapeString(s.NextAction),
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
		dlBtn := ""
		if mediaCount > 0 {
			dlBtn = fmt.Sprintf(
				` <button class="btn-sm btn-green" onclick="downloadAll('%s')">Download all</button>`,
				html.EscapeString(s.ID),
			)
		}
		mediaCell := mediaInfo + viewBtn + dlBtn

		sourceRows = append(sourceRows, fmt.Sprintf(
			"<tr><td>%s</td><td>%s</td><td>%s</td><td>"+
				`<button class="btn-sm btn-gray" onclick="reindex('%s')">Re-index</button>`+
				"</td></tr>",
			detail,
			mediaCell,
			buildScheduleCell(s.ID, schedBySource),
			html.EscapeString(s.ID),
		))
	}
	if len(sourceRows) == 0 {
		sourceRows = []string{`<tr><td colspan="4" class="empty">No sources yet — add one above</td></tr>`}
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
			dur = fmt.Sprintf("%dm%ds", int(m.Duration)/60, int(m.Duration)%60)
		}
		fp := m.FilePath
		if fp == "" {
			fp = "—"
		}
		mediaRows = append(mediaRows, fmt.Sprintf(
			`<tr data-source="%s"><td>%s</td><td class="src-meta">%s</td><td>%s</td><td class="src-meta">%s</td></tr>`,
			html.EscapeString(m.SourceID),
			html.EscapeString(m.Title),
			html.EscapeString(dur),
			statusBadge(string(m.Status)),
			html.EscapeString(fp),
		))
	}
	if len(mediaRows) == 0 {
		mediaRows = []string{`<tr><td colspan="4" class="empty">No media yet — index a source first</td></tr>`}
	}

	// ── Task rows ─────────────────────────────────────────────────────────────
	var taskRows []string
	for _, t := range tasks {
		action := ""
		if t.Status == models.TaskStatusQueued {
			action = fmt.Sprintf(
				`<button class="btn-sm btn-blue" onclick="startTask('%s')">Start</button>`,
				html.EscapeString(t.ID),
			)
		}
		typeLabel := html.EscapeString(string(t.Type))
		if t.AutoDownload {
			typeLabel += ` <span class="tag">auto-dl</span>`
		}
		statusCell := statusBadge(string(t.Status))
		if t.ErrorCode != "" {
			statusCell += fmt.Sprintf(` <span class="src-meta">%s</span>`, html.EscapeString(t.ErrorCode))
		}
		createdAt := t.CreatedAt.UTC().Format("2006-01-02 15:04")
		taskRows = append(taskRows, fmt.Sprintf(
			`<tr><td>%s</td><td class="src-meta">%s</td><td>%s</td><td class="src-meta">%s</td><td>%s</td></tr>`,
			typeLabel,
			html.EscapeString(t.EntityID),
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
		autoLabel := "—"
		if sched.AutoDownload {
			autoLabel = "yes"
		}
		lastRun := "never"
		if sched.LastRunAt != nil {
			lastRun = sched.LastRunAt.UTC().Format("2006-01-02 15:04")
		}
		nextRun := sched.NextRunAt.UTC().Format("2006-01-02 15:04")
		removeBtn := fmt.Sprintf(
			`<button class="btn-sm btn-orange" onclick="removeSchedule(this,'%s')">Remove</button>`,
			html.EscapeString(sched.SourceID),
		)
		schedRows = append(schedRows, fmt.Sprintf(
			`<tr><td>%s</td><td class="src-meta">Every %dh</td><td class="src-meta">%s</td><td>%s</td><td class="src-meta">%s</td><td class="src-meta">%s</td><td>%s</td></tr>`,
			html.EscapeString(srcTitle),
			sched.IntervalHours,
			autoLabel,
			state,
			lastRun,
			nextRun,
			removeBtn,
		))
	}
	if len(schedRows) == 0 {
		schedRows = []string{`<tr><td colspan="7" class="empty">No schedules — set one from the Sources tab</td></tr>`}
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
.stat-pill{background:#172033;border:1px solid #1e3a5f;color:#93c5fd;font-size:11px;padding:2px 9px;border-radius:999px}
.main{padding:20px;max-width:1300px;margin:0 auto}
.tab-content{display:none}.tab-content.active{display:block}
.card{background:#121a2b;border:1px solid #1e293b;border-radius:12px;padding:18px;margin-bottom:18px}
.card-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:14px;flex-wrap:wrap;gap:8px}
.card-title{font-size:15px;font-weight:600;color:#f1f5f9}
.form-row{display:grid;grid-template-columns:140px 1fr;gap:10px;margin-bottom:10px;align-items:end}
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
td{padding:9px 8px;border-bottom:1px solid #111827;vertical-align:top}
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
.src-title{font-weight:600;color:#f1f5f9;margin-bottom:3px}
.src-meta{color:#94a3b8;font-size:12px}
.tags{display:flex;gap:4px;flex-wrap:wrap;margin:3px 0}
.filter-bar{display:flex;align-items:center;gap:8px}
.filter-bar select{width:auto}
#toast{position:fixed;bottom:20px;right:20px;z-index:999;display:flex;flex-direction:column;gap:6px;pointer-events:none}
.toast-msg{background:#1e293b;border-radius:10px;padding:10px 14px;font-size:13px;max-width:340px;border-left:3px solid #3b82f6;pointer-events:auto;animation:tslide .2s ease}
.toast-ok{border-color:#22c55e;color:#86efac}
.toast-err{border-color:#ef4444;color:#fca5a5}
@keyframes tslide{from{opacity:0;transform:translateY(6px)}to{opacity:1;transform:translateY(0)}}
.empty{color:#4b5563;text-align:center;padding:28px;font-size:13px}
@media(max-width:768px){.form-row{grid-template-columns:1fr}.nav-stats{display:none}}
</style>
</head>
<body>
<nav>
  <span class="logo">YtVault</span>
  <div class="nav-tabs">
    <button class="nav-tab" id="tab-btn-sources" onclick="showTab('sources')">Sources</button>
    <button class="nav-tab" id="tab-btn-media" onclick="showTab('media')">Media</button>
    <button class="nav-tab" id="tab-btn-tasks" onclick="showTab('tasks')">Tasks</button>
    <button class="nav-tab" id="tab-btn-schedules" onclick="showTab('schedules')">Schedules</button>
  </div>
  <div class="nav-stats">
    <span class="stat-pill">%d sources</span>
    <span class="stat-pill">%d media</span>
    <span class="stat-pill">%d tasks</span>
    <span class="stat-pill">%d schedules</span>
  </div>
</nav>
<div id="toast"></div>
<div class="main">

<div id="tab-sources" class="tab-content">
  <div class="card">
    <div class="card-header"><span class="card-title">Add Source</span></div>
    <div class="form-row">
      <div><label>Kind</label>
        <select id="kind">
          <option value="video">Video</option>
          <option value="playlist">Playlist</option>
          <option value="channel">Channel</option>
        </select>
      </div>
      <div><label>YouTube URL</label>
        <input type="text" id="url" placeholder="https://www.youtube.com/..." onkeydown="if(event.key==='Enter')submitForm()">
      </div>
    </div>
    <div style="display:flex;align-items:center;gap:12px">
      <button class="btn-primary" onclick="submitForm()">Add Source</button>
      <span id="add-status" class="src-meta"></span>
    </div>
  </div>
  <div class="card">
    <div class="card-header"><span class="card-title">Sources (%d)</span></div>
    <div class="table-wrap"><table>
      <thead><tr><th>Source</th><th>Media</th><th>Schedule</th><th>Actions</th></tr></thead>
      <tbody>%s</tbody>
    </table></div>
  </div>
</div>

<div id="tab-media" class="tab-content">
  <div class="card">
    <div class="card-header">
      <span class="card-title">Media Items (%d)</span>
      <div class="filter-bar">
        <span class="src-meta">Filter:</span>
        <select id="media-filter" onchange="filterMedia(this.value)">
          <option value="">All sources</option>%s
        </select>
      </div>
    </div>
    <div class="table-wrap"><table>
      <thead><tr><th>Title</th><th>Duration</th><th>Status</th><th>File</th></tr></thead>
      <tbody id="media-tbody">%s</tbody>
    </table></div>
  </div>
</div>

<div id="tab-tasks" class="tab-content">
  <div class="card">
    <div class="card-header"><span class="card-title">Tasks (%d)</span></div>
    <div class="table-wrap"><table>
      <thead><tr><th>Type</th><th>Entity</th><th>Status</th><th>Created</th><th></th></tr></thead>
      <tbody>%s</tbody>
    </table></div>
  </div>
</div>

<div id="tab-schedules" class="tab-content">
  <div class="card">
    <div class="card-header"><span class="card-title">Schedules (%d)</span></div>
    <div class="table-wrap"><table>
      <thead><tr><th>Source</th><th>Interval</th><th>Auto-DL</th><th>State</th><th>Last run</th><th>Next run</th><th></th></tr></thead>
      <tbody>%s</tbody>
    </table></div>
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
}
function toast(msg,type){
  const d=document.createElement('div');
  d.className='toast-msg toast-'+(type||'ok');
  d.textContent=msg;
  document.getElementById('toast').appendChild(d);
  setTimeout(()=>d.remove(),4000);
}
async function submitForm(){
  const kind=document.getElementById('kind').value;
  const url=document.getElementById('url').value.trim();
  const st=document.getElementById('add-status');
  if(!url){st.textContent='Please enter a URL';st.style.color='#fca5a5';return;}
  st.textContent='Adding...';st.style.color='#94a3b8';
  try{
    const res=await fetch('/api/v1/sources',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({kind,url})});
    const data=await res.json();
    if(res.ok){
      toast('Source added: '+data.source.title,'ok');
      setTimeout(()=>location.reload(),300);
    }else{
      const m=data.error?.message||JSON.stringify(data);
      st.textContent=m;st.style.color='#fca5a5';
      toast('Failed: '+m,'err');
    }
  }catch(e){st.textContent='Network error';st.style.color='#fca5a5';toast('Network error','err');}
}
async function reindex(sourceId){
  try{
    const res=await fetch('/api/v1/sources/'+sourceId+'/reindex',{method:'POST'});
    const data=await res.json();
    if(res.ok||res.status===202){
      toast('Re-index started','ok');
      setTimeout(()=>location.reload(),300);
    }else{
      toast('Re-index failed: '+(data.error?.message||JSON.stringify(data)),'err');
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
        toast('No pending media to download','ok');
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
async function setSchedule(sourceId){
  const iv=document.getElementById('sched-interval-'+sourceId);
  const ad=document.getElementById('sched-autodl-'+sourceId);
  const body={intervalHours:parseInt(iv.value),enabled:true,autoDownload:ad.checked};
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
  document.querySelectorAll('#media-tbody tr[data-source]').forEach(tr=>{
    tr.style.display=(sourceId===''||tr.dataset.source===sourceId)?'':'none';
  });
}
function viewMedia(sourceId){
  showTab('media');
  const f=document.getElementById('media-filter');
  if(f){f.value=sourceId;filterMedia(sourceId);}
}
window.onload=function(){
  const h=location.hash.slice(1);
  showTab(['sources','media','tasks','schedules'].includes(h)?h:'sources');
};
</script>
</body></html>`

	_, _ = w.Write([]byte(fmt.Sprintf(page,
		len(sources), len(allMedia), len(tasks), len(schedules), // nav stats
		len(sources), strings.Join(sourceRows, ""),               // sources tab: title, rows
		len(allMedia), mediaFilterOptions, strings.Join(mediaRows, ""), // media tab
		len(tasks), strings.Join(taskRows, ""),                         // tasks tab
		len(schedules), strings.Join(schedRows, ""),                    // schedules tab
	)))
}
