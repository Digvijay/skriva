// Package handler — admin HTML templates embedded as Go string constants.
// These are served directly from the binary without external files.
package handler

const adminNewsletterHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>Newsletter — Blog Admin</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{--bg:#f8f9fa;--card:#fff;--border:#d0d7de;--primary:#0969da;--danger:#cf222e;--text:#1f2328;--muted:#656d76;--success:#1a7f37;--warning:#9a6700;--accent-subtle:#ddf4ff;--canvas-subtle:#f6f8fa;--shadow:0 1px 0 rgba(27,31,36,0.04);--shadow-md:0 3px 6px rgba(140,149,159,0.15);--radius:6px}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","Noto Sans",Helvetica,Arial,sans-serif;background:var(--bg);color:var(--text);line-height:1.5;font-size:14px}
.header{background:var(--card);border-bottom:1px solid var(--border);padding:.75rem 2rem;display:flex;justify-content:space-between;align-items:center;box-shadow:var(--shadow)}
.header h1{font-size:1.1rem;font-weight:600}
.header h1 a{color:var(--text);text-decoration:none}
.header h1 a:hover{color:var(--primary)}
.header nav{display:flex;gap:.75rem;align-items:center}
.header nav a{color:var(--primary);text-decoration:none;font-size:.85rem;font-weight:500;padding:5px 12px;border-radius:var(--radius)}
.header nav a:hover{background:var(--canvas-subtle)}
.container{max-width:960px;margin:2rem auto;padding:0 1.5rem}
.tabs{display:flex;gap:0;border-bottom:1px solid var(--border);margin-bottom:1.5rem}
.tab{padding:.5rem 1rem;font-size:.875rem;font-weight:500;color:var(--muted);cursor:pointer;border:none;background:none;border-bottom:2px solid transparent;transition:all .15s}
.tab:hover{color:var(--text)}
.tab.active{color:var(--text);border-bottom-color:var(--primary)}
.tab-content{display:none}
.tab-content.active{display:block}
.card{background:var(--card);border:1px solid var(--border);border-radius:var(--radius);margin-bottom:1.5rem;box-shadow:var(--shadow)}
.card-header{padding:1rem 1.25rem;border-bottom:1px solid var(--border);font-weight:600;font-size:.9rem;display:flex;justify-content:space-between;align-items:center}
.card-body{padding:1.25rem}
.form-group{margin-bottom:1rem}
.form-group label{display:block;font-size:.8rem;font-weight:600;margin-bottom:.3rem;color:var(--text)}
.form-group .hint{font-size:.75rem;color:var(--muted);margin-top:.25rem}
.form-control{width:100%;padding:5px 12px;font-size:14px;line-height:20px;color:var(--text);background:var(--card);border:1px solid var(--border);border-radius:var(--radius);font-family:inherit;box-shadow:inset var(--shadow)}
.form-control:focus{outline:none;border-color:var(--primary);box-shadow:0 0 0 3px rgba(9,105,218,.3)}
textarea.form-control{resize:vertical;min-height:200px;font-family:"SFMono-Regular",Consolas,"Liberation Mono",Menlo,monospace;font-size:13px;line-height:1.6}
.btn{display:inline-flex;align-items:center;padding:5px 16px;font-size:14px;font-weight:500;line-height:20px;border:1px solid transparent;border-radius:var(--radius);cursor:pointer;font-family:inherit;gap:4px;transition:.12s}
.btn-primary{background:var(--success);color:#fff;border-color:rgba(27,31,36,.15);box-shadow:var(--shadow)}
.btn-primary:hover{background:#1a7f37}
.btn-blue{background:var(--primary);color:#fff;border-color:rgba(27,31,36,.15)}
.btn-blue:hover{background:#0757b5}
.btn-outline{background:var(--card);color:var(--primary);border-color:var(--border);box-shadow:var(--shadow)}
.btn-outline:hover{background:var(--accent-subtle);border-color:var(--primary)}
.btn-danger{background:var(--danger);color:#fff;border-color:rgba(27,31,36,.15)}
.btn-danger:hover{background:#b31e28}
.btn-warning{background:#bf8700;color:#fff;border-color:rgba(27,31,36,.15)}
.btn-sm{padding:3px 10px;font-size:12px}
table{width:100%;border-collapse:collapse}
th,td{text-align:left;padding:.75rem 1.25rem;border-bottom:1px solid var(--border)}
th{font-size:.8rem;text-transform:uppercase;letter-spacing:.05em;color:var(--muted);font-weight:600}
td{font-size:.9rem}
.badge{display:inline-block;padding:.15rem .5rem;border-radius:12px;font-size:.75rem;font-weight:600}
.badge-draft{background:#fff8c5;color:var(--warning)}
.badge-scheduled{background:var(--accent-subtle);color:var(--primary)}
.badge-sending{background:#fff8c5;color:var(--warning)}
.badge-sent{background:#dafbe1;color:var(--success)}
.badge-failed{background:#ffebe9;color:var(--danger)}
.badge-confirmed{background:#dafbe1;color:var(--success)}
.badge-pending{background:#fff8c5;color:var(--warning)}
.empty{padding:2rem;text-align:center;color:var(--muted)}
.stats-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:1rem;margin-bottom:1.5rem}
.stat-card{background:var(--card);border:1px solid var(--border);border-radius:var(--radius);padding:1rem;text-align:center}
.stat-card .label{font-size:.75rem;text-transform:uppercase;letter-spacing:.05em;color:var(--muted)}
.stat-card .value{font-size:1.5rem;font-weight:700;margin-top:.25rem}
.modal-overlay{display:none;position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.5);z-index:100;justify-content:center;align-items:center}
.modal-overlay.show{display:flex}
.modal{background:var(--card);border-radius:var(--radius);box-shadow:var(--shadow-md);max-width:500px;width:90%;max-height:80vh;overflow-y:auto}
.modal-header{padding:1rem 1.25rem;border-bottom:1px solid var(--border);font-weight:600;display:flex;justify-content:space-between;align-items:center}
.modal-body{padding:1.25rem}
.modal-footer{padding:1rem 1.25rem;border-top:1px solid var(--border);display:flex;gap:.5rem;justify-content:flex-end}
.preview-pane{border:1px solid var(--border);border-radius:var(--radius);padding:1.25rem;min-height:200px;background:#fff;font-size:15px;line-height:1.7}
.toast{position:fixed;bottom:24px;right:24px;padding:12px 20px;border-radius:var(--radius);font-size:.875rem;font-weight:500;color:#fff;z-index:1000;transform:translateY(100px);opacity:0;transition:all .3s;box-shadow:var(--shadow-md)}
.toast.show{transform:translateY(0);opacity:1}
.toast-success{background:var(--success)}
.toast-error{background:var(--danger)}
@media(max-width:768px){.stats-grid{grid-template-columns:1fr 1fr}}
</style>
</head>
<body>
<div class="header">
<h1><a href="/admin/">&#8592; Dashboard</a> / Newsletter</h1>
<nav>
<a href="/admin/">Dashboard</a>
<a href="/admin/editor">New Post</a>
<a href="/admin/settings">Settings</a>
<a href="/" target="_blank">View Blog</a>
</nav>
</div>
<div class="container">
<div class="stats-grid">
<div class="stat-card"><div class="label">Subscribers</div><div class="value" id="subCount">0</div></div>
<div class="stat-card"><div class="label">Newsletters Sent</div><div class="value" id="sentCount">0</div></div>
<div class="stat-card"><div class="label">Scheduled</div><div class="value" id="scheduledCount">0</div></div>
<div class="stat-card"><div class="label">Drafts</div><div class="value" id="draftCount">0</div></div>
</div>

<div class="tabs">
<button class="tab active" onclick="switchTab('compose')">Compose</button>
<button class="tab" onclick="switchTab('newsletters')">All Newsletters</button>
<button class="tab" onclick="switchTab('subscribers')">Subscribers</button>
</div>

<!-- Compose Tab — Ghost-style distraction-free editor -->
<div id="tab-compose" class="tab-content active">
<div class="card" style="border:none;box-shadow:none;background:transparent">
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:1rem;padding:0 .5rem">
<span id="editorTitle" style="font-size:.85rem;color:var(--muted)">New Newsletter</span>
<div style="display:flex;gap:.5rem;align-items:center">
<button class="btn btn-outline btn-sm" onclick="togglePreview()">Preview</button>
<button class="btn btn-outline btn-sm" onclick="saveDraft()">Save Draft</button>
<button style="background:#6366f1;color:#fff;border:none;padding:5px 14px;border-radius:var(--radius);font-size:13px;font-weight:500;cursor:pointer" onclick="openScheduleModal()">Schedule</button>
<button style="background:#30cf43;color:#fff;border:none;padding:5px 14px;border-radius:var(--radius);font-size:13px;font-weight:600;cursor:pointer" onclick="sendNow()">Send Now</button>
</div>
</div>
<div style="max-width:700px;margin:0 auto;padding:1rem 0">
<input type="text" id="nlSubject" style="width:100%;font-size:2rem;font-weight:800;border:none;outline:none;background:transparent;letter-spacing:-0.02em;line-height:1.2;margin-bottom:12px;color:var(--text);padding:0" placeholder="Newsletter subject...">
<textarea id="nlBody" style="width:100%;min-height:50vh;font-family:Georgia,'Times New Roman',serif;font-size:17px;line-height:1.9;border:none;outline:none;resize:none;background:transparent;color:#374151;padding:0" placeholder="Begin writing your newsletter..."></textarea>
<div id="previewContainer" style="display:none;margin-top:1rem;border-top:2px solid var(--border);padding-top:1rem">
<div class="preview-pane" id="previewPane" style="font-family:Georgia,serif;font-size:17px;line-height:1.8"></div>
</div>
<input type="hidden" id="editingId" value="">
</div>
</div>
</div>

<!-- All Newsletters Tab -->
<div id="tab-newsletters" class="tab-content">
<div class="card">
<div class="card-header"><span>All Newsletters</span><button class="btn btn-primary btn-sm" onclick="switchTab('compose');resetEditor()">New Newsletter</button></div>
<table>
<thead><tr><th>Subject</th><th>Status</th><th>Scheduled</th><th>Sent</th><th>Actions</th></tr></thead>
<tbody id="newslettersTable"></tbody>
</table>
</div>
</div>

<!-- Subscribers Tab -->
<div id="tab-subscribers" class="tab-content">
<div class="card">
<div class="card-header"><span>Subscribers</span></div>
<table>
<thead><tr><th>Email</th><th>Status</th><th>Subscribed</th><th></th></tr></thead>
<tbody id="subscribersTable"></tbody>
</table>
</div>
</div>
</div>

<!-- Schedule Modal -->
<div class="modal-overlay" id="scheduleModal">
<div class="modal">
<div class="modal-header"><span>Schedule Newsletter</span><button onclick="closeScheduleModal()" style="background:none;border:none;font-size:1.2rem;cursor:pointer">&times;</button></div>
<div class="modal-body">
<div class="form-group">
<label for="scheduleDate">Date &amp; Time (UTC)</label>
<input type="datetime-local" id="scheduleDate" class="form-control">
<div class="hint">The newsletter will be sent automatically at this time.</div>
</div>
</div>
<div class="modal-footer">
<button class="btn btn-outline" onclick="closeScheduleModal()">Cancel</button>
<button class="btn btn-blue" onclick="confirmSchedule()">Schedule</button>
</div>
</div>
</div>

<div class="toast" id="toast"></div>

<script>
let allNewsletters=[], allSubscribers=[];
let previewVisible=false;
let previewTimer=null;

function showToast(msg,type){const t=document.getElementById('toast');t.textContent=msg;t.className='toast toast-'+type+' show';setTimeout(()=>t.classList.remove('show'),3000);}
function esc(s){if(!s)return'';const d=document.createElement('div');d.textContent=s;return d.innerHTML;}

function switchTab(name){
  document.querySelectorAll('.tab').forEach((t,i)=>{t.classList.remove('active');if(t.textContent.toLowerCase().includes(name.substring(0,4)))t.classList.add('active');});
  document.querySelectorAll('.tab-content').forEach(t=>t.classList.remove('active'));
  const el=document.getElementById('tab-'+name);if(el)el.classList.add('active');
}

async function loadData(){
  try{
    const [nlRes,subRes]=await Promise.all([
      fetch('/admin/api/newsletters'),fetch('/admin/api/subscribers')
    ]);
    if(nlRes.status===401){window.location.href='/admin/login';return;}
    allNewsletters=await nlRes.json()||[];
    allSubscribers=await subRes.json()||[];
    renderStats();
    renderNewsletters();
    renderSubscribers();
  }catch(e){showToast('Failed to load data','error');}
}

function renderStats(){
  const confirmed=allSubscribers.filter(s=>s.confirmed).length;
  document.getElementById('subCount').textContent=confirmed;
  document.getElementById('sentCount').textContent=allNewsletters.filter(n=>n.status==='sent').length;
  document.getElementById('scheduledCount').textContent=allNewsletters.filter(n=>n.status==='scheduled').length;
  document.getElementById('draftCount').textContent=allNewsletters.filter(n=>n.status==='draft').length;
}

function renderNewsletters(){
  const tb=document.getElementById('newslettersTable');
  if(!allNewsletters.length){tb.innerHTML='<tr><td colspan="5" class="empty">No newsletters yet. Create one in the Compose tab!</td></tr>';return;}
  tb.innerHTML=allNewsletters.map(n=>{
    const badge={'draft':'badge-draft','scheduled':'badge-scheduled','sending':'badge-sending','sent':'badge-sent','failed':'badge-failed'}[n.status]||'badge-draft';
    const sched=n.scheduled_at?new Date(n.scheduled_at).toLocaleString():'—';
    const sentInfo=n.status==='sent'?(n.sent_count+' sent'+(n.fail_count?' / '+n.fail_count+' failed':'')):'—';
    let actions='<button class="btn btn-sm btn-outline" onclick="editNewsletter('+n.id+')">Edit</button> ';
    if(n.status==='scheduled') actions+='<button class="btn btn-sm btn-warning" onclick="unscheduleNewsletter('+n.id+')">Unschedule</button> ';
    if(n.status==='draft'||n.status==='failed') actions+='<button class="btn btn-sm btn-danger" onclick="deleteNewsletter('+n.id+')">Delete</button>';
    return '<tr><td>'+esc(n.subject)+'</td><td><span class="badge '+badge+'">'+n.status+'</span></td><td>'+sched+'</td><td>'+sentInfo+'</td><td style="white-space:nowrap">'+actions+'</td></tr>';
  }).join('');
}

function renderSubscribers(){
  const tb=document.getElementById('subscribersTable');
  if(!allSubscribers.length){tb.innerHTML='<tr><td colspan="4" class="empty">No subscribers yet.</td></tr>';return;}
  tb.innerHTML=allSubscribers.map(s=>{
    const badge=s.confirmed?'badge-confirmed':'badge-pending';
    const label=s.confirmed?'Confirmed':'Pending';
    const date=s.created_at?new Date(s.created_at).toLocaleDateString():'—';
    return '<tr><td>'+esc(s.email)+'</td><td><span class="badge '+badge+'">'+label+'</span></td><td>'+date+'</td><td><button class="btn btn-sm btn-danger" onclick="deleteSubscriber('+s.id+')">Remove</button></td></tr>';
  }).join('');
}

function resetEditor(){
  document.getElementById('nlSubject').value='';
  document.getElementById('nlBody').value='';
  document.getElementById('editingId').value='';
  document.getElementById('editorTitle').textContent='New Newsletter';
  document.getElementById('previewContainer').style.display='none';
  previewVisible=false;
}

function editNewsletter(id){
  const n=allNewsletters.find(x=>x.id===id);
  if(!n)return;
  document.getElementById('nlSubject').value=n.subject;
  document.getElementById('nlBody').value=n.body;
  document.getElementById('editingId').value=n.id;
  document.getElementById('editorTitle').textContent='Edit Newsletter #'+n.id;
  switchTab('compose');
}

async function saveDraft(){
  const subject=document.getElementById('nlSubject').value.trim();
  const body=document.getElementById('nlBody').value;
  const editId=document.getElementById('editingId').value;
  if(!subject){showToast('Subject is required','error');return;}
  try{
    if(editId){
      const res=await fetch('/admin/api/newsletters/'+editId,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({subject,body})});
      if(!res.ok){const d=await res.json();showToast(d.error||'Failed to save','error');return;}
      showToast('Newsletter updated','success');
    }else{
      const res=await fetch('/admin/api/newsletters',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({subject,body})});
      if(!res.ok){const d=await res.json();showToast(d.error||'Failed to create','error');return;}
      const data=await res.json();
      document.getElementById('editingId').value=data.id;
      document.getElementById('editorTitle').textContent='Edit Newsletter #'+data.id;
      showToast('Draft created','success');
    }
    await loadData();
  }catch(e){showToast('Network error','error');}
}

async function sendNow(){
  const subject=document.getElementById('nlSubject').value.trim();
  const body=document.getElementById('nlBody').value;
  const editId=document.getElementById('editingId').value;
  if(!subject){showToast('Subject is required','error');return;}
  const confirmed=allSubscribers.filter(s=>s.confirmed).length;
  if(!confirm('Send this newsletter to '+confirmed+' subscriber'+(confirmed!==1?'s':'')+'?'))return;

  try{
    // Save first if needed
    let nlId=editId;
    if(!nlId){
      const res=await fetch('/admin/api/newsletters',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({subject,body})});
      if(!res.ok){const d=await res.json();showToast(d.error||'Failed to create','error');return;}
      const data=await res.json();
      nlId=data.id;
      document.getElementById('editingId').value=nlId;
    }else{
      await fetch('/admin/api/newsletters/'+nlId,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({subject,body})});
    }
    // Send
    const res=await fetch('/admin/api/newsletters/'+nlId+'/send',{method:'POST'});
    if(!res.ok){const d=await res.json();showToast(d.error||'Failed to send','error');return;}
    showToast('Newsletter is being sent!','success');
    resetEditor();
    await loadData();
  }catch(e){showToast('Network error','error');}
}

function openScheduleModal(){
  const subject=document.getElementById('nlSubject').value.trim();
  if(!subject){showToast('Subject is required','error');return;}
  // Default to 1 hour from now
  const d=new Date(Date.now()+3600000);
  document.getElementById('scheduleDate').value=d.toISOString().slice(0,16);
  document.getElementById('scheduleModal').classList.add('show');
}
function closeScheduleModal(){document.getElementById('scheduleModal').classList.remove('show');}

async function confirmSchedule(){
  const subject=document.getElementById('nlSubject').value.trim();
  const body=document.getElementById('nlBody').value;
  const editId=document.getElementById('editingId').value;
  const dt=document.getElementById('scheduleDate').value;
  if(!dt){showToast('Pick a date and time','error');return;}
  const scheduledAt=new Date(dt).toISOString();

  try{
    let nlId=editId;
    if(!nlId){
      const res=await fetch('/admin/api/newsletters',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({subject,body})});
      if(!res.ok){const d=await res.json();showToast(d.error||'Failed to create','error');return;}
      const data=await res.json();
      nlId=data.id;
      document.getElementById('editingId').value=nlId;
    }else{
      await fetch('/admin/api/newsletters/'+nlId,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({subject,body})});
    }
    const res=await fetch('/admin/api/newsletters/'+nlId+'/schedule',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({scheduled_at:scheduledAt})});
    if(!res.ok){const d=await res.json();showToast(d.error||'Failed to schedule','error');return;}
    showToast('Newsletter scheduled!','success');
    closeScheduleModal();
    resetEditor();
    await loadData();
  }catch(e){showToast('Network error','error');}
}

async function unscheduleNewsletter(id){
  if(!confirm('Move this newsletter back to draft?'))return;
  try{
    const res=await fetch('/admin/api/newsletters/'+id+'/unschedule',{method:'POST'});
    if(!res.ok){const d=await res.json();showToast(d.error||'Failed','error');return;}
    showToast('Moved back to draft','success');
    await loadData();
  }catch(e){showToast('Network error','error');}
}

async function deleteNewsletter(id){
  if(!confirm('Delete this newsletter?'))return;
  try{
    const res=await fetch('/admin/api/newsletters/'+id,{method:'DELETE'});
    if(!res.ok){showToast('Failed to delete','error');return;}
    showToast('Newsletter deleted','success');
    if(document.getElementById('editingId').value==id)resetEditor();
    await loadData();
  }catch(e){showToast('Network error','error');}
}

async function deleteSubscriber(id){
  if(!confirm('Remove this subscriber?'))return;
  try{
    const res=await fetch('/admin/api/subscribers/'+id,{method:'DELETE'});
    if(!res.ok){showToast('Failed to remove','error');return;}
    showToast('Subscriber removed','success');
    await loadData();
  }catch(e){showToast('Network error','error');}
}

function togglePreview(){
  previewVisible=!previewVisible;
  document.getElementById('previewContainer').style.display=previewVisible?'block':'none';
  if(previewVisible)renderPreview();
}
document.getElementById('nlBody').addEventListener('input',function(){
  if(previewVisible){clearTimeout(previewTimer);previewTimer=setTimeout(renderPreview,500);}
});
async function renderPreview(){
  const body=document.getElementById('nlBody').value;
  try{
    const res=await fetch('/admin/api/newsletters/preview',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({body})});
    if(res.ok){const d=await res.json();document.getElementById('previewPane').innerHTML=d.html;}
  }catch(e){}
}

// Init
loadData();
</script>
</body>
</html>`
