// Package handler — admin HTML templates embedded as Go string constants.
// These are served directly from the binary without external files.
package handler

// adminLoginHTML is the login page for the admin dashboard.
const adminLoginHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>Admin Login — Skriva</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#15171a;display:flex;justify-content:center;align-items:center;min-height:100vh;color:#e5e5e5}
.login-wrapper{text-align:center;width:100%;max-width:380px;padding:0 1rem}
.login-logo{font-size:2.5rem;font-weight:800;color:#fff;margin-bottom:.5rem;letter-spacing:-0.03em}
.login-logo span{color:#6366f1}
.login-sub{color:#7c8088;margin-bottom:2.5rem;font-size:.9rem}
.login-card{background:#1e2028;padding:2rem 2rem 1.5rem;border-radius:12px;box-shadow:0 8px 32px rgba(0,0,0,.3)}
label{display:block;font-weight:600;margin-bottom:.4rem;font-size:.8rem;color:#a1a1aa;text-align:left}
input[type=password],input[type=text]{width:100%;padding:.75rem 1rem;border:1px solid #2e3039;border-radius:8px;font-size:1rem;margin-bottom:1rem;background:#15171a;color:#fff;transition:border-color .15s}
input:focus{outline:none;border-color:#6366f1;box-shadow:0 0 0 3px rgba(99,102,241,.25)}
button[type=submit]{width:100%;padding:.8rem;background:linear-gradient(135deg,#6366f1,#8b5cf6);color:#fff;border:none;border-radius:8px;font-size:1rem;cursor:pointer;font-weight:600;transition:opacity .15s}
button[type=submit]:hover{opacity:.9}
.error{color:#ef4444;font-size:.85rem;margin-bottom:.75rem;display:none;text-align:center;background:rgba(239,68,68,.1);padding:.5rem;border-radius:6px}
.totp-field{display:none}
.passkey-btn{width:100%;padding:.8rem;background:#2e3039;color:#e5e5e5;border:1px solid #3f4150;border-radius:8px;font-size:1rem;cursor:pointer;font-weight:600;transition:all .15s;margin-top:.75rem}
.passkey-btn:hover{background:#3f4150;border-color:#6366f1}
.divider{display:flex;align-items:center;gap:.75rem;margin:.75rem 0;color:#4b5563;font-size:.8rem}
.divider::before,.divider::after{content:'';flex:1;height:1px;background:#2e3039}
</style>
</head>
<body>
<div class="login-wrapper">
<div class="login-logo">S<span>kriva</span></div>
<p class="login-sub">Sign in to your admin dashboard</p>
<div class="login-card">
<div class="error" id="error"></div>
<form id="loginForm">
<label for="password">Password</label>
<input type="password" id="password" name="password" required autofocus autocomplete="current-password" placeholder="Enter your password">
<div class="totp-field" id="totpField">
<label for="totp">Authenticator Code</label>
<input type="text" id="totp" name="totp" inputmode="numeric" pattern="[0-9]{6}" maxlength="6" placeholder="000000" autocomplete="one-time-code">
</div>
<button type="submit">Sign In</button>
</form>
<div id="passkeyLogin" style="display:none">
<div class="divider">or</div>
<button type="button" onclick="loginWithPasskey()" class="passkey-btn">🔑 Sign in with Passkey</button>
</div>
</div>
</div>
<script>
// Check if passkeys are available
(async function(){
  try{
    const r=await fetch('/admin/api/passkeys/login/begin',{method:'POST',headers:{'Content-Type':'application/json'}});
    if(r.ok) document.getElementById('passkeyLogin').style.display='block';
  }catch(e){}
})();

async function loginWithPasskey(){
  const errEl=document.getElementById('error');
  errEl.style.display='none';
  try{
    const beginRes=await fetch('/admin/api/passkeys/login/begin',{method:'POST',headers:{'Content-Type':'application/json'}});
    if(!beginRes.ok){errEl.textContent='No passkeys configured';errEl.style.display='block';return;}
    const options=await beginRes.json();
    options.publicKey.challenge=b64ToAb(options.publicKey.challenge);
    if(options.publicKey.allowCredentials){
      options.publicKey.allowCredentials=options.publicKey.allowCredentials.map(c=>({...c,id:b64ToAb(c.id)}));
    }
    const assertion=await navigator.credentials.get(options);
    const body={
      id:assertion.id,
      rawId:abToB64(assertion.rawId),
      type:assertion.type,
      response:{
        authenticatorData:abToB64(assertion.response.authenticatorData),
        clientDataJSON:abToB64(assertion.response.clientDataJSON),
        signature:abToB64(assertion.response.signature),
        userHandle:assertion.response.userHandle?abToB64(assertion.response.userHandle):''
      }
    };
    const finishRes=await fetch('/admin/api/passkeys/login/finish',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    if(finishRes.ok){window.location.href='/admin/';}
    else{const d=await finishRes.json();errEl.textContent=d.error||'Authentication failed';errEl.style.display='block';}
  }catch(e){
    if(e.name==='NotAllowedError') return;
    errEl.textContent='Passkey error: '+e.message;errEl.style.display='block';
  }
}
function b64ToAb(b){const s=b.replace(/-/g,'+').replace(/_/g,'/');const d=atob(s);const a=new Uint8Array(d.length);for(let i=0;i<d.length;i++)a[i]=d.charCodeAt(i);return a.buffer;}
function abToB64(a){const b=new Uint8Array(a);let s='';for(let i=0;i<b.length;i++)s+=String.fromCharCode(b[i]);return btoa(s).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');}

let totpRequired = false;
document.getElementById('loginForm').addEventListener('submit', async function(e){
  e.preventDefault();
  const errEl = document.getElementById('error');
  errEl.style.display='none';
  try{
    const payload = {password: document.getElementById('password').value};
    if(totpRequired) payload.totp_code = document.getElementById('totp').value;
    const res = await fetch('/admin/api/login', {
      method:'POST',
      headers:{'Content-Type':'application/json'},
      body:JSON.stringify(payload)
    });
    const data = await res.json();
    if(res.ok){window.location.href='/admin/';}
    else if(data.totp_required){
      totpRequired = true;
      document.getElementById('totpField').style.display='block';
      document.getElementById('totp').focus();
      errEl.textContent='Enter the 6-digit code from your authenticator app.';
      errEl.style.display='block';
      errEl.style.color='#333';
    }
    else{errEl.textContent=data.error||'Invalid credentials';errEl.style.display='block';errEl.style.color='#d1242f';}
  }catch(err){errEl.textContent='Network error';errEl.style.display='block';}
});
</script>
</body>
</html>`

// adminDashboardHTML is the admin dashboard SPA.

