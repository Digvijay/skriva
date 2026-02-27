// Package handler — admin HTML templates embedded as Go string constants.
// These are served directly from the binary without external files.
package handler

const adminSettingsHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>Settings — Blog Admin</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{--bg:#f8f9fa;--card:#fff;--border:#d0d7de;--primary:#0969da;--danger:#cf222e;--text:#1f2328;--muted:#656d76;--success:#1a7f37;--accent-subtle:#ddf4ff;--canvas-subtle:#f6f8fa;--shadow:0 1px 0 rgba(27,31,36,0.04);--shadow-md:0 3px 6px rgba(140,149,159,0.15);--radius:6px}
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
textarea.form-control{resize:vertical;min-height:80px}
.form-row{display:grid;grid-template-columns:1fr 1fr;gap:1rem}
.form-row-3{display:grid;grid-template-columns:1fr 1fr 1fr;gap:1rem}
.btn{display:inline-flex;align-items:center;padding:5px 16px;font-size:14px;font-weight:500;line-height:20px;border:1px solid transparent;border-radius:var(--radius);cursor:pointer;font-family:inherit;gap:4px;transition:.12s}
.btn-primary{background:var(--success);color:#fff;border-color:rgba(27,31,36,.15);box-shadow:var(--shadow)}
.btn-primary:hover{background:#1a7f37}
.btn-outline{background:var(--card);color:var(--primary);border-color:var(--border);box-shadow:var(--shadow)}
.btn-outline:hover{background:var(--accent-subtle);border-color:var(--primary)}
.btn-sm{padding:3px 10px;font-size:12px}
.btn-danger{background:var(--danger);color:#fff;border-color:rgba(27,31,36,.15)}
.btn-danger:hover{background:#b31e28}
.themes-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:1rem}
.theme-card{background:var(--card);border:1px solid var(--border);border-radius:var(--radius);overflow:hidden;transition:all .15s;position:relative}
.theme-card:hover{box-shadow:var(--shadow-md)}
.theme-card.active{border-color:var(--primary);box-shadow:0 0 0 2px rgba(9,105,218,.3)}
.theme-card.active::after{content:"Active";position:absolute;top:8px;right:8px;background:var(--success);color:#fff;font-size:11px;font-weight:600;padding:2px 8px;border-radius:2em}
.theme-preview{height:160px;background:var(--canvas-subtle);border-bottom:1px solid var(--border);overflow:hidden;position:relative}
.theme-preview iframe{width:200%;height:200%;transform:scale(0.5);transform-origin:top left;border:none;pointer-events:none}
.theme-info{padding:1rem}
.theme-name{font-size:.9rem;font-weight:600;margin-bottom:.25rem}
.theme-desc{font-size:.8rem;color:var(--muted);margin-bottom:.5rem;line-height:1.4}
.theme-meta{font-size:.75rem;color:var(--muted);display:flex;gap:.75rem}
.theme-actions{display:flex;gap:.5rem;margin-top:.75rem}
.footer-sections{border:1px solid var(--border);border-radius:var(--radius);padding:1rem;margin-bottom:.5rem}
.footer-section-item{padding:.75rem;border:1px solid var(--border);border-radius:var(--radius);margin-bottom:.5rem;background:var(--canvas-subtle)}
.footer-section-item .section-header-row{display:flex;justify-content:space-between;align-items:center;margin-bottom:.5rem}
.footer-link-row{display:flex;gap:.5rem;margin-bottom:.25rem;align-items:center}
.footer-link-row input{flex:1}
.footer-link-row button{flex-shrink:0}
.toast{position:fixed;bottom:24px;right:24px;padding:12px 20px;border-radius:var(--radius);font-size:.875rem;font-weight:500;color:#fff;z-index:1000;transform:translateY(100px);opacity:0;transition:all .3s;box-shadow:var(--shadow-md)}
.toast.show{transform:translateY(0);opacity:1}
.toast-success{background:var(--success)}
.toast-error{background:var(--danger)}
@media(max-width:768px){.form-row,.form-row-3{grid-template-columns:1fr}.themes-grid{grid-template-columns:1fr}}
</style>
</head>
<body>
<div class="header">
<h1><a href="/admin/">← Dashboard</a> / Settings</h1>
<nav>
<a href="/admin/">Dashboard</a>
<a href="/admin/editor">New Post</a>
<a href="/" target="_blank">View Blog</a>
</nav>
</div>
<div class="container">
<div class="tabs">
<button class="tab active" onclick="switchTab('general')">General</button>
<button class="tab" onclick="switchTab('social')">Social & Author</button>
<button class="tab" onclick="switchTab('footer')">Footer</button>
<button class="tab" onclick="switchTab('themes')">Themes</button>
<button class="tab" onclick="switchTab('indieweb')">IndieWeb</button>
<button class="tab" onclick="switchTab('security')">Security</button>
</div>

<!-- General Tab -->
<div id="tab-general" class="tab-content active">
<div class="card">
<div class="card-header">Site Settings</div>
<div class="card-body">
<div class="form-group">
<label for="siteTitle">Site Title</label>
<input type="text" id="siteTitle" class="form-control" placeholder="My Blog">
<div class="hint">The name of your blog, shown in the header and browser title.</div>
</div>
<div class="form-group">
<label for="siteTagline">Tagline</label>
<input type="text" id="siteTagline" class="form-control" placeholder="A short description of your blog">
<div class="hint">Displayed under the title. Also used as default meta description.</div>
</div>
<div class="form-row">
<div class="form-group">
<label for="siteBaseURL">Base URL</label>
<input type="url" id="siteBaseURL" class="form-control" placeholder="https://example.com">
<div class="hint">The full URL of your blog (no trailing slash).</div>
</div>
<div class="form-group">
<label for="sitePostsPerPage">Posts Per Page</label>
<input type="number" id="sitePostsPerPage" class="form-control" min="1" max="100" placeholder="10">
</div>
</div>
</div>
</div>
</div>

<!-- Social & Author Tab -->
<div id="tab-social" class="tab-content">
<div class="card">
<div class="card-header">Author</div>
<div class="card-body">
<div class="form-row">
<div class="form-group">
<label for="authorName">Name</label>
<input type="text" id="authorName" class="form-control" placeholder="Your Name">
</div>
<div class="form-group">
<label for="authorAvatar">Avatar</label>
<div style="display:flex;gap:.75rem;align-items:flex-start">
<div>
<img id="avatarPreview" src="" alt="" style="width:64px;height:64px;border-radius:50%;object-fit:cover;border:1px solid var(--border);display:none">
</div>
<div style="flex:1">
<input type="text" id="authorAvatar" class="form-control" placeholder="/static/profile.jpg" style="margin-bottom:.5rem">
<div style="display:flex;gap:.5rem;align-items:center">
<label class="btn btn-outline btn-sm" style="cursor:pointer"><input type="file" id="avatarFile" accept="image/*" style="display:none" onchange="uploadAvatar(this)"> Upload Image</label>
<span id="avatarStatus" style="font-size:.75rem;color:var(--muted)"></span>
</div>
</div>
</div>
</div>
</div>
<div class="form-group">
<label for="authorBio">Bio</label>
<textarea id="authorBio" class="form-control" rows="3" placeholder="A brief bio about yourself"></textarea>
</div>
</div>
</div>
<div class="card">
<div class="card-header">Social Links</div>
<div class="card-body">
<div class="form-row">
<div class="form-group">
<label for="socialGithub">GitHub Username</label>
<input type="text" id="socialGithub" class="form-control" placeholder="username">
</div>
<div class="form-group">
<label for="socialTwitter">Twitter Handle</label>
<input type="text" id="socialTwitter" class="form-control" placeholder="username">
</div>
</div>
<div class="form-row">
<div class="form-group">
<label for="socialLinkedin">LinkedIn</label>
<input type="text" id="socialLinkedin" class="form-control" placeholder="username">
</div>
<div class="form-group">
<label for="socialYoutube">YouTube</label>
<input type="text" id="socialYoutube" class="form-control" placeholder="channel">
</div>
</div>
<div class="form-group">
<label for="socialBluesky">Bluesky</label>
<input type="text" id="socialBluesky" class="form-control" placeholder="handle.bsky.social">
</div>
</div>
</div>
</div>

<!-- Footer Tab -->
<div id="tab-footer" class="tab-content">
<div class="card">
<div class="card-header">
Footer Sections
<button class="btn btn-outline btn-sm" onclick="addFooterSection()">+ Add Section</button>
</div>
<div class="card-body">
<div id="footerSections"></div>
<div class="hint">Each section has a title and a list of links shown in the footer.</div>
</div>
</div>
</div>

<!-- Themes Tab -->
<div id="tab-themes" class="tab-content">
<div class="card">
<div class="card-header">Available Themes</div>
<div class="card-body">
<div class="themes-grid" id="themesGrid">
<div style="color:var(--muted)">Loading themes...</div>
</div>
</div>
</div>
</div>

<!-- IndieWeb Tab -->
<div id="tab-indieweb" class="tab-content">
<div class="card">
<div class="card-header">Fediverse & IndieWeb Protocols</div>
<div class="card-body">
<p style="font-size:.85rem;color:var(--muted);margin-bottom:1rem">Enable or disable IndieWeb and Fediverse protocols. Disabling a protocol will return 404 for its endpoints.</p>

<div class="form-group" style="display:flex;align-items:center;gap:.75rem">
<input type="checkbox" id="apEnabled" style="width:auto">
<div>
<label for="apEnabled" style="margin:0;font-weight:600">ActivityPub</label>
<div class="hint" style="margin:0">Fediverse integration — followers, outbox, webfinger. Allows Mastodon/Pleroma users to follow your blog.</div>
</div>
</div>

<div class="form-group" style="display:flex;align-items:center;gap:.75rem">
<input type="checkbox" id="wmEnabled" style="width:auto">
<div>
<label for="wmEnabled" style="margin:0;font-weight:600">Webmention</label>
<div class="hint" style="margin:0">Send and receive webmentions (W3C spec). Enables cross-site replies, likes, and reposts.</div>
</div>
</div>

<div class="form-group" style="display:flex;align-items:center;gap:.75rem">
<input type="checkbox" id="iaEnabled" style="width:auto">
<div>
<label for="iaEnabled" style="margin:0;font-weight:600">IndieAuth</label>
<div class="hint" style="margin:0">OAuth-like authorization for IndieWeb apps. Required for Micropub to work.</div>
</div>
</div>

<div class="form-group" style="display:flex;align-items:center;gap:.75rem">
<input type="checkbox" id="mpEnabled" style="width:auto">
<div>
<label for="mpEnabled" style="margin:0;font-weight:600">Micropub</label>
<div class="hint" style="margin:0">Create, update, and delete posts from any Micropub client (e.g., Quill, Indigenous).</div>
</div>
</div>

<button class="btn btn-primary" onclick="saveIndieWebSettings()">Save IndieWeb Settings</button>
<span id="indiewebSaved" style="display:none;color:var(--primary);font-size:.85rem;margin-left:.5rem">&#10003; Saved</span>
</div>
</div>
</div>

<!-- Security Tab -->
<div id="tab-security" class="tab-content">

<div class="card">
<div class="card-header">Change Password</div>
<div class="card-body">
<div class="form-group">
<label>Current Password</label>
<input type="password" id="currentPassword" class="form-control" style="max-width:320px" autocomplete="current-password">
</div>
<div class="form-group">
<label>New Password</label>
<input type="password" id="newPassword" class="form-control" style="max-width:320px" autocomplete="new-password" minlength="8">
<div class="hint">Minimum 8 characters.</div>
</div>
<div class="form-group">
<label>Confirm New Password</label>
<input type="password" id="confirmPassword" class="form-control" style="max-width:320px" autocomplete="new-password">
</div>
<button class="btn btn-primary" onclick="changePassword()">Update Password</button>
</div>
</div>

<div class="card">
<div class="card-header">Two-Factor Authentication (TOTP)</div>
<div class="card-body">
<div id="totp-status">Loading...</div>

<div id="totp-disabled" style="display:none">
<p style="margin-bottom:1rem;color:var(--muted)">Two-factor authentication adds an extra layer of security to your admin login. When enabled, you will need to enter a 6-digit code from your authenticator app (Google Authenticator, Authy, etc.) in addition to your password.</p>
<button class="btn btn-primary" onclick="setupTOTP()">Enable 2FA</button>
</div>

<div id="totp-setup" style="display:none">
<p style="margin-bottom:.75rem">Scan this QR code with your authenticator app (Google Authenticator, Authy, 1Password, etc.):</p>
<div style="text-align:center;margin-bottom:1rem"><img id="totpQR" style="max-width:256px;border:1px solid var(--border);border-radius:var(--radius)" alt="QR Code"></div>
<details style="margin-bottom:1rem"><summary style="cursor:pointer;font-size:.85rem;color:var(--muted)">Can't scan? Enter manually</summary>
<div style="background:var(--canvas-subtle);border:1px solid var(--border);border-radius:var(--radius);padding:1rem;margin-top:.5rem;font-family:monospace;font-size:1.1rem;letter-spacing:.15em;word-break:break-all;text-align:center" id="totpSecret"></div>
<div class="form-group" style="margin-top:.5rem">
<label>Provisioning URI</label>
<input type="text" id="totpURI" class="form-control" readonly onclick="this.select()">
</div></details>
<div class="form-group">
<label>Verify Code</label>
<input type="text" id="totpVerifyCode" class="form-control" inputmode="numeric" pattern="[0-9]{6}" maxlength="6" placeholder="Enter 6-digit code from your app" style="max-width:220px">
<div class="hint">Enter the code shown in your authenticator to confirm setup.</div>
</div>
<button class="btn btn-primary" onclick="confirmTOTP()">Verify & Enable</button>
<button class="btn btn-outline" onclick="cancelTOTPSetup()" style="margin-left:.5rem">Cancel</button>
</div>

<div id="totp-enabled" style="display:none">
<p style="color:var(--success);font-weight:600;margin-bottom:1rem">&#10003; Two-factor authentication is enabled.</p>
<div class="form-group">
<label>Enter current TOTP code to disable</label>
<input type="text" id="totpDisableCode" class="form-control" inputmode="numeric" pattern="[0-9]{6}" maxlength="6" placeholder="000000" style="max-width:220px">
</div>
<button class="btn btn-danger" onclick="disableTOTP()">Disable 2FA</button>
</div>
</div>
</div>

<div class="card">
<div class="card-header">Passkeys (WebAuthn)
<button class="btn btn-primary btn-sm" onclick="registerPasskey()">+ Add Passkey</button>
</div>
<div class="card-body">
<p style="color:var(--muted);font-size:.85rem;margin-bottom:1rem">Passkeys let you log in using your device's biometrics (fingerprint, face) or security key — no password needed.</p>
<div id="passkeysList"><p style="color:var(--muted)">Loading...</p></div>
</div>
</div>

<div class="card">
<div class="card-header">Audit Log
<button class="btn btn-sm" onclick="clearAuditLog()" title="Clear entries older than 24 hours">Clear Old Entries</button>
</div>
<div class="card-body">
<p style="color:var(--muted);font-size:.85rem;margin-bottom:1rem">Persistent record of admin actions (login, logout, post changes, settings updates). Retained for 14 days. Entries from the last 24 hours cannot be cleared.</p>
<div style="max-height:400px;overflow-y:auto">
<table style="width:100%;font-size:.85rem">
<thead><tr><th>Time</th><th>Event</th><th>Detail</th><th>IP</th></tr></thead>
<tbody id="auditLogTable"><tr><td colspan="4" style="color:var(--muted)">Loading...</td></tr></tbody>
</table>
</div>
</div>
</div>

</div>

<div id="saveActions" style="display:flex;justify-content:flex-end;gap:.75rem;margin-top:1rem;padding-bottom:2rem">
<button class="btn btn-outline" onclick="loadSettings()">Reset Changes</button>
<button class="btn btn-primary" id="saveBtn" onclick="saveSettings()">Save Settings</button>
</div>
</div>

<div class="toast" id="toast"></div>

<script>
let settings = {};
let footerSections = [];

// Tab switching
function switchTab(name){
  document.querySelectorAll('.tab').forEach(t=>t.classList.remove('active'));
  document.querySelectorAll('.tab-content').forEach(t=>t.classList.remove('active'));
  event.target.classList.add('active');
  document.getElementById('tab-'+name).classList.add('active');
  // Show save buttons only on General, Social, Footer tabs
  var sa=document.getElementById('saveActions');
  if(sa) sa.style.display=(name==='general'||name==='social'||name==='footer')?'flex':'none';
}

// Toast notifications
function showToast(msg, type){
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.className = 'toast toast-' + type + ' show';
  setTimeout(()=>t.classList.remove('show'), 3000);
}

// Load settings from API
async function loadSettings(){
  try{
    const [settingsRes, themesRes] = await Promise.all([
      fetch('/admin/api/settings'),
      fetch('/admin/api/themes')
    ]);
    if(settingsRes.status === 401) { window.location.href='/admin/login'; return; }

    settings = await settingsRes.json();
    const themes = await themesRes.json();

    // Populate general fields
    document.getElementById('siteTitle').value = settings.title || '';
    document.getElementById('siteTagline').value = settings.tagline || '';
    document.getElementById('siteBaseURL').value = settings.base_url || '';
    document.getElementById('sitePostsPerPage').value = settings.posts_per_page || 10;

    // Author
    document.getElementById('authorName').value = (settings.author && settings.author.name) || '';
    document.getElementById('authorBio').value = (settings.author && settings.author.bio) || '';
    document.getElementById('authorAvatar').value = (settings.author && settings.author.avatar) || '';
    // Show avatar preview
    const av = (settings.author && settings.author.avatar) || '';
    if(av){const img=document.getElementById('avatarPreview');img.src=av;img.style.display='block';}

    // Social
    const s = settings.social || {};
    document.getElementById('socialGithub').value = s.github || '';
    document.getElementById('socialTwitter').value = s.twitter || '';
    document.getElementById('socialLinkedin').value = s.linkedin || '';
    document.getElementById('socialYoutube').value = s.youtube || '';
    document.getElementById('socialBluesky').value = s.bluesky || '';

    // Footer
    footerSections = (settings.footer && settings.footer.sections) || [];
    renderFooterSections();

    // IndieWeb toggles
    document.getElementById('apEnabled').checked = settings.activitypub_enabled !== false;
    document.getElementById('wmEnabled').checked = settings.webmention_enabled !== false;
    document.getElementById('iaEnabled').checked = settings.indieauth_enabled !== false;
    document.getElementById('mpEnabled').checked = settings.micropub_enabled !== false;

    // Themes
    renderThemes(themes);
  } catch(e) {
    showToast('Failed to load settings', 'error');
  }
}

// Render themes grid
function renderThemes(themes){
  const grid = document.getElementById('themesGrid');
  if(!themes || !themes.length){
    grid.innerHTML = '<p style="color:var(--muted)">No themes found.</p>';
    return;
  }
  grid.innerHTML = themes.map(t => {
    const activeClass = t.active ? ' active' : '';
    return ` + "`" + `<div class="theme-card${activeClass}" data-theme="${esc(t.id)}">
      <div class="theme-preview">
        <iframe src="/admin/api/theme-preview-css?theme=${esc(t.id)}" loading="lazy" style="pointer-events:none"></iframe>
      </div>
      <div class="theme-info">
        <div class="theme-name">${esc(t.name)}</div>
        <div class="theme-desc">${esc(t.description)}</div>
        <div class="theme-meta">
          <span>v${esc(t.version)}</span>
          <span>${esc(t.layout || 'single-column')}</span>
          <span>by ${esc(t.author)}</span>
        </div>
        <div class="theme-actions">
          ${t.active
            ? '<button class="btn btn-outline btn-sm" disabled>Active Theme</button>'
            : '<button class="btn btn-primary btn-sm" onclick="activateTheme(\''+esc(t.id)+'\')">Activate</button>'
          }
          <a href="/admin/api/theme-preview-css?theme=${esc(t.id)}" target="_blank" class="btn btn-outline btn-sm">Preview</a>
        </div>
      </div>
    </div>` + "`" + `;
  }).join('');
}

// Activate a theme
async function activateTheme(themeId){
  // Update the theme in settings and save
  document.getElementById('saveBtn').disabled = true;
  try{
    // Collect current form data + override theme
    const payload = collectSettings();
    payload.theme = themeId;
    const res = await fetch('/admin/api/settings', {
      method: 'PUT',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if(res.ok){
      showToast('Theme activated: ' + themeId, 'success');
      loadSettings(); // Reload to reflect changes
    } else {
      showToast(data.error || 'Failed to activate theme', 'error');
    }
  } catch(e) {
    showToast('Network error', 'error');
  }
  document.getElementById('saveBtn').disabled = false;
}

// Footer section management
function renderFooterSections(){
  const container = document.getElementById('footerSections');
  if(!footerSections.length){
    container.innerHTML = '<p style="color:var(--muted);font-size:.85rem">No footer sections. Click "Add Section" to create one.</p>';
    return;
  }
  container.innerHTML = footerSections.map((sec, si) => {
    const linksHTML = (sec.links || []).map((link, li) => ` + "`" + `
      <div class="footer-link-row">
        <input type="text" class="form-control" value="${esc(link.label||'')}" placeholder="Label" onchange="updateFooterLink(${si},${li},'label',this.value)">
        <input type="text" class="form-control" value="${esc(link.url||'')}" placeholder="URL" onchange="updateFooterLink(${si},${li},'url',this.value)">
        <button class="btn btn-outline btn-sm" onclick="removeFooterLink(${si},${li})" title="Remove">&times;</button>
      </div>` + "`" + `).join('');
    return ` + "`" + `
      <div class="footer-section-item">
        <div class="section-header-row">
          <input type="text" class="form-control" style="max-width:200px" value="${esc(sec.title||'')}" placeholder="Section title" onchange="footerSections[${si}].title=this.value">
          <div style="display:flex;gap:.25rem">
            <button class="btn btn-outline btn-sm" onclick="addFooterLink(${si})">+ Link</button>
            <button class="btn btn-outline btn-sm" onclick="removeFooterSection(${si})" title="Remove section">&times;</button>
          </div>
        </div>
        ${linksHTML}
      </div>` + "`" + `;
  }).join('');
}
function addFooterSection(){
  footerSections.push({title:'', links:[]});
  renderFooterSections();
}
function removeFooterSection(i){
  footerSections.splice(i,1);
  renderFooterSections();
}
function addFooterLink(si){
  if(!footerSections[si].links) footerSections[si].links = [];
  footerSections[si].links.push({label:'', url:''});
  renderFooterSections();
}
function removeFooterLink(si,li){
  footerSections[si].links.splice(li,1);
  renderFooterSections();
}
function updateFooterLink(si,li,field,val){
  footerSections[si].links[li][field] = val;
}

// Collect all form data into a settings object
function collectSettings(){
  return {
    title: document.getElementById('siteTitle').value,
    tagline: document.getElementById('siteTagline').value,
    base_url: document.getElementById('siteBaseURL').value,
    theme: settings.theme || 'classic',
    posts_per_page: parseInt(document.getElementById('sitePostsPerPage').value) || 10,
    activitypub_enabled: document.getElementById('apEnabled').checked,
    webmention_enabled: document.getElementById('wmEnabled').checked,
    indieauth_enabled: document.getElementById('iaEnabled').checked,
    micropub_enabled: document.getElementById('mpEnabled').checked,
    author: {
      name: document.getElementById('authorName').value,
      bio: document.getElementById('authorBio').value,
      avatar: document.getElementById('authorAvatar').value,
    },
    social: {
      github: document.getElementById('socialGithub').value,
      twitter: document.getElementById('socialTwitter').value,
      linkedin: document.getElementById('socialLinkedin').value,
      youtube: document.getElementById('socialYoutube').value,
      bluesky: document.getElementById('socialBluesky').value,
    },
    footer: {
      sections: footerSections
    }
  };
}

// Save settings
async function saveSettings(){
  const btn = document.getElementById('saveBtn');
  btn.disabled = true; btn.textContent = 'Saving...';
  try{
    const payload = collectSettings();
    const res = await fetch('/admin/api/settings', {
      method: 'PUT',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if(res.ok){
      showToast('Settings saved successfully!', 'success');
      settings = payload; // Update local cache
    } else {
      showToast(data.error || 'Failed to save', 'error');
    }
  } catch(e) {
    showToast('Network error', 'error');
  }
  btn.disabled = false; btn.textContent = 'Save Settings';
}

function esc(s){
  if(!s) return '';
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML.replace(/"/g, '&quot;');
}

async function saveIndieWebSettings(){
  try{
    const payload = collectSettings();
    const res = await fetch('/admin/api/settings', {
      method: 'PUT',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify(payload)
    });
    if(res.ok){
      settings = payload;
      const badge = document.getElementById('indiewebSaved');
      badge.style.display='inline';
      setTimeout(()=>badge.style.display='none',3000);
    } else {
      const data = await res.json();
      showToast(data.error || 'Failed to save', 'error');
    }
  }catch(e){showToast('Network error','error');}
}

// --- Password Change ---
async function changePassword(){
  const current = document.getElementById('currentPassword').value;
  const newPw = document.getElementById('newPassword').value;
  const confirm = document.getElementById('confirmPassword').value;
  if(!current || !newPw){showToast('Please fill in all fields','error');return;}
  if(newPw.length < 8){showToast('Password must be at least 8 characters','error');return;}
  if(newPw !== confirm){showToast('Passwords do not match','error');return;}
  try{
    const res = await fetch('/admin/api/password',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({current_password:current,new_password:newPw})});
    const data = await res.json();
    if(res.ok){
      showToast('Password changed successfully!','success');
      document.getElementById('currentPassword').value='';
      document.getElementById('newPassword').value='';
      document.getElementById('confirmPassword').value='';
    } else {
      showToast(data.error||'Failed to change password','error');
    }
  }catch(e){showToast('Network error','error');}
}

// --- Avatar Upload ---
async function uploadAvatar(input){
  if(!input.files||!input.files[0])return;
  const file=input.files[0];
  if(file.size>5*1024*1024){showToast('File too large (max 5MB)','error');return;}
  const status=document.getElementById('avatarStatus');
  status.textContent='Uploading...';
  try{
    const fd=new FormData();fd.append('avatar',file);
    const res=await fetch('/admin/api/avatar',{method:'POST',body:fd});
    const data=await res.json();
    if(res.ok){
      document.getElementById('authorAvatar').value=data.url;
      const img=document.getElementById('avatarPreview');img.src=data.url;img.style.display='block';
      status.textContent='Uploaded!';status.style.color='var(--success)';
      setTimeout(()=>{status.textContent='';status.style.color='var(--muted)';},3000);
    }else{status.textContent=data.error||'Upload failed';status.style.color='var(--danger)';}
  }catch(e){status.textContent='Network error';status.style.color='var(--danger)';}
}

// --- TOTP 2FA ---
let pendingTOTPSecret = '';

async function loadTOTPStatus(){
  try{
    const res = await fetch('/admin/api/totp/status');
    if(!res.ok) return;
    const data = await res.json();
    document.getElementById('totp-status').style.display='none';
    if(data.enabled){
      document.getElementById('totp-enabled').style.display='block';
      document.getElementById('totp-disabled').style.display='none';
    } else {
      document.getElementById('totp-disabled').style.display='block';
      document.getElementById('totp-enabled').style.display='none';
    }
  }catch(e){}
}

async function setupTOTP(){
  try{
    const res = await fetch('/admin/api/totp/setup',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({})});
    const data = await res.json();
    if(!res.ok){showToast(data.error||'Failed','error');return;}
    pendingTOTPSecret = data.secret;
    document.getElementById('totpSecret').textContent = data.secret;
    document.getElementById('totpURI').value = data.uri;
    if(data.qr) document.getElementById('totpQR').src = data.qr;
    document.getElementById('totp-disabled').style.display='none';
    document.getElementById('totp-setup').style.display='block';
  }catch(e){showToast('Network error','error');}
}

function cancelTOTPSetup(){
  pendingTOTPSecret='';
  document.getElementById('totp-setup').style.display='none';
  document.getElementById('totp-disabled').style.display='block';
}

async function confirmTOTP(){
  const code = document.getElementById('totpVerifyCode').value.trim();
  if(code.length!==6){showToast('Enter a 6-digit code','error');return;}
  try{
    const res = await fetch('/admin/api/totp/confirm',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({secret:pendingTOTPSecret,code:code})});
    const data = await res.json();
    if(res.ok){
      showToast('2FA enabled successfully!','success');
      document.getElementById('totp-setup').style.display='none';
      document.getElementById('totp-enabled').style.display='block';
      pendingTOTPSecret='';
    } else {
      showToast(data.error||'Verification failed','error');
    }
  }catch(e){showToast('Network error','error');}
}

async function disableTOTP(){
  const code = document.getElementById('totpDisableCode').value.trim();
  if(code.length!==6){showToast('Enter your current TOTP code','error');return;}
  if(!confirm('Disable two-factor authentication?'))return;
  try{
    const res = await fetch('/admin/api/totp/disable',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({code:code})});
    const data = await res.json();
    if(res.ok){
      showToast('2FA disabled','success');
      document.getElementById('totp-enabled').style.display='none';
      document.getElementById('totp-disabled').style.display='block';
    } else {
      showToast(data.error||'Failed to disable','error');
    }
  }catch(e){showToast('Network error','error');}
}

// Load on page ready
loadSettings();
loadTOTPStatus();
loadPasskeys();
loadAuditLog();

// --- Passkeys ---
async function loadPasskeys(){
  try{
    const res = await fetch('/admin/api/passkeys');
    if(!res.ok) return;
    const keys = await res.json();
    const el = document.getElementById('passkeysList');
    if(!keys||!keys.length){
      el.innerHTML='<p style="color:var(--muted);font-size:.85rem">No passkeys registered yet.</p>';
      return;
    }
    el.innerHTML='<table style="width:100%"><thead><tr><th>Name</th><th>Added</th><th></th></tr></thead><tbody>'+
      keys.map(k=>'<tr><td>'+esc(k.name)+'</td><td>'+esc(k.created_at)+'</td><td><button class="btn btn-danger btn-sm" onclick="deletePasskey('+k.id+')">Delete</button></td></tr>').join('')+
      '</tbody></table>';
  }catch(e){}
}

async function registerPasskey(){
  const name = prompt('Name for this passkey (e.g. "MacBook Touch ID"):','Passkey');
  if(!name) return;
  try{
    // Begin
    const beginRes = await fetch('/admin/api/passkeys/register/begin',{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'});
    if(!beginRes.ok){showToast('Failed to start registration','error');return;}
    const options = await beginRes.json();
    // Convert base64url to ArrayBuffer
    options.publicKey.challenge = b64ToAb(options.publicKey.challenge);
    options.publicKey.user.id = b64ToAb(options.publicKey.user.id);
    if(options.publicKey.excludeCredentials){
      options.publicKey.excludeCredentials = options.publicKey.excludeCredentials.map(c=>({...c, id: b64ToAb(c.id)}));
    }
    // Create credential
    const credential = await navigator.credentials.create(options);
    // Build response
    const attestation = {
      id: credential.id,
      rawId: abToB64(credential.rawId),
      type: credential.type,
      response: {
        attestationObject: abToB64(credential.response.attestationObject),
        clientDataJSON: abToB64(credential.response.clientDataJSON),
      }
    };
    if(credential.response.getTransports) attestation.response.transports = credential.response.getTransports();
    // Finish
    const finishRes = await fetch('/admin/api/passkeys/register/finish?name='+encodeURIComponent(name),{
      method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(attestation)
    });
    if(finishRes.ok){
      showToast('Passkey registered!','success');
      loadPasskeys();
    } else {
      const d = await finishRes.json();
      showToast(d.error||'Registration failed','error');
    }
  }catch(e){
    if(e.name==='NotAllowedError') showToast('Registration cancelled','error');
    else showToast('Error: '+e.message,'error');
  }
}

async function deletePasskey(id){
  if(!confirm('Delete this passkey?'))return;
  const r = await fetch('/admin/api/passkeys/'+id,{method:'DELETE'});
  if(r.ok){showToast('Passkey deleted','success');loadPasskeys();}
  else showToast('Failed to delete','error');
}

// Base64url helpers
function b64ToAb(b64){
  const s = b64.replace(/-/g,'+').replace(/_/g,'/');
  const bin = atob(s);
  const buf = new Uint8Array(bin.length);
  for(let i=0;i<bin.length;i++) buf[i]=bin.charCodeAt(i);
  return buf.buffer;
}
function abToB64(ab){
  const bytes = new Uint8Array(ab);
  let s = '';
  for(let i=0;i<bytes.length;i++) s+=String.fromCharCode(bytes[i]);
  return btoa(s).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');
}

// --- Audit Log ---
async function loadAuditLog(){
  try{
    const res = await fetch('/admin/api/audit-log?limit=200');
    if(!res.ok) return;
    const entries = await res.json();
    const tb = document.getElementById('auditLogTable');
    if(!entries||!entries.length){
      tb.innerHTML='<tr><td colspan="4" style="color:var(--muted)">No audit events recorded yet.</td></tr>';
      return;
    }
    tb.innerHTML=entries.map(e=>{
      const d=new Date(e.created_at);
      const ts=d.toLocaleDateString()+' '+d.toLocaleTimeString();
      const evtColor=e.event.startsWith('login.failed')||e.event.startsWith('login.lockout')?'#cf222e':e.event.startsWith('login.success')?'#1a7f37':'var(--text)';
      return '<tr><td style="white-space:nowrap;font-size:.8rem">'+esc(ts)+'</td><td><span style="color:'+evtColor+';font-weight:500">'+esc(e.event)+'</span></td><td>'+esc(e.detail)+'</td><td style="font-family:monospace;font-size:.8rem">'+esc(e.ip)+'</td></tr>';
    }).join('');
  }catch(e){}
}

async function clearAuditLog(){
  if(!confirm('Clear audit entries older than 24 hours? Recent entries will be retained.'))return;
  try{
    const res = await fetch('/admin/api/audit-log/clear',{method:'POST'});
    if(res.ok){
      const d = await res.json();
      showToast('Cleared '+d.deleted+' old entries','success');
      loadAuditLog();
    } else {
      showToast('Failed to clear audit log','error');
    }
  }catch(e){showToast('Network error','error');}
}
</script>
</body>
</html>`

// adminNewsletterHTML is the newsletter editor and scheduler page.
