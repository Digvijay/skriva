// Package handler — admin HTML templates embedded as Go string constants.
// These are served directly from the binary without external files.
package handler

const adminDashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>Dashboard — Skriva</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{--sidebar-bg:#15171a;--sidebar-text:#a1a1aa;--sidebar-active:#fff;--sidebar-hover:#1e2028;--sidebar-width:260px;--bg:#f4f5f7;--card:#fff;--border:#e5e7eb;--primary:#6366f1;--primary-hover:#4f46e5;--danger:#ef4444;--text:#111827;--muted:#6b7280;--success:#10b981;--warning:#f59e0b}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:var(--bg);color:var(--text);line-height:1.5;display:flex;min-height:100vh}

/* Ghost-style sidebar */
.sidebar{width:var(--sidebar-width);background:var(--sidebar-bg);color:var(--sidebar-text);padding:1.5rem 0;display:flex;flex-direction:column;position:fixed;top:0;bottom:0;overflow-y:auto;z-index:50}
.sidebar-brand{padding:0 1.5rem;margin-bottom:2rem;display:flex;align-items:center;gap:.75rem}
.sidebar-brand .logo{width:36px;height:36px;background:linear-gradient(135deg,#6366f1,#8b5cf6);border-radius:8px;display:flex;align-items:center;justify-content:center;color:#fff;font-weight:800;font-size:1.1rem}
.sidebar-brand span{color:#fff;font-weight:700;font-size:1.1rem}
.sidebar-nav{flex:1}
.sidebar-nav a{display:flex;align-items:center;gap:.75rem;padding:.65rem 1.5rem;color:var(--sidebar-text);text-decoration:none;font-size:.9rem;font-weight:500;transition:all .15s;border-left:3px solid transparent}
.sidebar-nav a:hover{background:var(--sidebar-hover);color:#e5e5e5}
.sidebar-nav a.active{color:var(--sidebar-active);border-left-color:var(--primary);background:rgba(99,102,241,.1)}
.sidebar-nav a svg{width:18px;height:18px;flex-shrink:0;opacity:.6}
.sidebar-nav a.active svg{opacity:1}
.sidebar-section{padding:.5rem 1.5rem;font-size:.7rem;text-transform:uppercase;letter-spacing:.08em;color:#4b5563;margin-top:1.5rem;margin-bottom:.25rem}
.sidebar-bottom{padding:1rem 1.5rem;border-top:1px solid #2e3039}
.sidebar-bottom a{display:flex;align-items:center;gap:.75rem;color:var(--sidebar-text);text-decoration:none;font-size:.85rem;padding:.4rem 0}
.sidebar-bottom a:hover{color:#ef4444}

/* Main content */
.main-content{margin-left:var(--sidebar-width);flex:1;padding:2rem 2.5rem;max-width:1200px}
.page-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:2rem}
.page-header h1{font-size:1.6rem;font-weight:700;letter-spacing:-0.02em}
.stats-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:1rem;margin-bottom:2rem}
.stat-card{background:var(--card);border-radius:12px;padding:1.25rem 1.5rem;box-shadow:0 1px 3px rgba(0,0,0,.05);transition:box-shadow .15s}
.stat-card:hover{box-shadow:0 4px 12px rgba(0,0,0,.08)}
.stat-card .label{font-size:.75rem;text-transform:uppercase;letter-spacing:.06em;color:var(--muted);font-weight:600;margin-bottom:.35rem}
.stat-card .value{font-size:1.8rem;font-weight:700;letter-spacing:-0.02em}
.stat-card .value.primary{color:var(--primary)}
.stat-card .value.success{color:var(--success)}
.section{background:var(--card);border-radius:12px;margin-bottom:1.5rem;box-shadow:0 1px 3px rgba(0,0,0,.05);overflow:hidden}
.section-header{padding:1rem 1.5rem;border-bottom:1px solid var(--border);display:flex;justify-content:space-between;align-items:center}
.section-header h2{font-size:1rem;font-weight:600}
.btn{display:inline-flex;align-items:center;gap:.4rem;padding:.5rem 1rem;border-radius:8px;font-size:.85rem;font-weight:500;text-decoration:none;border:none;cursor:pointer;transition:all .15s}
.btn-primary{background:var(--primary);color:#fff}
.btn-primary:hover{background:var(--primary-hover)}
.btn-danger{background:var(--danger);color:#fff}
.btn-danger:hover{opacity:.9}
.btn-ghost{background:transparent;color:var(--muted);border:1px solid var(--border)}
.btn-ghost:hover{background:var(--bg);color:var(--text)}
.btn-sm{padding:.35rem .7rem;font-size:.8rem}
table{width:100%;border-collapse:collapse}
th,td{text-align:left;padding:.75rem 1.5rem;border-bottom:1px solid var(--border)}
th{font-size:.75rem;text-transform:uppercase;letter-spacing:.06em;color:var(--muted);font-weight:600;background:var(--bg)}
td{font-size:.9rem}
tr:hover td{background:rgba(99,102,241,.02)}
.badge{display:inline-block;padding:.2rem .65rem;border-radius:20px;font-size:.75rem;font-weight:600}
.badge-draft{background:#fef3c7;color:#92400e}
.badge-published{background:#d1fae5;color:#065f46}
.empty{padding:2.5rem;text-align:center;color:var(--muted);font-size:.9rem}
#loading{text-align:center;padding:4rem;color:var(--muted);font-size:.9rem}
@media(max-width:768px){.sidebar{display:none}.main-content{margin-left:0}}
</style>
</head>
<body>
<aside class="sidebar">
<div class="sidebar-brand">
<div class="logo">S</div>
<span>Skriva</span>
</div>
<nav class="sidebar-nav">
<a href="/admin/" class="active">
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>
Dashboard
</a>
<a href="/admin/editor">
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></svg>
New Post
</a>
<a href="/admin/newsletter">
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/></svg>
Newsletter
</a>
<a href="/admin/settings">
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
Settings
</a>
<div class="sidebar-section">Content</div>
<a href="/" target="_blank">
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg>
View Blog
</a>
</nav>
<div class="sidebar-bottom">
<a href="#" id="logoutBtn">
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
Sign out
</a>
</div>
</aside>
<div class="main-content">
<div id="loading">Loading dashboard...</div>
<div id="dashboard" style="display:none">
<div class="page-header">
<h1>Dashboard</h1>
<a href="/admin/editor" class="btn btn-primary">
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
New Post
</a>
</div>

<div class="stats-grid">
<div class="stat-card"><div class="label">Total Posts</div><div class="value" id="totalPosts">0</div></div>
<div class="stat-card"><div class="label">Total Views</div><div class="value" id="totalViews">0</div></div>
<div class="stat-card"><div class="label">Total Comments</div><div class="value" id="totalComments">0</div></div>
<div class="stat-card"><div class="label">Fedi Followers</div><div class="value" id="fediFollowers">0</div></div>
<div class="stat-card"><div class="label">Webmentions</div><div class="value" id="webmentionCount">0</div></div>
</div>

<div id="fediSection" class="section" style="display:none">
<div class="section-header"><h2>Fediverse</h2><span id="fediAddress" style="font-size:.8rem;color:var(--muted);font-family:monospace"></span></div>
<div style="padding:1rem 1.25rem">
<div style="display:flex;gap:2rem;flex-wrap:wrap">
<div>
<strong style="font-size:.8rem;text-transform:uppercase;color:var(--muted)">Your Fediverse Address</strong>
<p id="fediAddressDisplay" style="font-family:monospace;font-size:1rem;margin:.25rem 0"></p>
<p style="font-size:.75rem;color:var(--muted)">People can follow your blog from Mastodon, Misskey, Pleroma, etc.</p>
</div>
<div style="flex:1;min-width:200px">
<strong style="font-size:.8rem;text-transform:uppercase;color:var(--muted)">Followers</strong>
<div id="followersList" style="margin-top:.5rem;max-height:200px;overflow-y:auto"></div>
</div>
</div>
</div>
</div>

<div id="webmentionSection" class="section" style="display:none">
<div class="section-header"><h2>Webmentions</h2></div>
<table>
<thead><tr><th>Source</th><th>Post</th><th>Author</th><th>Type</th><th>Verified</th><th>Date</th><th></th></tr></thead>
<tbody id="webmentionsTable"></tbody>
</table>
<p id="webmentionsEmpty" style="padding:1rem 1.25rem;font-size:.85rem;color:var(--muted);display:none">No webmentions received yet.</p>
</div>

<div class="section">
<div class="section-header"><h2>Posts</h2>
<div style="display:flex;gap:.5rem;align-items:center">
<select id="bulkPostAction" style="padding:4px 8px;border:1px solid var(--border);border-radius:4px;font-size:.8rem"><option value="">Bulk Actions</option><option value="publish">Publish</option><option value="draft">Set Draft</option><option value="delete">Delete</option></select>
<button class="btn btn-sm" onclick="bulkPostOp()">Apply</button>
<a href="/admin/editor" class="btn btn-primary">New Post</a>
</div></div>
<div style="padding:.75rem 1.25rem;border-bottom:1px solid var(--border);display:flex;gap:.75rem;align-items:center;flex-wrap:wrap">
<label style="font-size:.8rem;color:var(--muted)">Per page:
<select id="postsPerPage" onchange="renderPosts()" style="padding:2px 6px;border:1px solid var(--border);border-radius:4px;font-size:.8rem"><option value="10" selected>10</option><option value="25">25</option><option value="50">50</option><option value="100">100</option></select>
</label>
<span id="postsPageInfo" style="font-size:.8rem;color:var(--muted);margin-left:auto"></span>
<button class="btn btn-sm" onclick="postsPage--;renderPosts()" id="postsPrev" disabled>&laquo; Prev</button>
<button class="btn btn-sm" onclick="postsPage++;renderPosts()" id="postsNext" disabled>Next &raquo;</button>
</div>
<table>
<thead><tr><th style="width:30px"><input type="checkbox" onchange="toggleAllPosts(this)"></th><th>Title</th><th>Date</th><th>Status</th><th>Actions</th></tr></thead>
<tbody id="postsTable"></tbody>
</table>
</div>

<div class="section">
<div class="section-header"><h2>Comments <span id="pendingBadge" style="display:none;background:#e3b341;color:#fff;padding:2px 8px;border-radius:12px;font-size:.75rem;margin-left:.5rem"></span></h2></div>
<div style="padding:.75rem 1.25rem;border-bottom:1px solid var(--border);display:flex;gap:.75rem;align-items:center;flex-wrap:wrap">
<label style="font-size:.8rem;color:var(--muted)">Filter by post:
<select id="commentFilter" onchange="commentsPage=0;renderComments()" style="padding:2px 6px;border:1px solid var(--border);border-radius:4px;font-size:.8rem"><option value="">All posts</option></select>
</label>
<label style="font-size:.8rem;color:var(--muted)">Per page:
<select id="commentsPerPage" onchange="commentsPage=0;renderComments()" style="padding:2px 6px;border:1px solid var(--border);border-radius:4px;font-size:.8rem"><option value="10" selected>10</option><option value="25">25</option><option value="50">50</option><option value="100">100</option></select>
</label>
<span id="commentsPageInfo" style="font-size:.8rem;color:var(--muted);margin-left:auto"></span>
<button class="btn btn-sm" onclick="commentsPage--;renderComments()" id="commentsPrev" disabled>&laquo; Prev</button>
<button class="btn btn-sm" onclick="commentsPage++;renderComments()" id="commentsNext" disabled>Next &raquo;</button>
</div>
<table>
<thead><tr><th>Author</th><th>Post</th><th>Comment</th><th>Status</th><th>Date</th><th></th></tr></thead>
<tbody id="commentsTable"></tbody>
</table>
</div>

</div>
</div>
<script>
let allPosts=[], allComments=[], postsPage=0, commentsPage=0;

// Restore saved preferences
(function(){
  const pp=localStorage.getItem('admin_postsPerPage');
  const cp=localStorage.getItem('admin_commentsPerPage');
  const cf=localStorage.getItem('admin_commentFilter');
  if(pp) document.getElementById('postsPerPage').value=pp;
  if(cp) document.getElementById('commentsPerPage').value=cp;
  // commentFilter options aren't populated yet, so we restore after data loads
  window._savedCommentFilter=cf||'';
})();

(async function(){
  try{
    const [statsRes,postsRes,commentsRes] = await Promise.all([
      fetch('/admin/api/stats'),fetch('/admin/api/posts'),fetch('/admin/api/comments')
    ]);
    if(!statsRes.ok||!postsRes.ok||!commentsRes.ok){
      if(statsRes.status===401)window.location.href='/admin/login';
      return;
    }
    const stats=await statsRes.json();
    allPosts=await postsRes.json()||[];
    allComments=await commentsRes.json()||[];

    document.getElementById('totalPosts').textContent=stats.total_posts||0;
    document.getElementById('totalViews').textContent=stats.total_views||0;
    document.getElementById('totalComments').textContent=stats.total_comments||0;

    // Load fediverse stats
    try{
      const fediRes=await fetch('/admin/api/fediverse');
      if(fediRes.ok){
        const fedi=await fediRes.json();
        document.getElementById('fediFollowers').textContent=fedi.follower_count||0;
        document.getElementById('webmentionCount').textContent=fedi.webmention_count||0;
        if(fedi.enabled){
          document.getElementById('fediSection').style.display='block';
          document.getElementById('fediAddress').textContent=fedi.fedi_address;
          document.getElementById('fediAddressDisplay').textContent=fedi.fedi_address;
          const fl=document.getElementById('followersList');
          if(fedi.followers&&fedi.followers.length){
            fl.innerHTML=fedi.followers.map(f=>'<div style="padding:4px 0;font-size:.85rem;display:flex;align-items:center;gap:.5rem"><a href="'+esc(f.actor_url)+'" target="_blank" rel="noopener" style="flex:1">'+esc(f.name||f.actor_url)+'</a><button onclick="deleteFollower('+f.id+')" style="background:none;border:none;color:#cf222e;cursor:pointer;font-size:.75rem" title="Remove follower">&times;</button></div>').join('');
          }else{fl.innerHTML='<span style="font-size:.85rem;color:var(--muted)">No followers yet. Share your Fediverse address!</span>';}
        }
      }
    }catch(e){}

    // Load webmentions
    try{
      const wmRes=await fetch('/admin/api/webmentions');
      if(wmRes.ok){
        const wms=await wmRes.json();
        if(wms&&wms.length){
          document.getElementById('webmentionSection').style.display='block';
          const tb=document.getElementById('webmentionsTable');
          tb.innerHTML=wms.map(wm=>'<tr><td style="max-width:200px;overflow:hidden;text-overflow:ellipsis"><a href="'+esc(wm.source)+'" target="_blank" rel="noopener" title="'+esc(wm.source)+'">'+esc(wm.source.replace(/https?:\/\//,'').substring(0,40))+'</a></td><td><a href="/'+esc(wm.post_slug)+'">'+esc(wm.post_slug)+'</a></td><td>'+esc(wm.author_name||'Unknown')+'</td><td><span style="padding:2px 6px;border-radius:4px;font-size:.75rem;background:'+(wm.mention_type==='reply'?'#1a7f37':wm.mention_type==='like'?'#e3b341':'#8b949e')+';color:#fff">'+esc(wm.mention_type)+'</span></td><td>'+(wm.verified?'&#10003;':'&#10007;')+'</td><td>'+new Date(wm.created_at).toLocaleDateString()+'</td><td><button onclick="deleteWebmention('+wm.id+')" class="btn-del">&times;</button></td></tr>').join('');
        }else{
          document.getElementById('webmentionSection').style.display='block';
          document.getElementById('webmentionsEmpty').style.display='block';
          document.getElementById('webmentionsTable').closest('table').style.display='none';
        }
      }
    }catch(e){}

    // Pending badge
    const pendingCount=allComments.filter(c=>!c.approved).length;
    if(pendingCount>0){const b=document.getElementById('pendingBadge');b.textContent=pendingCount+' pending';b.style.display='inline';}

    // Populate post filter for comments
    const filter=document.getElementById('commentFilter');
    const slugs=[...new Set(allComments.map(c=>c.post_slug))].sort();
    slugs.forEach(s=>{const o=document.createElement('option');o.value=s;o.textContent=s;filter.appendChild(o);});
    if(window._savedCommentFilter){filter.value=window._savedCommentFilter;}

    renderPosts();
    renderComments();

    document.getElementById('loading').style.display='none';
    document.getElementById('dashboard').style.display='block';
  }catch(e){document.getElementById('loading').textContent='Failed to load dashboard.';}
})();

function renderPosts(){
  const perPage=parseInt(document.getElementById('postsPerPage').value);
  localStorage.setItem('admin_postsPerPage',perPage);
  const total=allPosts.length;
  const totalPages=Math.max(1,Math.ceil(total/perPage));
  if(postsPage<0)postsPage=0;if(postsPage>=totalPages)postsPage=totalPages-1;
  const start=postsPage*perPage;
  const page=allPosts.slice(start,start+perPage);
  const pt=document.getElementById('postsTable');
  pt.innerHTML='';
  if(page.length){
    page.forEach(function(p){
      const tr=document.createElement('tr');
      const status=p.Draft?'<span class="badge badge-draft">Draft</span>':'<span class="badge badge-published">Published</span>';
      const date=p.Date?new Date(p.Date).toLocaleDateString():'\u2014';
      tr.innerHTML='<td><input type="checkbox" class="post-check" value="'+esc(p.Slug)+'"></td><td><a href="/admin/editor/'+esc(p.Slug)+'">'+esc(p.Title)+'</a></td><td>'+date+'</td><td>'+status+'</td><td><button class="btn btn-danger btn-sm" onclick="deletePost(\''+esc(p.Slug)+'\')">Delete</button></td>';
      pt.appendChild(tr);
    });
  }else{pt.innerHTML='<tr><td colspan="5" class="empty">No posts yet.</td></tr>';}
  document.getElementById('postsPrev').disabled=postsPage<=0;
  document.getElementById('postsNext').disabled=postsPage>=totalPages-1;
  document.getElementById('postsPageInfo').textContent='Page '+(postsPage+1)+' of '+totalPages+' ('+total+' posts)';
}

function renderComments(){
  const perPage=parseInt(document.getElementById('commentsPerPage').value);
  const filter=document.getElementById('commentFilter').value;
  localStorage.setItem('admin_commentsPerPage',perPage);
  localStorage.setItem('admin_commentFilter',filter);
  const filtered=filter?allComments.filter(c=>c.post_slug===filter):allComments;
  const total=filtered.length;
  const totalPages=Math.max(1,Math.ceil(total/perPage));
  if(commentsPage<0)commentsPage=0;if(commentsPage>=totalPages)commentsPage=totalPages-1;
  const start=commentsPage*perPage;
  const page=filtered.slice(start,start+perPage);
  const ct=document.getElementById('commentsTable');
  ct.innerHTML='';
  if(page.length){
    page.forEach(function(c){
      const tr=document.createElement('tr');
      const date=c.created_at?new Date(c.created_at).toLocaleDateString():'\u2014';
      const status=c.approved?'<span class="badge badge-published">Approved</span>':'<span class="badge badge-draft">Pending</span>';
      let actions='<button class="btn btn-danger btn-sm" onclick="deleteComment('+c.id+')">Delete</button>';
      if(!c.approved) actions='<button class="btn btn-primary btn-sm" onclick="approveComment('+c.id+')" style="margin-right:4px">Approve</button>'+actions;
      else actions='<button class="btn btn-sm" onclick="unapproveComment('+c.id+')" style="margin-right:4px;background:#e3b341;color:#fff">Unapprove</button>'+actions;
      tr.innerHTML='<td>'+esc(c.author)+'</td><td><a href="/'+esc(c.post_slug)+'" target="_blank">'+esc(c.post_slug)+'</a></td><td title="'+esc(c.content)+'">'+esc(c.content.substring(0,80))+(c.content.length>80?'...':'')+'</td><td>'+status+'</td><td>'+date+'</td><td style="white-space:nowrap">'+actions+'</td>';
      ct.appendChild(tr);
    });
  }else{ct.innerHTML='<tr><td colspan="6" class="empty">No comments'+(filter?' for this post':'')+'.</td></tr>';}
  document.getElementById('commentsPrev').disabled=commentsPage<=0;
  document.getElementById('commentsNext').disabled=commentsPage>=totalPages-1;
  document.getElementById('commentsPageInfo').textContent='Page '+(commentsPage+1)+' of '+totalPages+' ('+total+' comments)';
}

function esc(s){if(!s)return'';const d=document.createElement('div');d.textContent=s;return d.innerHTML.replace(/"/g,'&quot;').replace(/'/g,'&#39;');}

async function deletePost(slug){
  if(!confirm('Delete post "'+slug+'"? This cannot be undone.'))return;
  const r=await fetch('/admin/api/posts/'+slug,{method:'DELETE'});
  if(r.ok){allPosts=allPosts.filter(p=>p.Slug!==slug);renderPosts();}else alert('Failed to delete post.');
}
async function deleteComment(id){
  if(!confirm('Delete this comment?'))return;
  const r=await fetch('/admin/api/comments/'+id,{method:'DELETE'});
  if(r.ok){allComments=allComments.filter(c=>c.id!==id);renderComments();}else alert('Failed to delete comment.');
}
async function approveComment(id){
  const r=await fetch('/admin/api/comments/'+id+'/approve',{method:'POST'});
  if(r.ok){const c=allComments.find(c=>c.id===id);if(c)c.approved=true;renderComments();}else alert('Failed to approve comment.');
}
async function unapproveComment(id){
  const r=await fetch('/admin/api/comments/'+id+'/unapprove',{method:'POST'});
  if(r.ok){const c=allComments.find(c=>c.id===id);if(c)c.approved=false;renderComments();}else alert('Failed to unapprove comment.');
}
function toggleAllPosts(cb){document.querySelectorAll('.post-check').forEach(c=>c.checked=cb.checked);}
async function bulkPostOp(){
  const action=document.getElementById('bulkPostAction').value;
  if(!action){alert('Select an action');return;}
  const slugs=[...document.querySelectorAll('.post-check:checked')].map(c=>c.value);
  if(!slugs.length){alert('Select at least one post');return;}
  if(action==='delete'&&!confirm('Delete '+slugs.length+' posts? This cannot be undone.'))return;
  const r=await fetch('/admin/api/posts/bulk',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({action,slugs})});
  if(r.ok){const d=await r.json();alert(d.processed+' posts processed');location.reload();}else alert('Failed');
}
async function bulkCommentOp(){
  const action=document.getElementById('bulkCommentAction').value;
  if(!action){alert('Select an action');return;}
  const ids=[...document.querySelectorAll('.comment-check:checked')].map(c=>parseInt(c.value));
  if(!ids.length){alert('Select at least one comment');return;}
  if(action==='delete'&&!confirm('Delete '+ids.length+' comments?'))return;
  const r=await fetch('/admin/api/comments/bulk',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({action,ids})});
  if(r.ok){const d=await r.json();alert(d.processed+' comments processed');location.reload();}else alert('Failed');
}
async function deleteWebmention(id){
  if(!confirm('Delete this webmention?'))return;
  const r=await fetch('/admin/api/webmentions/'+id,{method:'DELETE'});
  if(r.ok){location.reload();}else alert('Failed to delete webmention.');
}
async function deleteFollower(id){
  if(!confirm('Remove this follower?'))return;
  const r=await fetch('/admin/api/followers/'+id,{method:'DELETE'});
  if(r.ok){location.reload();}else alert('Failed to remove follower.');
}
document.getElementById('logoutBtn').addEventListener('click',async function(e){
  e.preventDefault();
  await fetch('/admin/api/logout',{method:'POST'});
  window.location.href='/admin/login';
});
</script>
</body>
</html>`

// adminEditorHTML is the WYSIWYG post editor.
