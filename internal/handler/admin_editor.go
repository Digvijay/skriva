// Package handler — admin editor page (Ghost-style distraction-free editor).
package handler

const adminEditorHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>Editor — Skriva</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{--bg:#fff;--surface:#f9fafb;--border:#e5e7eb;--primary:#30cf43;--primary-hover:#28b73a;--accent:#6366f1;--danger:#ef4444;--text:#111827;--muted:#9ca3af;--success:#30cf43;--dark:#15171a}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:var(--bg);color:var(--text);line-height:1.6}

/* === Top bar — Ghost-style minimal === */
.topbar{position:fixed;top:0;left:0;right:0;height:64px;background:var(--bg);display:flex;justify-content:space-between;align-items:center;padding:0 20px;z-index:100;border-bottom:1px solid transparent;transition:border-color .2s}
.topbar.scrolled{border-bottom-color:var(--border)}
.topbar-left{display:flex;align-items:center;gap:16px}
.topbar-left a{color:var(--muted);text-decoration:none;font-size:14px;display:flex;align-items:center;gap:6px;font-weight:500;transition:color .15s}
.topbar-left a:hover{color:var(--text)}
.topbar-left a svg{width:20px;height:20px}
.topbar-right{display:flex;align-items:center;gap:8px}
.topbar-status{font-size:13px;color:var(--muted);margin-right:8px}
.btn-publish{background:var(--primary);color:#fff;border:none;padding:7px 18px;border-radius:4px;font-size:14px;font-weight:600;cursor:pointer;transition:background .12s}
.btn-publish:hover{background:var(--primary-hover)}
.btn-outline{background:transparent;border:1px solid var(--border);color:var(--text);padding:6px 14px;border-radius:4px;font-size:13px;font-weight:500;cursor:pointer;transition:all .12s}
.btn-outline:hover{background:var(--surface);border-color:#ccc}
.btn-icon{background:none;border:none;color:var(--muted);padding:6px;border-radius:4px;cursor:pointer;display:flex;align-items:center}
.btn-icon:hover{color:var(--text);background:var(--surface)}
.btn-sm{padding:4px 10px;font-size:12px}

/* === Editor canvas — Ghost-style centered === */
.editor-canvas{max-width:740px;margin:0 auto;padding:88px 24px 60px}
#titleInput{width:100%;font-size:42px;font-weight:800;border:none;outline:none;background:transparent;letter-spacing:-0.025em;line-height:1.15;margin-bottom:8px;color:var(--text);font-family:inherit}
#titleInput::placeholder{color:#d1d5db}
#contentInput{width:100%;min-height:65vh;font-family:Georgia,"Times New Roman",Times,serif;font-size:18px;line-height:1.9;border:none;outline:none;resize:none;background:transparent;color:#374151}
#contentInput::placeholder{color:#d1d5db;font-style:italic}
#previewArea{display:none;font-family:Georgia,"Times New Roman",Times,serif;font-size:18px;line-height:1.9;color:#374151}

/* === Floating toolbar (on text selection) === */
.float-tb{position:fixed;top:-100px;left:50%;transform:translateX(-50%);background:var(--dark);border-radius:6px;padding:4px 6px;display:flex;gap:2px;box-shadow:0 4px 20px rgba(0,0,0,.3);z-index:200;opacity:0;transition:opacity .12s}
.float-tb.visible{opacity:1}
.float-tb button{background:none;border:none;color:#aaa;padding:5px 8px;border-radius:3px;cursor:pointer;font-size:13px;font-weight:600;transition:all .08s;font-family:inherit}
.float-tb button:hover{background:rgba(255,255,255,.12);color:#fff}
.float-tb .sep{width:1px;height:18px;background:#444;margin:0 2px;align-self:center}

/* === Settings drawer (Ghost-style slide-out) === */
.drawer-backdrop{position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.12);z-index:299;display:none}
.drawer-backdrop.open{display:block}
.drawer{position:fixed;top:0;right:0;bottom:0;width:400px;background:var(--bg);border-left:1px solid var(--border);z-index:300;transform:translateX(100%);transition:transform .2s ease;overflow-y:auto;box-shadow:-2px 0 20px rgba(0,0,0,.06)}
.drawer.open{transform:translateX(0)}
.drawer-head{display:flex;justify-content:space-between;align-items:center;padding:20px 24px;border-bottom:1px solid var(--border)}
.drawer-head h2{font-size:16px;font-weight:700}
.drawer-close{background:none;border:none;font-size:20px;cursor:pointer;color:var(--muted);padding:4px}
.drawer-close:hover{color:var(--text)}
.drawer-body{padding:24px}
.df{margin-bottom:20px}
.df label{display:block;font-size:13px;font-weight:600;color:var(--text);margin-bottom:4px}
.df .hint{font-size:11px;color:var(--muted);margin-top:3px}
.df input,.df textarea,.df select{width:100%;padding:8px 12px;border:1px solid var(--border);border-radius:4px;font-size:14px;font-family:inherit;transition:border-color .12s;background:var(--bg);color:var(--text)}
.df input:focus,.df textarea:focus{outline:none;border-color:var(--accent);box-shadow:0 0 0 3px rgba(99,102,241,.1)}
.df textarea{resize:vertical}
.df-check{display:flex;align-items:center;gap:8px;margin-bottom:12px}
.df-check input{width:16px;height:16px;accent-color:var(--accent)}
.df-check label{font-size:14px;color:var(--text)}
.df-tags{display:flex;flex-wrap:wrap;gap:4px;padding:6px 8px;border:1px solid var(--border);border-radius:4px;min-height:36px;align-items:center;cursor:text}
.df-tags:focus-within{border-color:var(--accent);box-shadow:0 0 0 3px rgba(99,102,241,.1)}
.df-tags input{border:none;outline:none;flex:1;min-width:60px;font-size:13px;padding:2px}
.tag{display:inline-flex;align-items:center;background:rgba(99,102,241,.08);color:var(--accent);padding:2px 8px;border-radius:10px;font-size:12px;font-weight:500}
.tag button{background:none;border:none;color:var(--accent);margin-left:4px;cursor:pointer;font-size:14px;line-height:1}
.dsec{font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:var(--muted);margin:24px 0 12px;padding-top:16px;border-top:1px solid var(--border)}
.d-img-preview{position:relative;margin-bottom:8px}
.d-img-preview img{width:100%;border-radius:6px;border:1px solid var(--border)}
.d-img-preview .rm{position:absolute;top:6px;right:6px;background:rgba(0,0,0,.7);color:#fff;border:none;border-radius:50%;width:22px;height:22px;cursor:pointer;font-size:13px;display:flex;align-items:center;justify-content:center}
.media-drop{border:2px dashed var(--border);border-radius:6px;padding:16px;text-align:center;cursor:pointer;transition:all .15s;font-size:13px;color:var(--muted)}
.media-drop:hover{border-color:var(--accent);background:rgba(99,102,241,.02)}
.media-drop input{display:none}

/* === Word count bar === */
.wordbar{position:fixed;bottom:0;left:0;right:0;height:28px;background:var(--surface);border-top:1px solid var(--border);display:flex;justify-content:space-between;align-items:center;padding:0 20px;font-size:12px;color:var(--muted)}
.wordbar .dot{display:inline-block;width:6px;height:6px;border-radius:50%;background:var(--success);margin-right:6px}
</style>
</head>
<body>

<!-- Top bar -->
<div class="topbar" id="topbar">
<div class="topbar-left">
<a href="/admin/"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6"/></svg>Posts</a>
</div>
<div class="topbar-right">
<span class="topbar-status" id="saveStatus"></span>
<button class="btn-outline" id="previewBtn" onclick="togglePreview()">Preview</button>
<button class="btn-icon" onclick="toggleSettings()" title="Post settings">
<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"></circle><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"></path></svg>
</button>
<button class="btn-publish" id="saveBtn" onclick="savePost()">Publish</button>
</div>
</div>

<!-- Floating toolbar -->
<div class="float-tb" id="floatingToolbar">
<button onclick="insertMD('**','**')"><strong>B</strong></button>
<button onclick="insertMD('*','*')"><em>I</em></button>
<span class="sep"></span>
<button onclick="insertMD('[','](url)')">Link</button>
<button onclick="insertMD('\\n## ','\\n')">H2</button>
<button onclick="insertMD('\\n### ','\\n')">H3</button>
<span class="sep"></span>
<button onclick="insertMD('\\n> ','\\n')">"</button>
<button onclick="insertCodeBlock()">&lt;/&gt;</button>
</div>

<!-- Editor canvas -->
<div class="editor-canvas">
<input type="text" id="titleInput" placeholder="Post title..." autofocus>
<textarea id="contentInput" placeholder="Begin writing your post..."></textarea>
<div id="previewArea"></div>
</div>

<!-- Settings drawer -->
<div class="drawer-backdrop" id="drawerBackdrop" onclick="toggleSettings()"></div>
<div class="drawer" id="settingsDrawer">
<div class="drawer-head">
<h2>Post settings</h2>
<button class="drawer-close" onclick="toggleSettings()">&times;</button>
</div>
<div class="drawer-body">
<div class="df"><label>Slug</label><input type="text" id="slugInput" placeholder="auto-generated-from-title"><div class="hint">URL-friendly version of the title</div></div>
<div class="df"><label>Excerpt</label><textarea id="descInput" rows="3" placeholder="A short summary for SEO and social sharing..."></textarea></div>
<div class="df"><label>Tags</label><div class="df-tags" id="tagsContainer" onclick="this.querySelector('input').focus()"><input type="text" id="tagInput" placeholder="Add a tag..."></div></div>
<div class="df">
<label>Feature image</label>
<input type="hidden" id="imageInput">
<div id="featuredImagePreview" class="d-img-preview" style="display:none">
<img id="featuredImageImg" alt="Featured image" onerror="this.style.display='none'">
<button class="rm" onclick="removeFeaturedImage()">&times;</button>
</div>
<div style="display:flex;gap:6px">
<label class="btn-outline btn-sm" style="cursor:pointer"><input type="file" accept="image/*" style="display:none" onchange="uploadFeaturedImage(this)"> Upload</label>
<button class="btn-outline btn-sm" type="button" onclick="toggleUnsplash()">Unsplash</button>
</div>
</div>
<div id="unsplashPanel" style="display:none;margin-bottom:16px">
<div style="display:flex;gap:4px;margin:8px 0">
<input type="text" id="unsplashQuery" placeholder="Search photos..." style="flex:1;padding:6px 10px;border:1px solid var(--border);border-radius:4px;font-size:13px" onkeydown="if(event.key==='Enter'){event.preventDefault();searchUnsplash()}">
<button class="btn-outline btn-sm" onclick="searchUnsplash()">Go</button>
</div>
<div id="unsplashResults" style="display:grid;grid-template-columns:1fr 1fr;gap:4px;max-height:200px;overflow-y:auto"></div>
<p id="unsplashEmpty" style="display:none;font-size:12px;color:var(--muted);text-align:center;padding:8px">No results.</p>
</div>

<div class="dsec">Publishing</div>
<div class="df-check"><input type="checkbox" id="draftCheck"><label for="draftCheck">Draft</label></div>
<div class="df-check"><input type="checkbox" id="featuredCheck"><label for="featuredCheck">Featured post</label></div>
<div class="df-check"><input type="checkbox" id="tocCheck"><label for="tocCheck">Table of contents</label></div>

<div class="dsec">Series</div>
<div class="df"><label>Series name</label><input type="text" id="seriesInput" placeholder="e.g. Learning Go"></div>
<div class="df"><label>Order in series</label><input type="number" id="seriesOrderInput" min="1" placeholder="1"></div>

<div class="dsec">Media</div>
<div class="media-drop" id="dropZone" onclick="document.getElementById('fileInput').click()">
<input type="file" id="fileInput" accept="image/*,video/*,audio/*" multiple>
Drop files or click to upload
</div>

<div class="dsec" id="shareSection" style="display:none">Sharing</div>
<div class="df" id="shareField" style="display:none">
<div id="shareInfo" style="font-size:13px"></div>
<button class="btn-outline btn-sm" style="margin-top:8px" onclick="createShareLink()">Generate share link</button>
</div>
<div class="dsec" id="revisionsSection" style="display:none">History</div>
<div class="df" id="revisionsField" style="display:none">
<div id="revisionsList" style="max-height:200px;overflow-y:auto"></div>
</div>
</div>
</div>

<!-- Word count bar -->
<div class="wordbar">
<div><span class="dot"></span><span id="wordCount">0 words</span></div>
<span id="readingTime">0 min read</span>
</div>

<script>
// Settings drawer toggle
function toggleSettings(){
  document.getElementById('settingsDrawer').classList.toggle('open');
  document.getElementById('drawerBackdrop').classList.toggle('open');
}

// Show floating toolbar on text selection
document.getElementById('contentInput').addEventListener('mouseup', showFloatingToolbar);
document.getElementById('contentInput').addEventListener('keyup', function(e){if(e.shiftKey)showFloatingToolbar();});
document.addEventListener('mousedown', function(e){
  if(!e.target.closest('.float-tb')&&!e.target.closest('#contentInput')){
    document.getElementById('floatingToolbar').classList.remove('visible');
    document.getElementById('floatingToolbar').style.top='-100px';
  }
});
function showFloatingToolbar(){
  const ta=document.getElementById('contentInput');
  if(ta.selectionStart===ta.selectionEnd)return;
  const tb=document.getElementById('floatingToolbar');
  const rect=ta.getBoundingClientRect();
  // Position above the textarea, centered
  const lineHeight=parseFloat(getComputedStyle(ta).lineHeight)||28;
  const lines=ta.value.substring(0,ta.selectionStart).split('\\n').length;
  const approxY=rect.top+lines*lineHeight-ta.scrollTop-40;
  tb.style.top=Math.max(70,approxY)+'px';
  tb.classList.add('visible');
}

// Topbar scroll shadow
window.addEventListener('scroll',function(){
  document.getElementById('topbar').classList.toggle('scrolled',window.scrollY>10);
});

// Keyboard shortcuts
document.getElementById('contentInput').addEventListener('keydown',function(e){
  if((e.ctrlKey||e.metaKey)&&e.key==='b'){e.preventDefault();insertMD('**','**');}
  if((e.ctrlKey||e.metaKey)&&e.key==='i'){e.preventDefault();insertMD('*','*');}
  if((e.ctrlKey||e.metaKey)&&e.key==='k'){e.preventDefault();insertMD('[','](url)');}
  if((e.ctrlKey||e.metaKey)&&e.key==='s'){e.preventDefault();savePost();}
});

// Show share/revisions sections when editing
if(window.location.pathname.startsWith('/admin/editor/')){
  var slug=window.location.pathname.replace('/admin/editor/','');
  if(slug){
    document.getElementById('shareSection').style.display='block';
    document.getElementById('shareField').style.display='block';
    document.getElementById('revisionsSection').style.display='block';
    document.getElementById('revisionsField').style.display='block';
  }
}

const isEdit = window.location.pathname.startsWith('/admin/editor/');
const editSlug = isEdit ? window.location.pathname.replace('/admin/editor/','') : null;
let tags = [];
let autoSaveTimer = null;

// Load existing post if editing
if(isEdit && editSlug){
  (async function(){
    const res = await fetch('/admin/api/posts');
    if(!res.ok){window.location.href='/admin/login';return;}
    const posts = await res.json();
    const post = posts.find(p => p.Slug === editSlug);
    if(!post){alert('Post not found');return;}
    document.getElementById('titleInput').value = post.Title||'';
    document.getElementById('slugInput').value = post.Slug||'';
    document.getElementById('descInput').value = post.Description||'';
    document.getElementById('contentInput').value = post.Content||'';
    if(post.Image){setFeaturedImage(post.Image.startsWith('/')?post.Image:'/media/'+post.Slug+'/'+post.Image);}
    else{document.getElementById('imageInput').value='';}
    document.getElementById('draftCheck').checked = !!post.Draft;
    document.getElementById('featuredCheck').checked = !!post.Featured;
    document.getElementById('tocCheck').checked = !!post.TOC;
    if(post.Series){document.getElementById('seriesInput').value = post.Series;}
    if(post.SeriesOrder){document.getElementById('seriesOrderInput').value = post.SeriesOrder;}
    if(post.Tags){post.Tags.forEach(addTag);}
    document.getElementById('saveBtn').textContent = 'Update';
    updateWordCount();
  })();
}

// Tags input
document.getElementById('tagInput').addEventListener('keydown',function(e){
  if((e.key==='Enter'||e.key===',')&&this.value.trim()){
    e.preventDefault();
    addTag(this.value.trim().replace(',',''));
    this.value='';
  }
  if(e.key==='Backspace'&&!this.value&&tags.length){
    tags.pop();
    renderTags();
  }
});
function addTag(t){
  t=t.toLowerCase().trim();
  if(t&&!tags.includes(t)){tags.push(t);renderTags();}
}
function removeTag(t){tags=tags.filter(x=>x!==t);renderTags();}
function renderTags(){
  const c=document.getElementById('tagsContainer');
  const inp=document.getElementById('tagInput');
  c.querySelectorAll('.tag').forEach(el=>el.remove());
  tags.forEach(t=>{
    const span=document.createElement('span');span.className='tag';
    span.textContent=t;
    const btn=document.createElement('button');
    btn.textContent='\u00d7';
    btn.addEventListener('click',function(){removeTag(t);});
    span.appendChild(btn);
    c.insertBefore(span,inp);
  });
}

// Auto-generate slug from title
document.getElementById('titleInput').addEventListener('input',function(){
  if(!isEdit){
    const slug=this.value.toLowerCase().replace(/[^a-z0-9]+/g,'-').replace(/^-|-$/g,'');
    document.getElementById('slugInput').value=slug;
  }
  scheduleAutoSave();
  updateWordCount();
});
document.getElementById('contentInput').addEventListener('input',function(){
  scheduleAutoSave();
  updateWordCount();
});

// Word count & reading time
function updateWordCount(){
  const text=document.getElementById('contentInput').value;
  const words=text.trim()?text.trim().split(/\s+/).length:0;
  document.getElementById('wordCount').textContent=words+' words';
  document.getElementById('readingTime').textContent=Math.max(1,Math.ceil(words/200))+' min read';
}

// Auto-save (draft) every 30 seconds
function scheduleAutoSave(){
  clearTimeout(autoSaveTimer);
  autoSaveTimer=setTimeout(()=>{
    document.getElementById('saveStatus').textContent='Auto-saving...';
    // Store to localStorage as backup
    const data={title:document.getElementById('titleInput').value,content:document.getElementById('contentInput').value,slug:document.getElementById('slugInput').value};
    localStorage.setItem('blog_editor_draft',JSON.stringify(data));
    document.getElementById('saveStatus').textContent='Draft saved locally';
    setTimeout(()=>{document.getElementById('saveStatus').textContent='';},2000);
  },30000);
}

// Save post
async function savePost(){
  const btn=document.getElementById('saveBtn');
  btn.disabled=true;btn.textContent='Saving...';
  const payload={
    title:document.getElementById('titleInput').value,
    slug:document.getElementById('slugInput').value,
    content:document.getElementById('contentInput').value,
    description:document.getElementById('descInput').value,
    image:document.getElementById('imageInput').value,
    tags:tags,
    draft:document.getElementById('draftCheck').checked,
    featured:document.getElementById('featuredCheck').checked,
    toc:document.getElementById('tocCheck').checked,
    series:document.getElementById('seriesInput').value,
    series_order:parseInt(document.getElementById('seriesOrderInput').value)||0,
  };
  try{
    const url=isEdit?'/admin/api/posts/'+editSlug:'/admin/api/posts';
    const method=isEdit?'PUT':'POST';
    const res=await fetch(url,{method,headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});
    const data=await res.json();
    if(res.ok){
      localStorage.removeItem('blog_editor_draft');
      document.getElementById('saveStatus').textContent='Saved!';
      if(!isEdit)window.location.href='/admin/editor/'+data.slug;
    }else{alert(data.error||'Failed to save');}
  }catch(e){alert('Network error');}
  btn.disabled=false;btn.textContent=isEdit?'Update':'Publish';
}

// Live preview with debounce
let previewMode = false;
let previewTimer = null;

async function togglePreview(){
  const ta=document.getElementById('contentInput');
  const pa=document.getElementById('previewArea');
  const btn=document.getElementById('previewBtn');
  const canvas=document.querySelector('.editor-canvas');
  previewMode = !previewMode;
  if(previewMode){
    if(!document.getElementById('previewThemeCSS')){
      const link=document.createElement('link');link.id='previewThemeCSS';link.rel='stylesheet';link.href='/theme/css/theme.css';document.head.appendChild(link);
      const syn=document.createElement('link');syn.id='previewSyntaxCSS';syn.rel='stylesheet';syn.href='/theme/css/syntax.css';document.head.appendChild(syn);
    }
    canvas.style.display='grid';
    canvas.style.gridTemplateColumns='1fr 1fr';
    canvas.style.gap='2rem';
    canvas.style.maxWidth='1200px';
    ta.style.display='block';
    ta.style.minHeight='60vh';
    pa.style.display='block';
    pa.style.overflow='auto';
    pa.style.borderLeft='2px solid var(--border)';
    pa.style.paddingLeft='1.5rem';
    document.getElementById('titleInput').style.gridColumn='1 / -1';
    btn.textContent='Edit Only';
    renderPreview();
    ta.addEventListener('input', debouncedPreview);
  } else {
    canvas.style.display='block';
    canvas.style.maxWidth='740px';
    ta.style.display='block';
    pa.style.display='none';
    pa.style.borderLeft='';
    pa.style.paddingLeft='';
    btn.textContent='Preview';
    ta.removeEventListener('input', debouncedPreview);
  }
}

function debouncedPreview(){
  clearTimeout(previewTimer);
  previewTimer = setTimeout(renderPreview, 500);
}

async function renderPreview(){
  const ta=document.getElementById('contentInput');
  const pa=document.getElementById('previewArea');
  try{
    const res=await fetch('/admin/api/preview',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({markdown:ta.value})});
    if(res.ok){const d=await res.json();pa.innerHTML='<div class="post-content markdown-body">'+d.html+'</div>';}
  }catch(e){}
}

// File upload via drag & drop — insert at cursor position
const dropZone=document.getElementById('dropZone');
const fileInput=document.getElementById('fileInput');
dropZone.addEventListener('click',()=>fileInput.click());
dropZone.addEventListener('dragover',e=>{e.preventDefault();dropZone.classList.add('drag-over');});
dropZone.addEventListener('dragleave',()=>dropZone.classList.remove('drag-over'));
dropZone.addEventListener('drop',e=>{e.preventDefault();dropZone.classList.remove('drag-over');uploadFiles(e.dataTransfer.files);});
fileInput.addEventListener('change',()=>uploadFiles(fileInput.files));

// Also support drag & drop on the textarea itself
const contentTA=document.getElementById('contentInput');
contentTA.addEventListener('dragover',e=>{e.preventDefault();});
contentTA.addEventListener('drop',e=>{e.preventDefault();if(e.dataTransfer.files.length)uploadFiles(e.dataTransfer.files);});

// Paste images from clipboard
contentTA.addEventListener('paste',e=>{
  const items=e.clipboardData&&e.clipboardData.items;
  if(!items)return;
  for(const item of items){
    if(item.type.startsWith('image/')){
      e.preventDefault();
      const file=item.getAsFile();
      if(file)uploadFiles([file]);
      return;
    }
  }
});

function insertAtCursor(text){
  const ta=document.getElementById('contentInput');
  const start=ta.selectionStart;
  const end=ta.selectionEnd;
  ta.value=ta.value.substring(0,start)+text+ta.value.substring(end);
  ta.selectionStart=ta.selectionEnd=start+text.length;
  ta.focus();
  updateWordCount();
  if(previewMode)debouncedPreview();
}

async function uploadFiles(files){
  const slug=document.getElementById('slugInput').value||'untitled';
  for(const file of files){
    const fd=new FormData();fd.append('file',file);
    try{
      const res=await fetch('/admin/api/media/'+slug,{method:'POST',body:fd});
      const data=await res.json();
      if(res.ok){
        let insert='';
        if(file.type.startsWith('image/'))insert='\n!['+file.name+']('+data.url+')\n';
        else if(file.type.startsWith('video/'))insert='\n<video src="'+data.url+'" controls></video>\n';
        else if(file.type.startsWith('audio/'))insert='\n<audio src="'+data.url+'" controls></audio>\n';
        else insert='\n['+file.name+']('+data.url+')\n';
        insertAtCursor(insert);
      }else{alert(data.error||'Upload failed');}
    }catch(e){alert('Upload error: '+e.message);}
  }
}

// --- Featured Image ---
function removeFeaturedImage(){
  document.getElementById('imageInput').value='';
  document.getElementById('featuredImagePreview').style.display='none';
}
function setFeaturedImage(url){
  document.getElementById('imageInput').value=url;
  document.getElementById('featuredImageImg').src=url;
  document.getElementById('featuredImagePreview').style.display='block';
}
async function uploadFeaturedImage(input){
  if(!input.files||!input.files[0])return;
  const slug=document.getElementById('slugInput').value||'untitled';
  const fd=new FormData();fd.append('file',input.files[0]);
  try{
    const res=await fetch('/admin/api/media/'+slug,{method:'POST',body:fd});
    const data=await res.json();
    if(res.ok)setFeaturedImage(data.url);
    else alert(data.error||'Upload failed');
  }catch(e){alert('Upload error');}
}

// --- Unsplash ---
let unsplashOpen=false;
function toggleUnsplash(){
  unsplashOpen=!unsplashOpen;
  document.getElementById('unsplashPanel').style.display=unsplashOpen?'block':'none';
  if(unsplashOpen)document.getElementById('unsplashQuery').focus();
}
async function searchUnsplash(){
  const q=document.getElementById('unsplashQuery').value.trim();
  if(!q)return;
  const results=document.getElementById('unsplashResults');
  const empty=document.getElementById('unsplashEmpty');
  results.innerHTML='<p style="grid-column:1/-1;text-align:center;font-size:.8rem;color:var(--muted)">Searching...</p>';
  empty.style.display='none';
  try{
    const res=await fetch('/admin/api/unsplash?query='+encodeURIComponent(q));
    if(res.status===503){results.innerHTML='<p style="grid-column:1/-1;text-align:center;font-size:.8rem;color:var(--muted)">Unsplash API key not configured.<br>Add unsplash_access_key to secrets.yaml</p>';return;}
    if(!res.ok){results.innerHTML='';empty.style.display='block';return;}
    const data=await res.json();
    if(!data.results||!data.results.length){results.innerHTML='';empty.style.display='block';return;}
    results.innerHTML=data.results.map(img=>{
      const thumb=img.urls.small;
      const full=img.urls.regular;
      const credit=img.user.name;
      const creditUrl=img.user.links.html+'?utm_source=skriva&utm_medium=referral';
      return '<div style="cursor:pointer;position:relative" onclick="useUnsplashImage(\''+full+'\',\''+credit.replace(/'/g,"\\'")+'\',\''+creditUrl+'\')">'
        +'<img src="'+thumb+'" style="width:100%;height:80px;object-fit:cover;border-radius:4px">'
        +'<span style="position:absolute;bottom:2px;left:2px;font-size:.6rem;color:#fff;text-shadow:0 1px 2px rgba(0,0,0,.7)">'+credit+'</span>'
        +'</div>';
    }).join('');
  }catch(e){results.innerHTML='';empty.style.display='block';}
}
function useUnsplashImage(url,credit,creditUrl){
  // Ask: insert into post or set as featured?
  const action=confirm('Set as featured image?\\n\\nOK = Featured image\\nCancel = Insert into post');
  if(action){
    setFeaturedImage(url);
  }else{
    insertAtCursor('\\n![Photo by '+credit+']('+url+')\\n*Photo by ['+credit+']('+creditUrl+') on [Unsplash](https://unsplash.com?utm_source=skriva&utm_medium=referral)*\\n');
  }
  toggleUnsplash();
}
// --- Draft Sharing ---
async function createShareLink(){
  const slug=document.getElementById('slugInput').value;
  if(!slug){alert('Save the post first');return;}
  const res=await fetch('/admin/api/posts/'+slug+'/share',{method:'POST'});
  if(res.ok){const d=await res.json();document.getElementById('shareInfo').innerHTML='<input type="text" value="'+d.url+'" readonly style="width:100%;font-size:.75rem;padding:4px" onclick="this.select()"><br><span style="font-size:.7rem;color:var(--muted)">Expires: '+new Date(d.expires_at).toLocaleDateString()+'</span> <button class="btn btn-sm" style="font-size:.7rem" onclick="revokeShare()">Revoke</button>';}
  else alert('Failed to create share link');
}
async function revokeShare(){
  const slug=document.getElementById('slugInput').value;
  await fetch('/admin/api/posts/'+slug+'/share',{method:'DELETE'});
  document.getElementById('shareInfo').innerHTML='<span style="color:var(--muted);font-size:.8rem">No active share link</span>';
}
// --- Revision History ---
async function loadRevisions(){
  const slug=document.getElementById('slugInput').value;
  if(!slug)return;
  const res=await fetch('/admin/api/posts/'+slug+'/revisions');
  if(!res.ok)return;
  const revs=await res.json();
  const el=document.getElementById('revisionsList');
  if(!revs||!revs.length){el.innerHTML='<span style="color:var(--muted)">No revisions yet</span>';return;}
  el.innerHTML=revs.map(r=>'<div style="padding:4px 0;border-bottom:1px solid var(--border)"><strong>'+new Date(r.created_at).toLocaleString()+'</strong> — '+r.title.substring(0,40)+' <button class="btn btn-sm" style="font-size:.7rem;padding:1px 6px" onclick="restoreRevision('+r.id+')">Restore</button></div>').join('');
}
async function restoreRevision(id){
  if(!confirm('Restore this revision? Current version will be saved as a new revision.'))return;
  const res=await fetch('/admin/api/revisions/'+id+'/restore',{method:'POST'});
  if(res.ok){alert('Revision restored!');location.reload();}else alert('Failed to restore');
}
// Show share/revision sections when editing (also handled in new drawer code above)
if(isEdit&&editSlug){
  document.getElementById('shareSection').style.display='block';
  document.getElementById('shareField').style.display='block';
  document.getElementById('revisionsSection').style.display='block';
  document.getElementById('revisionsField').style.display='block';
  setTimeout(loadRevisions,500);
}
</script>
</body>
</html>`

// adminSettingsHTML is the settings page for editing site config, switching themes, and previewing themes.
