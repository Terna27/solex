'use strict';

const main = document.querySelector('#main');
const nav = document.querySelector('#main-nav');
const headerUserStatus = document.querySelector('#header-user-status');
const userPill = document.querySelector('#user-pill');
const headerLogoutBtn = document.querySelector('#header-logout-btn');

let programme = null;
let currentUser = null;
let currentProfile = null;
let currentApp = null;
let adminToken = sessionStorage.getItem('solex_admin_token') || '';
let applications = [];
let selectedAppID = '';
let viewEpoch = 0;

const escapeHTML = v => String(v ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const money = k => '₦' + (Number(k || 0) / 100).toLocaleString('en-NG');
const dateFmt = d => {
  if (!d) return 'TBD';
  const parts = d.split('-');
  if (parts.length === 3) {
    const dt = new Date(Date.UTC(Number(parts[0]), Number(parts[1]) - 1, Number(parts[2])));
    return dt.toLocaleDateString('en-NG', { day: 'numeric', month: 'short', year: 'numeric' });
  }
  return d;
};

const badge = s => `<span class="badge ${escapeHTML(s || 'draft')}">${escapeHTML(String(s || 'draft').replace(/_/g, ' '))}</span>`;
const heading = (eyebrow, title, sub) => `<div class="heading"><div><div class="eyebrow">${eyebrow}</div><h1>${title}</h1><p class="muted">${sub}</p></div></div>`;

async function api(path, { method = 'GET', body, token = '' } = {}) {
  const headers = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (token) headers.Authorization = 'Bearer ' + token;

  const baseUrl = window.SOLEX_API_URL || '/api/v1';
  let r;
  try {
    r = await fetch(baseUrl + path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'same-origin',
      signal: AbortSignal.timeout(20000)
    });
  } catch (err) {
    throw Error('Cannot reach the Solex server. Check your connection or that the Go backend is running.');
  }

  const data = await r.json().catch(() => ({}));
  if (!r.ok) {
    if (r.status === 401 && path === '/auth/me') return null;
    throw Error(data.error || 'The request could not be completed.');
  }
  return data;
}

function toast(text) {
  const box = document.querySelector('#toast');
  box.textContent = text;
  box.style.display = 'block';
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => { box.style.display = 'none'; }, 5000);
}

async function action(form, fn) {
  const buttons = form.querySelectorAll('button');
  buttons.forEach(b => { b.disabled = true; });
  const box = form.querySelector('[data-error]');
  if (box) box.innerHTML = '';
  try {
    await fn();
  } catch (e) {
    if (box) box.innerHTML = `<div class="error" role="alert">${escapeHTML(e.message)}</div>`;
    else toast(e.message);
  } finally {
    buttons.forEach(b => { b.disabled = false; });
  }
}

function pwField(name, label = 'Password', minlength = 8) {
  return `
    <label class="field">${label}
      <div class="pw-wrap">
        <input name="${name}" type="password" required minlength="${minlength}" autocomplete="current-password">
        <button type="button" class="pw-toggle" aria-label="Toggle password visibility">Show</button>
      </div>
    </label>
  `;
}

function initPasswordToggles(container = document) {
  container.querySelectorAll('.pw-toggle').forEach(btn => {
    btn.onclick = () => {
      const input = btn.previousElementSibling;
      if (!input) return;
      const isPw = input.type === 'password';
      input.type = isPw ? 'text' : 'password';
      btn.textContent = isPw ? 'Hide' : 'Show';
    };
  });
}

function renderNav() {
  const page = location.hash.slice(1) || 'home';
  const isAuth = !!currentUser;

  let items = [
    { id: 'home', label: '◈ Overview' },
    { id: 'about', label: 'ℹ About Programme' },
    { id: 'eligibility', label: '✓ Eligibility' }
  ];

  if (isAuth) {
    items.push({ id: 'dashboard', label: '◎ Applicant Dashboard' });
    items.push({ id: 'apply', label: '＋ Apply for Grant' });
  } else {
    items.push({ id: 'register', label: '＋ Create Account' });
    items.push({ id: 'login', label: '→ Sign In' });
  }

  items.push({ id: 'contact', label: '✉ Contact Desk' });
  items.push({ id: 'admin', label: '▦ Admin Workspace' });

  nav.innerHTML = items.map(item => `
    <a href="#${item.id}" data-page="${item.id}" class="${page === item.id ? 'active' : ''}">
      <span>${escapeHTML(item.label)}</span>
    </a>
  `).join('');

  if (isAuth) {
    headerUserStatus.innerHTML = `Welcome, <strong>${escapeHTML(currentUser.full_name)}</strong>`;
    userPill.textContent = currentUser.email;
    userPill.classList.remove('hidden');
    headerLogoutBtn.classList.remove('hidden');
  } else {
    headerUserStatus.textContent = 'Small businesses. More possibilities.';
    userPill.classList.add('hidden');
    headerLogoutBtn.classList.add('hidden');
  }
}

headerLogoutBtn.onclick = async () => {
  try {
    await api('/auth/logout', { method: 'POST' });
  } catch (_) {}
  currentUser = null;
  currentProfile = null;
  currentApp = null;
  toast('Signed out successfully.');
  location.hash = '#home';
  route();
};

// ----------------- PAGES -----------------

function home() {
  const isAuth = !!currentUser;
  main.innerHTML = heading('A NEXT STEP, TOGETHER', 'Room for your Nigerian business to grow.', 'Practical one-time grant support for everyday business owners and formal enterprises.') + `
    <section class="hero">
      <div class="hero-copy">
        <span class="eyebrow">SOLEX BUSINESS GRANT PROGRAMME</span>
        <h1>Small business.<br><em>Bigger possibilities.</em></h1>
        <p>One-time non-repayable grant funding to help Nigerian entrepreneurs restock goods, upgrade tools, and expand local commerce.</p>
        <div class="actions">
          ${isAuth
            ? `<a class="btn light" href="#dashboard">Go to Dashboard <span>↗</span></a><a class="btn secondary" href="#apply">Apply for Grant</a>`
            : `<a class="btn light" href="#register">Create Account <span>↗</span></a><a class="btn secondary" href="#login">Login</a>`
          }
        </div>
      </div>
      <div class="hero-art" aria-label="Solex grant categories">
        <div class="orb"></div>
        <div class="orb two"></div>
        <div class="grant-card">
          <span class="eyebrow">PROGRAMME TIERS</span>
          <div style="font:700 24px Georgia,serif;color:#244832;margin:18px 0 6px">Dual Grant Tiers</div>
          <span>Registered & Unregistered Businesses</span>
          <span class="stamp">
            COMMENCING: 10 DEC 2026<br>
            Disbursement: Dec 2026<br>
            One-time direct business support
          </span>
        </div>
      </div>
    </section>

    <div class="section-head">
      <h2>Two Grant Categories</h2>
      <span class="hint">Commencing 10 December 2026 · Selection requires review</span>
    </div>

    <div class="category-grid">
      <div class="category-card">
        <span class="cat-badge">CATEGORY A</span>
        <div>
          <span class="eyebrow">UNREGISTERED BUSINESSES</span>
          <h3 style="font-size:22px;margin:10px 0 6px">Unregistered Business Support</h3>
          <p class="muted">One-time business support grant</p>
          <hr class="divider">
          <p class="compact">
            ✓ Designed for everyday market traders, artisans, POS operators, and informal micro-businesses.<br>
            ✓ CAC registration certificate: <strong>Not required</strong>.<br>
            ✓ Paid directly as a one-time grant disbursement upon administrative approval.<br>
            ✓ Valid 11-digit NIN and supported Nigerian bank account required.
          </p>
        </div>
        <div class="actions" style="margin-top:20px">
          ${isAuth
            ? `<a class="btn secondary" href="#apply">Apply for Grant →</a>`
            : `<a class="btn secondary" href="#register">Create Account →</a>`
          }
        </div>
      </div>

      <div class="category-card featured">
        <span class="cat-badge">CATEGORY B</span>
        <div>
          <span class="eyebrow">REGISTERED BUSINESSES</span>
          <h3 style="font-size:22px;margin:10px 0 6px">Registered Business Support</h3>
          <p class="muted">One-time enterprise support grant</p>
          <hr class="divider">
          <p class="compact">
            ✓ Designed for registered Nigerian micro and small enterprises with Corporate Affairs Commission (CAC) status.<br>
            ✓ Valid CAC registration number: <strong>Required</strong>.<br>
            ✓ CAC registration certificate upload: <strong>Required</strong>.<br>
            ✓ Admin verification of business credentials is mandatory before grant award.
          </p>
        </div>
        <div class="actions" style="margin-top:20px">
          ${isAuth
            ? `<a class="btn" href="#apply">Apply for Grant →</a>`
            : `<a class="btn" href="#register">Create Account →</a>`
          }
        </div>
      </div>
    </div>

    <div class="stats">
      <div class="stat">
        <small>Programme Commencement</small>
        <strong>10 Dec 2026</strong>
        <p>Applications currently being received for review</p>
      </div>
      <div class="stat">
        <small>Target Beneficiaries</small>
        <strong>Nationwide</strong>
        <p>All 36 states and the Federal Capital Territory</p>
      </div>
      <div class="stat">
        <small>Application Processing Fee</small>
        <strong>₦1,000</strong>
        <p>Administrative review fee per submitted application</p>
      </div>
    </div>

    <div class="section-head">
      <h2>How the Application Journey Works</h2>
      <span class="hint">Strictly merit and eligibility-based</span>
    </div>

    <div class="steps">
      <article class="card">
        <div class="step-number">01</div>
        <h3>Create Your Account</h3>
        <p>Register with your name, phone, and secure password. Complete your personal profile with your 11-digit NIN.</p>
      </article>
      <article class="card">
        <div class="step-number">02</div>
        <h3>Select Business Category</h3>
        <p>Choose Unregistered or Registered business category. Upload required identity and CAC certificates.</p>
      </article>
      <article class="card">
        <div class="step-number">03</div>
        <h3>Review & Assessment</h3>
        <p>Review disclosures and process the ₦1,000 processing fee. Track application status via your dashboard.</p>
      </article>
    </div>

    <div class="tags">
      <span class="tag">Market traders</span>
      <span class="tag">POS businesses</span>
      <span class="tag">Retail shopkeepers</span>
      <span class="tag">Food and catering</span>
      <span class="tag">Agribusiness</span>
      <span class="tag">Registered SMEs</span>
    </div>

    <p class="hint" style="margin-top:22px">
      Disclaimer: Submission of an application and payment of the processing fee does not constitute grant approval. Selection is governed by administrative verification, document authenticity, and programme capacity.
    </p>
  `;
}

function about() {
  main.innerHTML = heading('PROGRAMME OVERVIEW', 'About the Solex Grant Initiative', 'Strengthening local commercial resilience and economic growth across Nigeria.') + `
    <div class="two-col">
      <section class="card">
        <h2>Mission and Purpose</h2>
        <p>The Solex Business Grant Programme was instituted to provide direct, non-repayable capital injections to hardworking Nigerian micro and small business owners.</p>
        <p>Recognizing the differing stages of Nigerian business formalization, the programme provides two structured grant tiers:</p>
        <ul>
          <li><strong>Category A (Unregistered Businesses)</strong>: Direct support for informal businesses, market traders, and craftsmen without CAC incorporation.</li>
          <li><strong>Category B (Registered Businesses)</strong>: Scaled funding for formal enterprises with authenticated CAC corporate registrations.</li>
        </ul>
        <div class="notice">
          <strong>Important Schedule:</strong> The programme commences on <strong>10 December 2026</strong>. Disbursements are scheduled for December 2026 following thorough verification.
        </div>
        <h3>Governance & Fair Selection</h3>
        <p>All applications undergo independent administrative review. Grant allocation is conducted regardless of party affiliation, religious background, or campaign participation. Every applicant must satisfy NIN validation and account ownership checks.</p>
      </section>

      <aside class="card">
        <span class="eyebrow">KEY PROGRAMME METRICS</span>
        <hr class="divider">
        <p class="compact">
          <strong>Commencement Date:</strong> 10 December 2026<br>
          <strong>Disbursement Window:</strong> December 2026<br>
          <strong>Tier A:</strong> Unregistered Businesses (CAC not required)<br>
          <strong>Tier B:</strong> Registered Businesses (CAC required)<br>
          <strong>Processing Fee:</strong> ₦1,000 per application<br>
          <strong>Coverage:</strong> All 36 States + FCT
        </p>
        <div class="actions">
          <a class="btn light" href="#eligibility">Check Eligibility Criteria →</a>
        </div>
      </aside>
    </div>
  `;
}

function eligibility() {
  main.innerHTML = heading('QUALIFICATION REQUIREMENTS', 'Programme Eligibility Guidelines', 'Carefully review the criteria before submitting your application.') + `
    <div class="two-col">
      <section class="card">
        <h2>General Eligibility Criteria</h2>
        <p>To be considered for either grant category, an applicant must satisfy all of the following baseline requirements:</p>
        <div class="detail-list">
          <div><dt>Age Requirement</dt><dd>Must be at least 18 years of age at the time of application.</dd></div>
          <div><dt>Nationality & Residence</dt><dd>Nigerian citizen residing and operating a business in Nigeria.</dd></div>
          <div><dt>National Identification</dt><dd>Valid 11-digit National Identification Number (NIN).</dd></div>
          <div><dt>Bank Account</dt><dd>Active Nigerian deposit-money bank account in the applicant's name (wallets excluded).</dd></div>
          <div><dt>Identity Verification</dt><dd>Clear passport photograph and valid government ID or proof of address.</dd></div>
          <div><dt>Processing Fee</dt><dd>Completed ₦1,000 application verification fee.</dd></div>
        </div>

        <hr class="divider">
        <h2>Category-Specific Criteria</h2>
        <h3>Category A: Unregistered Businesses</h3>
        <p>Open to informal traders, artisans, food vendors, and service providers. CAC incorporation is not required. Applicants must provide details of their business location and operating history.</p>

        <h3>Category B: Registered Businesses</h3>
        <p>Open to formally incorporated sole proprietorships, partnerships, or limited liability companies. Applicants must submit their valid CAC Registration Number and upload a legible CAC certificate.</p>
      </section>

      <aside class="card">
        <span class="eyebrow">WHAT YOU NEED TO APPLY</span>
        <hr class="divider">
        <p class="compact">
          1. 11-digit NIN<br>
          2. Portrait photograph (JPEG/PNG, max 5MB)<br>
          3. Government ID OR Utility bill<br>
          4. Commercial bank account details<br>
          5. CAC Certificate (Tier B only)<br>
          6. ₦1,000 processing fee
        </p>
        <div class="notice">
          <strong>Notice:</strong> Document submission does not constitute independent verification. Administrative checks remain mandatory.
        </div>
        <a class="btn" href="#register">Start Account Registration →</a>
      </aside>
    </div>
  `;
}

function contact() {
  main.innerHTML = heading('GET IN TOUCH', 'Programme Administration & Support', 'Questions about requirements, account access, or application status?') + `
    <div class="two-col">
      <section class="card">
        <h2>Solex Support Desk</h2>
        <p>Our dedicated support team is available to assist applicants with inquiries regarding business categories, document formats, and application tracking.</p>
        <div class="detail-list">
          <div><dt>Programme Operator</dt><dd>Solex Business Grant Programme Administration Committee</dd></div>
          <div><dt>Official Email</dt><dd><a href="mailto:support@solex.ng">support@solex.ng</a></dd></div>
          <div><dt>Physical Desk</dt><dd>Plot 12 Broad Street, Lagos Island, Lagos State</dd></div>
          <div><dt>Operating Hours</dt><dd>Monday – Friday: 8:00 AM – 5:00 PM (WAT)</dd></div>
        </div>
        <div class="notice warn">
          <strong>Security Warning:</strong> Solex officials will never ask for your bank account PIN, ATM card details, banking OTP, or account password.
        </div>
      </section>

      <aside class="card">
        <span class="eyebrow">SEND A DIRECT INQUIRY</span>
        <form id="contact-form" style="margin-top:14px">
          <label class="field">Your Name<input name="name" required placeholder="Full name"></label>
          <label class="field">Your Email<input name="email" type="email" required placeholder="you@example.ng"></label>
          <label class="field">Message<textarea name="message" required minlength="10" placeholder="How can we assist you?"></textarea></label>
          <div data-error></div>
          <button class="btn" style="margin-top:12px">Send Message</button>
        </form>
      </aside>
    </div>
  `;
  const form = document.querySelector('#contact-form');
  if (form) {
    form.onsubmit = e => {
      e.preventDefault();
      action(form, async () => {
        toast('Thank you. Your message has been recorded. Our team will contact you via email.');
        form.reset();
      });
    };
  }
}

function register() {
  main.innerHTML = heading('GET STARTED', 'Create an Applicant Account', 'Register to apply for the Solex Business Grant Programme.') + `
    <div class="login">
      <form id="register-form" class="card">
        <span class="eyebrow">APPLICANT REGISTRATION</span>
        <h2 style="margin-top:10px">Create your account</h2>
        <p class="muted compact">All fields are required. Enter your legal personal information.</p>

        <label class="field">Full Legal Name
          <input name="full_name" required minlength="3" maxlength="150" placeholder="e.g. Halima Danjuma" autocomplete="name">
        </label>

        <label class="field">Email Address
          <input name="email" type="email" required placeholder="you@example.ng" autocomplete="email">
        </label>

        <label class="field">Nigerian Phone Number
          <input name="phone" type="tel" required placeholder="+2348012345678 or 08012345678" autocomplete="tel">
          <span class="hint">Nigerian international (+234) or standard 11-digit format.</span>
        </label>

        ${pwField('password', 'Choose Password (min 8 characters)', 8)}
        ${pwField('confirm_password', 'Confirm Password', 8)}

        <label class="check">
          <input type="checkbox" name="terms" required>
          <span>I confirm the information provided is accurate and agree to Solex platform terms.</span>
        </label>

        <div data-error></div>
        <button class="btn" style="width:100%;margin-top:10px" type="submit">Create Account →</button>

        <p class="hint" style="text-align:center;margin-top:20px">
          Already have an account? <a href="#login" style="color:var(--green);font-weight:600">Sign in here</a>.
        </p>
      </form>
    </div>
  `;
  initPasswordToggles();

  const form = document.querySelector('#register-form');
  form.onsubmit = e => {
    e.preventDefault();
    action(form, async () => {
      const fd = new FormData(form);
      const body = {
        full_name: fd.get('full_name').trim(),
        email: fd.get('email').trim(),
        phone: fd.get('phone').trim(),
        password: fd.get('password'),
        confirm_password: fd.get('confirm_password')
      };
      const res = await api('/auth/register', { method: 'POST', body });
      currentUser = res.user;
      toast('Account created! Welcome to Solex.');
      location.hash = '#dashboard';
      route();
    });
  };
}

function login() {
  main.innerHTML = heading('ACCOUNT ACCESS', 'Sign In to Your Workspace', 'Manage your profile, grant application, and review status.') + `
    <div class="login">
      <form id="login-form" class="card">
        <span class="eyebrow">APPLICANT PORTAL</span>
        <h2 style="margin-top:10px">Sign in to your account</h2>
        <p class="muted compact">Enter your registered email and password.</p>

        <label class="field">Email Address
          <input name="email" type="email" required placeholder="you@example.ng" autocomplete="email">
        </label>

        ${pwField('password', 'Password')}

        <div data-error></div>
        <button class="btn" style="width:100%;margin-top:14px" type="submit">Sign In →</button>

        <p class="hint" style="text-align:center;margin-top:20px">
          Do not have an account yet? <a href="#register" style="color:var(--green);font-weight:600">Register here</a>.
        </p>
      </form>
    </div>
  `;
  initPasswordToggles();

  const form = document.querySelector('#login-form');
  form.onsubmit = e => {
    e.preventDefault();
    action(form, async () => {
      const fd = new FormData(form);
      const body = {
        email: fd.get('email').trim(),
        password: fd.get('password')
      };
      const res = await api('/auth/login', { method: 'POST', body });
      currentUser = res.user;
      toast('Welcome back, ' + currentUser.full_name + '!');
      location.hash = '#dashboard';
      route();
    });
  };
}

async function dashboard() {
  if (!currentUser) {
    location.hash = '#login';
    return;
  }

  main.innerHTML = '<div class="empty">Loading your dashboard…</div>';

  try {
    currentProfile = await api('/profile');
    currentApp = await api('/applications/current').catch(() => null);
  } catch (err) {
    main.innerHTML = `<div class="error">${escapeHTML(err.message)}</div>`;
    return;
  }

  const p = currentProfile || {};
  const hasProfile = p.nin && p.nin.length === 11 && p.business_name;
  const hasApp = currentApp && currentApp.id;

  let appCard = '';
  if (hasApp) {
    const a = currentApp;
    const catLabel = a.category === 'registered' ? 'Registered Business' : 'Unregistered Business';
    const docs = a.documents || [];

    appCard = `
      <section class="card" style="margin-top:24px">
        <div class="heading">
          <div>
            <span class="eyebrow">APPLICATION REFERENCE</span>
            <h2>${escapeHTML(a.reference_code || a.id)}</h2>
            <p class="muted">${catLabel} · Potential Grant: <strong>${money(a.grant_amount_kobo)}</strong></p>
          </div>
          ${badge(a.status)}
        </div>

        <div class="detail-list">
          <div><dt>Business Category</dt><dd>${catLabel}</dd></div>
          <div><dt>Potential Grant Amount</dt><dd><strong>${money(a.grant_amount_kobo)}</strong> (One-time)</dd></div>
          <div><dt>Application Status</dt><dd>${badge(a.status)}</dd></div>
          <div><dt>Processing Fee Status</dt><dd>${a.status === 'awaiting_payment' ? '<span class="badge awaiting_payment">Awaiting ₦1,000 Fee</span>' : '<span class="badge payment_confirmed">Verified ₦1,000</span>'}</dd></div>
          <div><dt>Scheduled Disbursement</dt><dd>${a.disbursement_date ? dateFmt(a.disbursement_date) : 'December 2026'}</dd></div>
          <div><dt>Documents Uploaded</dt><dd>${docs.length} document(s)</dd></div>
        </div>

        ${a.reason ? `<div class="notice warn"><strong>Review Feedback:</strong> ${escapeHTML(a.reason)}</div>` : ''}
        ${a.status === 'awaiting_payment' ? `
          <div class="notice">
            Your documents are uploaded. Please complete the ₦1,000 registration fee to finalize submission.
          </div>
          <a class="btn prominent-apply-btn" href="#apply">Proceed to Fee Payment →</a>
        ` : a.status === 'draft' ? `
          <a class="btn prominent-apply-btn" href="#apply">Apply for Grant →</a>
        ` : `
          <div class="notice">
            Your application is safely received. The review committee will assess your documents and eligibility.
          </div>
        `}

        <hr class="divider">
        <h3>Uploaded Documents</h3>
        <div id="dash-docs">
          ${docs.length ? docs.map(d => `
            <div class="payment">
              <div>
                <strong>${escapeHTML(String(d.kind).replace(/_/g, ' '))}</strong>
                <small>${Math.ceil(d.size_bytes / 1024)} KB · Uploaded ${dateFmt(d.uploaded_at.slice(0, 10))}</small>
              </div>
              <button class="btn secondary sm" data-dash-doc="${escapeHTML(d.id)}">Preview Image</button>
            </div>
          `).join('') : '<p class="muted compact">No documents attached yet.</p>'}
        </div>
      </section>
    `;
  } else {
    appCard = `
      <section class="card" style="margin-top:24px">
        <span class="eyebrow">GRANT APPLICATION</span>
        <h2>No Active Application</h2>
        <p class="muted">You have not submitted a grant application yet. Complete your profile and apply for business support.</p>
        <div class="actions">
          <a class="btn prominent-apply-btn" href="#apply">Apply for Grant →</a>
        </div>
      </section>
    `;
  }

  main.innerHTML = heading('APPLICANT WORKSPACE', 'Welcome, ' + escapeHTML(currentUser.full_name), 'Manage your profile, grant application, and supporting documents.') + `
    <div class="dashboard-hero-action card" style="display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:16px;background:var(--accent-warm);border:1px solid #dcd4b8;margin:20px 0 24px;">
      <div>
        <span class="eyebrow" style="color:var(--green)">GRANT OPPORTUNITY</span>
        <h3 style="margin:4px 0 2px">Solex Business Grant Programme</h3>
        <p class="muted compact" style="margin:0">
          ${hasApp
            ? (['submitted', 'under_review', 'approved', 'rejected', 'disbursed'].includes(currentApp.status)
                ? 'Your application reference is ' + escapeHTML(currentApp.reference_code || currentApp.id) + ' (' + escapeHTML(String(currentApp.status).replace(/_/g, ' ')) + ').'
                : (currentApp.status === 'awaiting_payment' ? 'Awaiting registration fee payment. Complete fee to finalize submission.' : 'You have a draft application in progress.'))
            : 'Dual funding tiers for registered and unregistered businesses. Commencing 10 December 2026.'
          }
        </p>
      </div>
      <div>
        <a class="btn prominent-apply-btn" href="#apply">${hasApp && currentApp.status !== 'draft' ? 'View Application Status →' : 'Apply for Grant →'}</a>
      </div>
    </div>

    <div class="stats">
      <div class="stat">
        <small>Profile Status</small>
        <strong>${hasProfile ? 'Complete' : 'Incomplete'}</strong>
        <p>${hasProfile ? 'NIN and bank verified' : 'NIN & address required'}</p>
      </div>
      <div class="stat">
        <small>Grant Tier</small>
        <strong>${hasApp ? money(currentApp.grant_amount_kobo) : (p.business_category === 'registered' ? '₦500,000' : (p.business_category === 'unregistered' ? '₦250,000' : 'Select on Apply'))}</strong>
        <p>${hasApp ? (currentApp.category === 'registered' ? 'Registered Enterprise' : 'Unregistered Business') : (p.business_category === 'registered' ? 'Registered Enterprise' : (p.business_category === 'unregistered' ? 'Unregistered Business' : 'Category pending'))}</p>
      </div>
      <div class="stat">
        <small>Application Stage</small>
        <strong>${hasApp ? escapeHTML(String(currentApp.status).replace(/_/g, ' ')) : 'Not Started'}</strong>
        <p>${hasApp ? currentApp.reference_code || 'In progress' : 'Ready to apply'}</p>
      </div>
    </div>

    ${!hasProfile ? `
      <div class="notice warn">
        <strong>Profile Action Required:</strong> Complete your personal details, National Identification Number (NIN), and bank information before applying.
        <div style="margin-top:10px">
          <a class="btn sm light" href="#apply">Complete Profile Now →</a>
        </div>
      </div>
    ` : ''}

    ${appCard}
  `;

  // Attach document preview handlers in dashboard
  document.querySelectorAll('[data-dash-doc]').forEach(b => {
    b.onclick = () => previewDocument(currentApp.id, b.dataset.dashDoc, false);
  });
}

async function previewDocument(appId, docId, isAdmin) {
  try {
    const prefix = isAdmin ? '/admin' : '';
    const token = isAdmin ? adminToken : '';
    const res = await fetch(`/api/v1${prefix}/applications/${encodeURIComponent(appId)}/documents/${encodeURIComponent(docId)}`, {
      headers: token ? { Authorization: 'Bearer ' + token } : {},
      credentials: 'same-origin'
    });
    if (!res.ok) throw Error('Could not open document preview.');
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);

    const dialog = document.createElement('dialog');
    dialog.className = 'document-dialog';
    dialog.innerHTML = `
      <div class="heading">
        <h3>Private Document Preview</h3>
        <button class="btn secondary sm" type="button">Close</button>
      </div>
      <img src="${url}" alt="Supporting Document">
    `;
    document.body.append(dialog);
    dialog.querySelector('button').onclick = () => dialog.close();
    dialog.addEventListener('close', () => {
      URL.revokeObjectURL(url);
      dialog.remove();
    }, { once: true });
    dialog.showModal();
  } catch (err) {
    toast(err.message);
  }
}

// ----------------- MULTI-STEP APPLY FLOW -----------------

async function apply() {
  if (!currentUser) {
    location.hash = '#login';
    return;
  }

  main.innerHTML = '<div class="empty">Loading application workflow…</div>';

  try {
    currentProfile = await api('/profile');
    currentApp = await api('/applications/current').catch(() => null);
  } catch (err) {
    main.innerHTML = `<div class="error">${escapeHTML(err.message)}</div>`;
    return;
  }

  // Determine current step
  // Step 1: Profile & NIN
  // Step 2: Business details & CAC
  // Step 3: Document uploads
  // Step 4: Fee payment
  // Step 5: Submitted confirmation
  if (currentApp && ['submitted', 'under_review', 'approved', 'rejected', 'disbursed'].includes(currentApp.status)) {
    renderSubmittedView(currentApp);
    return;
  }

  if (currentApp && currentApp.status === 'awaiting_payment') {
    renderPaymentStep(currentApp);
    return;
  }

  if (currentApp && currentApp.status === 'payment_confirmed') {
    renderReviewSubmitStep(currentApp);
    return;
  }

  // Start by selecting business category
  renderCategorySelection();
}

function renderCategorySelection(preselectedCategory) {
  const p = currentProfile || {};
  let selected = preselectedCategory || currentApp?.category || p.business_category || null;

  main.innerHTML = heading('GRANT APPLICATION', 'Select Your Business Category', 'Choose whether your business is formally registered with CAC or operates informally.') + `
    <div style="max-width:900px;margin:0 auto">
      <div class="steps-indicator">
        <div class="step-dot active" title="Step 1: Category Selection"></div>
        <div class="step-dot" title="Step 2: Profile & Business Form"></div>
        <div class="step-dot" title="Step 3: Document Uploads"></div>
        <div class="step-dot" title="Step 4: Fee & Review"></div>
      </div>

      <div class="card" style="margin-top:20px">
        <span class="eyebrow">STEP 1 OF 3</span>
        <h2>Select Your Business Category</h2>
        <p class="muted">Please indicate whether your business has valid registration with the Corporate Affairs Commission (CAC). Your category determines documentation requirements and grant amount.</p>

        <div class="category-select-grid">
          <!-- Option A: Registered Business (CAC registered) -->
          <div class="cat-select-card ${selected === 'registered' ? 'selected' : ''}" id="cat-card-registered" data-cat="registered">
            <div>
              <div class="cat-select-header">
                <div>
                  <span class="cat-badge">OPTION A</span>
                  <h3 style="margin:8px 0 4px">Registered Business</h3>
                  <small class="muted">CAC registered enterprise or company</small>
                </div>
                <input type="radio" name="business_category_choice" value="registered" class="cat-select-radio" ${selected === 'registered' ? 'checked' : ''} aria-label="Option A: Registered Business (CAC registered)">
              </div>

              <p class="compact" style="margin:12px 0">
                For formally incorporated business names, sole proprietorships, partnerships, or limited liability enterprises with the Corporate Affairs Commission.
              </p>

              <ul class="compact muted" style="padding-left:18px;line-height:1.6">
                <li>CAC Registration Number: <strong>Required</strong></li>
                <li>CAC Registration Certificate: <strong>Required upload</strong></li>
                <li>Administrative verification of CAC records is mandatory</li>
              </ul>
            </div>

            <div class="cat-amount-reveal ${selected === 'registered' ? '' : 'hidden'}" id="reveal-registered">
              <small class="muted" style="text-transform:uppercase;font-weight:700;letter-spacing:0.5px">Grant Allocation</small>
              <div class="amount-figure">₦500,000</div>
              <p class="compact" style="color:#244832;font-weight:600;margin:0">
                One-time grant, subject to eligibility verification and approval.
              </p>
            </div>
          </div>

          <!-- Option B: Unregistered Business -->
          <div class="cat-select-card ${selected === 'unregistered' ? 'selected' : ''}" id="cat-card-unregistered" data-cat="unregistered">
            <div>
              <div class="cat-select-header">
                <div>
                  <span class="cat-badge">OPTION B</span>
                  <h3 style="margin:8px 0 4px">Unregistered Business</h3>
                  <small class="muted">Everyday informal commerce</small>
                </div>
                <input type="radio" name="business_category_choice" value="unregistered" class="cat-select-radio" ${selected === 'unregistered' ? 'checked' : ''} aria-label="Option B: Unregistered Business">
              </div>

              <p class="compact" style="margin:12px 0">
                For everyday market traders, artisans, food vendors, POS operators, and informal small businesses operating without CAC incorporation.
              </p>

              <ul class="compact muted" style="padding-left:18px;line-height:1.6">
                <li>CAC Registration Number: <strong>Not required</strong></li>
                <li>CAC Certificate Upload: <strong>Not required</strong></li>
                <li>Valid 11-digit NIN and Nigerian bank account required</li>
              </ul>
            </div>

            <div class="cat-amount-reveal ${selected === 'unregistered' ? '' : 'hidden'}" id="reveal-unregistered">
              <small class="muted" style="text-transform:uppercase;font-weight:700;letter-spacing:0.5px">Grant Allocation</small>
              <div class="amount-figure">₦250,000</div>
              <p class="compact" style="color:#244832;font-weight:600;margin:0">
                One-time grant, subject to eligibility verification and approval.
              </p>
            </div>
          </div>
        </div>

        <div id="selection-summary-banner" class="notice ${selected ? '' : 'hidden'}" style="margin:20px 0">
          <strong id="banner-cat-title">${selected === 'registered' ? 'Selected: Option A — Registered Business (₦500,000 Grant)' : (selected === 'unregistered' ? 'Selected: Option B — Unregistered Business (₦250,000 Grant)' : '')}</strong>
          <p class="compact" style="margin:4px 0 0" id="banner-cat-desc">
            ${selected === 'registered'
              ? 'One-time grant, subject to eligibility verification and approval. You will need to provide your CAC registration number and upload your CAC certificate in the next step.'
              : (selected === 'unregistered' ? 'One-time grant, subject to eligibility verification and approval. CAC documents are not required.' : '')}
          </p>
        </div>

        <div style="display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:12px;margin-top:24px">
          <a class="btn secondary" href="#dashboard">← Back to Dashboard</a>
          <button id="btn-proceed-form" class="btn prominent-apply-btn" ${selected ? '' : 'disabled'}>
            ${selected === 'registered' ? 'Proceed with Registered Business Application (₦500,000) →' : (selected === 'unregistered' ? 'Proceed with Unregistered Business Application (₦250,000) →' : 'Select a Category to Continue →')}
          </button>
        </div>
      </div>
    </div>
  `;

  const cardReg = document.querySelector('#cat-card-registered');
  const cardUnreg = document.querySelector('#cat-card-unregistered');
  const radioReg = cardReg.querySelector('input[type="radio"]');
  const radioUnreg = cardUnreg.querySelector('input[type="radio"]');
  const revealReg = document.querySelector('#reveal-registered');
  const revealUnreg = document.querySelector('#reveal-unregistered');
  const banner = document.querySelector('#selection-summary-banner');
  const bannerTitle = document.querySelector('#banner-cat-title');
  const bannerDesc = document.querySelector('#banner-cat-desc');
  const proceedBtn = document.querySelector('#btn-proceed-form');

  function updateCategory(cat) {
    selected = cat;
    cardReg.classList.toggle('selected', cat === 'registered');
    cardUnreg.classList.toggle('selected', cat === 'unregistered');
    radioReg.checked = (cat === 'registered');
    radioUnreg.checked = (cat === 'unregistered');
    revealReg.classList.toggle('hidden', cat !== 'registered');
    revealUnreg.classList.toggle('hidden', cat !== 'unregistered');
    banner.classList.remove('hidden');

    if (cat === 'registered') {
      bannerTitle.textContent = 'Selected: Option A — Registered Business (₦500,000 Grant)';
      bannerDesc.textContent = 'One-time grant, subject to eligibility verification and approval. CAC registration number and CAC certificate upload are required.';
      proceedBtn.textContent = 'Proceed with Registered Business Application (₦500,000) →';
      proceedBtn.disabled = false;
    } else if (cat === 'unregistered') {
      bannerTitle.textContent = 'Selected: Option B — Unregistered Business (₦250,000 Grant)';
      bannerDesc.textContent = 'One-time grant, subject to eligibility verification and approval. CAC documents are not required.';
      proceedBtn.textContent = 'Proceed with Unregistered Business Application (₦250,000) →';
      proceedBtn.disabled = false;
    }
  }

  cardReg.onclick = () => updateCategory('registered');
  radioReg.onchange = (e) => { e.stopPropagation(); updateCategory('registered'); };

  cardUnreg.onclick = () => updateCategory('unregistered');
  radioUnreg.onchange = (e) => { e.stopPropagation(); updateCategory('unregistered'); };

  proceedBtn.onclick = () => {
    if (!selected) return;
    renderApplicationForm(selected);
  };
}

function renderApplicationForm(selectedCategory) {
  const p = currentProfile || {};
  const category = selectedCategory || currentApp?.category || p.business_category || 'unregistered';
  const isRegistered = category === 'registered';
  const grantAmountDisplay = isRegistered ? '₦500,000' : '₦250,000';
  const categoryTitle = isRegistered ? 'Registered Business' : 'Unregistered Business';
  const hasPassport = currentApp?.documents?.some(d => d.kind === 'passport_photo');
  const hasSupport = currentApp?.documents?.some(d => ['government_id', 'proof_of_address'].includes(d.kind));
  const hasCacDoc = currentApp?.documents?.some(d => d.kind === 'cac_certificate');

  main.innerHTML = heading('GRANT APPLICATION', 'Apply for ' + categoryTitle + ' Support', 'Complete the structured application form. All sections are verified by the backend.') + `
    <div class="two-col">
      <form id="apply-workflow-form" class="card">
        <div style="display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:10px;margin-bottom:16px">
          <div class="steps-indicator" style="margin:0">
            <div class="step-dot done" title="Step 1: Category Selected"></div>
            <div class="step-dot active" title="Step 2: Profile & Details"></div>
            <div class="step-dot active" title="Step 3: Document Uploads"></div>
          </div>
          <button type="button" id="btn-change-category" class="btn secondary sm">← Change Business Category</button>
        </div>

        <input type="hidden" name="business_category" value="${escapeHTML(category)}">

        <div class="notice" style="margin-bottom:20px;border-left:4px solid var(--green)">
          <strong>Selected Category: ${escapeHTML(categoryTitle)} (${grantAmountDisplay} Grant)</strong>
          <p class="compact muted" style="margin:4px 0 0">
            One-time grant, subject to eligibility verification and approval.
            ${isRegistered
              ? 'Valid CAC registration number and CAC certificate upload are required.'
              : 'CAC registration certificate is not required for unregistered businesses.'}
          </p>
        </div>

        <h2>1. Personal Identity & Bank Details</h2>
        <p class="hint">National Identification Number (NIN) must be exactly 11 digits.</p>

        <div class="form-grid">
          <label class="field">Full Legal Name
            <input name="full_name" required minlength="3" maxlength="150" value="${escapeHTML(p.full_name || currentUser.full_name)}">
          </label>
          <label class="field">Date of Birth (18+ only)
            <input name="date_of_birth" type="date" required value="${escapeHTML(p.date_of_birth || '')}">
          </label>
          <label class="field">National Identification Number (NIN)
            <input name="nin" required pattern="[0-9]{11}" maxlength="11" minlength="11" inputmode="numeric" placeholder="11 digits" value="${escapeHTML(p.nin || '')}">
            <span class="hint">11-digit NIN string format validation.</span>
          </label>
          <label class="field">Phone Number
            <input name="phone" type="tel" required value="${escapeHTML(p.phone || currentUser.phone)}">
          </label>
          <label class="field">Email Address
            <input name="email" type="email" required value="${escapeHTML(p.email || currentUser.email)}">
          </label>
          <label class="field">State of Origin
            <select name="state_of_origin" required>
              <option value="">Select state of origin</option>
              ${(programme?.states || []).map(s => `<option value="${escapeHTML(s)}" ${p.state_of_origin === s ? 'selected' : ''}>${escapeHTML(s)}</option>`).join('')}
            </select>
          </label>
          <label class="field">LGA of Origin
            <input name="lga" required minlength="2" maxlength="100" placeholder="e.g. Makurdi" value="${escapeHTML(p.lga || '')}">
          </label>
          <label class="field full">Full Residential Address
            <textarea name="residential_address" required minlength="10" maxlength="500" placeholder="Street, landmark, town">${escapeHTML(p.residential_address || '')}</textarea>
          </label>

          <label class="field">Bank Name
            <select name="bank_name" required>
              <option value="">Select your bank</option>
              ${(programme?.banks || []).map(b => `<option value="${escapeHTML(b)}" ${p.bank_name === b ? 'selected' : ''}>${escapeHTML(b)}</option>`).join('')}
            </select>
          </label>
          <label class="field">Account Number (10 digits)
            <input name="account_number" required pattern="[0-9]{10}" maxlength="10" minlength="10" inputmode="numeric" placeholder="10-digit NUBAN" value="${escapeHTML(p.account_number || '')}">
          </label>
          <label class="field full">Account-Holder Name
            <input name="account_name" required minlength="3" maxlength="150" placeholder="Name as registered with your bank" value="${escapeHTML(p.account_name || '')}">
          </label>
        </div>

        <div class="notice">
          <strong>Bank ownership check:</strong> Enter an account in your name. We never ask for bank PINs, passwords, BVN, or OTPs.
        </div>

        <hr class="divider">
        <h2>2. Business Information</h2>
        <p class="hint">${isRegistered ? 'Provide registered CAC business details.' : 'Provide operational business details. CAC incorporation is not required.'}</p>

        <div class="form-grid">
          ${isRegistered ? `
            <div class="field full">
              <div class="form-grid" style="background:#f9faf6;padding:16px;border-radius:10px;border:1px solid var(--border)">
                <label class="field">CAC Registration Number
                  <input name="cac_number" required minlength="3" maxlength="50" placeholder="e.g. RC-123456 or BN-123456" value="${escapeHTML(p.cac_number || '')}">
                  <span class="hint">Required for registered businesses.</span>
                </label>
                <label class="field">Registered Business Name
                  <input name="registered_business_name" required minlength="2" maxlength="150" placeholder="Name as recorded by CAC" value="${escapeHTML(p.registered_business_name || '')}">
                  <span class="hint">Must match your CAC registration.</span>
                </label>
              </div>
            </div>
          ` : `
            <div class="field full">
              <div class="notice" style="margin:0">
                <strong>Unregistered Business (Category A):</strong> CAC registration number and certificate are <em>not required</em>.
              </div>
            </div>
          `}

          <label class="field">Business / Trading Name
            <input name="business_name" required minlength="2" maxlength="150" placeholder="Trading or shop name" value="${escapeHTML(p.business_name || '')}">
          </label>
          <label class="field">Business Type / Industry
            <select name="business_type" required>
              <option value="">Select industry</option>
              ${['Market trading', 'Retail shop', 'POS business', 'Food & catering', 'Transport & logistics', 'Agriculture & agro-processing', 'Manufacturing & crafts', 'Technology & services', 'Other small enterprise'].map(t => `<option value="${t}" ${p.business_type === t ? 'selected' : ''}>${t}</option>`).join('')}
            </select>
          </label>

          <label class="field">State of Operation
            <select name="state_of_operation" required>
              <option value="">Select operational state</option>
              ${(programme?.states || []).map(s => `<option value="${escapeHTML(s)}" ${p.state_of_operation === s ? 'selected' : ''}>${escapeHTML(s)}</option>`).join('')}
            </select>
          </label>
          <label class="field">LGA of Operation
            <input name="lga_of_operation" required minlength="2" maxlength="100" placeholder="e.g. Ikeja" value="${escapeHTML(p.lga_of_operation || '')}">
          </label>

          <label class="field full">Operating Location / Business Address
            <textarea name="business_address" required minlength="5" maxlength="500" placeholder="Shop, market stall, or usual operating location">${escapeHTML(p.business_address || '')}</textarea>
          </label>

          <label class="field">Operating History
            <select name="operating_history" required>
              <option value="">Select operating duration</option>
              ${['Less than 1 year', '1 - 2 years', '3 - 5 years', 'More than 5 years'].map(h => `<option value="${h}" ${p.operating_history === h ? 'selected' : ''}>${h}</option>`).join('')}
            </select>
          </label>

          <label class="field full">Purpose of the Grant
            <textarea name="grant_purpose" required minlength="5" maxlength="1000" placeholder="How will this grant support your business growth?">${escapeHTML(p.grant_purpose || '')}</textarea>
          </label>

          <label class="field full">Intended Use of Funds
            <textarea name="intended_use" required minlength="5" maxlength="1000" placeholder="Break down how the funds will be deployed (e.g. inventory restock, equipment purchase).">${escapeHTML(p.intended_use || '')}</textarea>
          </label>
        </div>

        <hr class="divider">
        <h2>3. Document Uploads</h2>
        <p class="hint">Clear JPEG or PNG images only. Maximum 5 MB per file.</p>

        <div class="form-grid">
          <label class="field full">Passport Photograph (Portrait Photo)
            <input type="file" name="passport_file" accept="image/jpeg,image/png" ${hasPassport ? '' : 'required'}>
            <span class="hint">${hasPassport ? '✓ Previously uploaded (select new file to replace)' : 'A clear frontal portrait photo of yourself.'}</span>
          </label>

          <label class="field">Supporting Document Type
            <select name="support_kind" id="support-kind">
              <option value="government_id">Government-issued ID (NIN Slip, Voter Card, Driver License)</option>
              <option value="proof_of_address">Proof of Address / Utility Bill</option>
            </select>
          </label>
          <label class="field">Supporting Identity / Address Image
            <input type="file" name="support_file" accept="image/jpeg,image/png" ${hasSupport ? '' : 'required'}>
            <span class="hint">${hasSupport ? '✓ Previously uploaded (select new file to replace)' : 'Upload clear image or scan.'}</span>
          </label>

          ${isRegistered ? `
            <label class="field full">CAC Registration Certificate (Category B Required)
              <input type="file" name="cac_file" id="cac-file-input" accept="image/jpeg,image/png" ${hasCacDoc ? '' : 'required'}>
              <span class="hint">${hasCacDoc ? '✓ Previously uploaded (select new file to replace)' : 'Upload clear photo or scan of your CAC certificate of incorporation / registration.'}</span>
            </label>
          ` : ''}
        </div>

        <label class="check" style="margin-top:20px">
          <input type="checkbox" name="consent" required>
          <span>I declare that all details and documents provided are genuine and agree to administrative review.</span>
        </label>

        <div data-error></div>
        <button class="btn prominent-apply-btn" style="width:100%;margin-top:16px" type="submit">Save & Proceed to Fee Payment →</button>
      </form>

      <aside class="card">
        <span class="eyebrow">APPLICATION SUMMARY</span>
        <h2>${grantAmountDisplay}</h2>
        <p class="muted">${categoryTitle} Support</p>
        <hr class="divider">
        <p class="compact">
          ✓ One-time business grant<br>
          ✓ Personal identity & 11-digit NIN<br>
          ✓ Supported Nigerian commercial bank<br>
          ✓ Passport photograph<br>
          ✓ Identity card or utility bill<br>
          ${isRegistered ? '✓ CAC Registration Certificate (Required)<br>' : '✓ CAC Certificate: <strong>Not required</strong><br>'}
          ✓ Statutory administrative review
        </p>
        <div class="notice">
          <strong>Registration Fee:</strong> ₦1,000 per application. Covers administrative processing and verification.
        </div>
      </aside>
    </div>
  `;

  document.querySelector('#btn-change-category').onclick = () => {
    renderCategorySelection(category);
  };

  const form = document.querySelector('#apply-workflow-form');
  form.onsubmit = e => {
    e.preventDefault();
    action(form, async () => {
      const fd = new FormData(form);

      const prof = {
        full_name: fd.get('full_name').trim(),
        date_of_birth: fd.get('date_of_birth'),
        nin: fd.get('nin').trim(),
        phone: fd.get('phone').trim(),
        email: fd.get('email').trim(),
        state_of_origin: fd.get('state_of_origin'),
        lga: fd.get('lga').trim(),
        residential_address: fd.get('residential_address').trim(),
        business_category: category,
        business_name: fd.get('business_name').trim(),
        business_type: fd.get('business_type'),
        business_description: (fd.get('business_name') || '').trim(),
        business_address: fd.get('business_address').trim(),
        state_of_operation: fd.get('state_of_operation'),
        lga_of_operation: fd.get('lga_of_operation').trim(),
        operating_history: fd.get('operating_history'),
        grant_purpose: fd.get('grant_purpose').trim(),
        intended_use: fd.get('intended_use').trim(),
        bank_name: fd.get('bank_name'),
        account_number: fd.get('account_number').trim(),
        account_name: fd.get('account_name').trim()
      };

      if (isRegistered) {
        prof.cac_number = (fd.get('cac_number') || '').trim();
        prof.registered_business_name = (fd.get('registered_business_name') || '').trim();
        if (!prof.cac_number || !prof.registered_business_name) {
          throw Error('CAC registration number and registered business name are required for registered businesses.');
        }
      }

      // 1. Save profile
      await api('/profile', { method: 'PUT', body: prof });

      // 2. Start application if not already started
      let app = currentApp;
      if (!app || app.status === 'draft') {
        app = await api('/applications/start', { method: 'POST', body: { category } });
        currentApp = app;
      }

      // 3. Upload documents
      const passportFile = fd.get('passport_file');
      if (passportFile && passportFile.size > 0) {
        await uploadFile(app.id, 'passport_photo', passportFile);
      }
      const supportFile = fd.get('support_file');
      if (supportFile && supportFile.size > 0) {
        await uploadFile(app.id, fd.get('support_kind'), supportFile);
      }
      if (isRegistered) {
        const cacFile = fd.get('cac_file');
        if (cacFile && cacFile.size > 0) {
          await uploadFile(app.id, 'cac_certificate', cacFile);
        }
      }

      // 4. Transition to ready for payment
      app = await api('/applications/ready-for-payment', { method: 'POST', body: { application_id: app.id } });
      currentApp = app;

      toast('Application saved! Proceeding to the registration fee stage.');
      renderPaymentStep(currentApp);
    });
  };
}

async function uploadFile(appId, kind, file) {
  if (file.size > 5 * 1024 * 1024) {
    throw Error(`File for ${kind} exceeds the 5 MB limit.`);
  }
  const data = new FormData();
  data.set('kind', kind);
  data.set('file', file);

  const res = await fetch(`/api/v1/applications/${encodeURIComponent(appId)}/documents`, {
    method: 'POST',
    body: data,
    credentials: 'same-origin'
  });
  const json = await res.json().catch(() => ({}));
  if (!res.ok) throw Error(json.error || `Upload failed for ${kind}`);
  return json;
}

async function renderPaymentStep(app) {
  main.innerHTML = '<div class="empty">Preparing payment details…</div>';
  const disclosure = await api('/fee-disclosure');
  const payInit = await api('/payments/init', { method: 'POST', body: { application_id: app.id } });
  const payment = payInit.payment;

  main.innerHTML = heading('APPLICATION FEE', '₦1,000 Administrative Processing Fee', 'Review the required statutory disclosures before processing the verification fee.') + `
    <div class="two-col">
      <section class="card">
        <div class="steps-indicator">
          <div class="step-dot done"></div>
          <div class="step-dot done"></div>
          <div class="step-dot active"></div>
        </div>

        <h2>Administrative Fee Disclosure</h2>
        <div class="notice">
          <strong>Mandatory Disclosure:</strong> ${escapeHTML(disclosure.purpose)}
        </div>

        <div class="detail-list">
          <div><dt>Programme Operator</dt><dd>${escapeHTML(disclosure.operator_identity)}</dd></div>
          <div><dt>Processing Fee</dt><dd><strong>${money(disclosure.fee_amount_kobo)}</strong></dd></div>
          <div><dt>Refund Policy</dt><dd>${escapeHTML(disclosure.refund_policy)}</dd></div>
          <div><dt>Payment Reference</dt><dd><code class="key" style="margin:0">${escapeHTML(payment.reference)}</code></dd></div>
          <div><dt>Support Contact</dt><dd>${escapeHTML(disclosure.contact_information)}</dd></div>
          <div><dt>Approval Guarantee</dt><dd>${escapeHTML(disclosure.no_guarantee_notice)}</dd></div>
        </div>

        <p class="hint">${escapeHTML(disclosure.policy_status)}</p>

        <hr class="divider">
        <h3>Payment Processing</h3>
        <p class="hint">In this development prototype, payment verification is conducted using the secure local sandbox payment simulator below.</p>

        <div style="background:#f4f6ee;border:1px dashed var(--green);border-radius:10px;padding:20px;margin:20px 0">
          <span class="pill" style="margin-bottom:10px"><i></i> LOCAL SANDBOX SIMULATOR</span>
          <p><strong>Simulator Amount:</strong> ₦1,000 (100,000 kobo)</p>
          <p class="compact muted">Simulate payment verification against the backend payment ledger.</p>
          <div class="actions">
            <button id="sim-success-btn" class="btn">Simulate Successful Payment (₦1,000)</button>
            <button id="sim-fail-btn" class="btn secondary">Simulate Payment Failure</button>
          </div>
          <div id="sim-status" style="margin-top:14px"></div>
        </div>
      </section>

      <aside class="card">
        <span class="eyebrow">YOUR APPLICATION</span>
        <h2>${app.category === 'registered' ? '₦500,000' : '₦250,000'}</h2>
        <p class="muted">${app.category === 'registered' ? 'Registered Enterprise' : 'Unregistered Business'}</p>
        <hr class="divider">
        <p class="compact">
          Ref: <strong>${escapeHTML(app.reference_code || app.id)}</strong><br>
          Applicant: ${escapeHTML(currentUser.full_name)}<br>
          Commencement: 10 December 2026<br>
          Status: <strong>Awaiting payment</strong>
        </p>
        <div class="notice">
          We never accept direct transfers to personal accounts or unverified screenshot receipts.
        </div>
      </aside>
    </div>
  `;

  document.querySelector('#sim-success-btn').onclick = async () => {
    const statusBox = document.querySelector('#sim-status');
    statusBox.innerHTML = '<span class="hint">Verifying payment with backend…</span>';
    try {
      const res = await api('/payments/verify', {
        method: 'POST',
        body: {
          reference: payment.reference,
          simulator_action: 'simulate_success'
        }
      });
      currentApp = res.application;
      toast('Payment verified successfully!');
      renderReviewSubmitStep(currentApp);
    } catch (err) {
      statusBox.innerHTML = `<div class="error">${escapeHTML(err.message)}</div>`;
    }
  };

  document.querySelector('#sim-fail-btn').onclick = async () => {
    const statusBox = document.querySelector('#sim-status');
    try {
      await api('/payments/verify', {
        method: 'POST',
        body: {
          reference: payment.reference,
          simulator_action: 'simulate_failure'
        }
      });
      statusBox.innerHTML = '<div class="error">Simulated payment failed. Application remains unpaid.</div>';
    } catch (err) {
      statusBox.innerHTML = `<div class="error">${escapeHTML(err.message)}</div>`;
    }
  };
}

function renderReviewSubmitStep(app) {
  main.innerHTML = heading('FINAL REVIEW', 'Confirm & Submit Application', 'Your ₦1,000 processing fee is verified. Review your application details before submission.') + `
    <div class="two-col">
      <section class="card">
        <div class="steps-indicator">
          <div class="step-dot done"></div>
          <div class="step-dot done"></div>
          <div class="step-dot done"></div>
        </div>

        <div class="notice">
          <strong>Payment Verified:</strong> Your ₦1,000 application fee has been confirmed. You are now ready to submit.
        </div>

        <h2>Application Summary</h2>
        <div class="detail-list">
          <div><dt>Applicant Name</dt><dd>${escapeHTML(app.profile?.full_name || currentUser.full_name)}</dd></div>
          <div><dt>NIN</dt><dd>${escapeHTML(app.profile?.nin || 'Provided')}</dd></div>
          <div><dt>Business Category</dt><dd>${app.category === 'registered' ? 'Registered Business' : 'Unregistered Business'}</dd></div>
          <div><dt>Potential Grant Amount</dt><dd><strong>${money(app.grant_amount_kobo)}</strong> (One-time)</dd></div>
          <div><dt>Bank Name</dt><dd>${escapeHTML(app.profile?.bank_name)}</dd></div>
          <div><dt>Account Number</dt><dd>${escapeHTML(app.profile?.account_number)}</dd></div>
          <div><dt>Account Name</dt><dd>${escapeHTML(app.profile?.account_name)}</dd></div>
          <div><dt>Commencement Date</dt><dd>10 December 2026</dd></div>
        </div>

        <form id="final-submit-form" style="margin-top:20px">
          <label class="check">
            <input type="checkbox" name="confirm_submit" required>
            <span>I confirm this submission and understand that applications are subject to manual administrative review and eligibility checks.</span>
          </label>
          <div data-error></div>
          <button class="btn" style="width:100%;margin-top:12px" type="submit">Submit Application for Consideration →</button>
        </form>
      </section>

      <aside class="card">
        <span class="eyebrow">SUBMISSION CHECKLIST</span>
        <h2>${money(app.grant_amount_kobo)}</h2>
        <p class="muted">One-time allocation</p>
        <hr class="divider">
        <p class="compact">
          ✓ Profile & NIN complete<br>
          ✓ Supported bank details saved<br>
          ✓ Passport & ID images uploaded<br>
          ✓ ₦1,000 fee verified<br>
          ✓ Confirmation email queued
        </p>
      </aside>
    </div>
  `;

  const form = document.querySelector('#final-submit-form');
  form.onsubmit = e => {
    e.preventDefault();
    action(form, async () => {
      const res = await api('/applications/submit', {
        method: 'POST',
        body: { application_id: app.id }
      });
      currentApp = res.application;
      toast('Application submitted successfully!');
      renderSubmittedView(currentApp);
    });
  };
}

function renderSubmittedView(app) {
  main.innerHTML = heading('APPLICATION SUBMITTED', 'Your Application is Under Consideration', 'Your submission has been securely recorded and queued for committee review.') + `
    <div class="card" style="max-width:800px;margin:20px auto">
      <div style="text-align:center;padding:20px 0">
        <div style="width:64px;height:64px;border-radius:50%;background:#eaf0e4;color:var(--green);display:grid;place-items:center;font-size:32px;margin:0 auto 16px">✓</div>
        <h2>Application Received</h2>
        <p class="muted">Application Reference Code</p>
        <code class="key" style="font-size:18px;font-weight:700;display:inline-block">${escapeHTML(app.reference_code || app.id)}</code>
      </div>

      <div class="detail-list">
        <div><dt>Applicant</dt><dd>${escapeHTML(app.profile?.full_name || currentUser?.full_name)}</dd></div>
        <div><dt>Business Category</dt><dd>${app.category === 'registered' ? 'Registered Business' : 'Unregistered Business'}</dd></div>
        <div><dt>Potential Grant Amount</dt><dd><strong>${money(app.grant_amount_kobo)}</strong></dd></div>
        <div><dt>Application Status</dt><dd>${badge(app.status)}</dd></div>
      </div>

      <div class="notice">
        <strong>Confirmation Email Sent:</strong> A confirmation email with your reference code and category details has been dispatched to <strong>${escapeHTML(app.profile?.email || currentUser?.email)}</strong>.
      </div>

      <p class="compact muted" style="margin-top:20px">
        Please note: Successful submission does not mean your application has been approved. All applications are subject to document verification, Nigerian bank check, and eligibility review.
      </p>

      <div class="actions" style="justify-content:center;margin-top:26px">
        <a class="btn" href="#dashboard">Go to Applicant Dashboard</a>
        <a class="btn secondary" href="#home">Back to Homepage</a>
      </div>
    </div>
  `;
}

// ----------------- ADMIN DASHBOARD -----------------

function admin() {
  if (!adminToken) {
    main.innerHTML = heading('PROGRAMME OPERATIONS', 'Administrator Workspace', 'Authorised access to application review, checklist verification, and disbursement.') + `
      <form id="admin-login-form" class="card login">
        <span class="eyebrow">RESTRICTED ACCESS</span>
        <h2 style="margin-top:10px">Sign in as Administrator</h2>
        <p class="muted compact">Enter the configured ADMIN_TOKEN.</p>
        <label class="field">Admin Security Token
          <input name="token" type="password" required autocomplete="off" placeholder="At least 32 characters">
        </label>
        <div data-error></div>
        <button class="btn" style="width:100%;margin-top:14px" type="submit">Access Workspace →</button>
      </form>
    `;
    const form = document.querySelector('#admin-login-form');
    form.onsubmit = e => {
      e.preventDefault();
      action(form, async () => {
        const token = new FormData(form).get('token').trim();
        const metrics = await api('/admin/metrics', { token });
        adminToken = token;
        sessionStorage.setItem('solex_admin_token', token);
        adminDashboard(metrics);
      });
    };
    return;
  }

  loadAdmin();
}

async function loadAdmin() {
  try {
    const metrics = await api('/admin/metrics', { token: adminToken });
    adminDashboard(metrics);
  } catch (err) {
    adminToken = '';
    sessionStorage.removeItem('solex_admin_token');
    admin();
  }
}

async function adminDashboard(metrics) {
  main.innerHTML = '<div class="empty">Loading administrator workspace…</div>';

  let list = [];
  try {
    list = await api('/admin/applications', { token: adminToken });
    applications = list;
  } catch (err) {
    main.innerHTML = `<div class="error">${escapeHTML(err.message)}</div>`;
    return;
  }

  main.innerHTML = heading('PROGRAMME OPERATIONS', 'Administrator Workspace', 'Review submissions, verify CAC records, record decisions, and export data.') + `
    <div class="actions" style="margin:0 0 20px">
      <button id="admin-refresh" class="btn secondary sm">↻ Refresh Data</button>
      <button id="admin-export-csv" class="btn secondary sm">⤓ Export CSV (Privacy Safe)</button>
      <button id="admin-view-emails" class="btn secondary sm">✉ View Email Outbox</button>
      <button id="admin-view-audit" class="btn secondary sm">📋 View Audit Log</button>
      <button id="admin-logout" class="btn danger sm">Sign Out Admin</button>
    </div>

    <div class="stats admin-grid">
      <div class="stat"><small>Registered Users</small><strong>${metrics.total_users}</strong><p>Applicant accounts</p></div>
      <div class="stat"><small>Total Applications</small><strong>${metrics.total_applications}</strong><p>All recorded applications</p></div>
      <div class="stat"><small>Tier A (Unregistered)</small><strong>${metrics.unregistered_business_apps}</strong><p>₦250,000 grant category</p></div>
      <div class="stat"><small>Tier B (Registered)</small><strong>${metrics.registered_business_apps}</strong><p>₦500,000 CAC category</p></div>
      <div class="stat"><small>Awaiting Payment</small><strong>${metrics.applications_awaiting_payment}</strong><p>Pending ₦1,000 fee</p></div>
      <div class="stat"><small>Submitted / Review</small><strong>${metrics.submitted_applications + metrics.applications_under_review}</strong><p>Queued for assessment</p></div>
      <div class="stat"><small>Approved Applications</small><strong>${metrics.approved_applications}</strong><p>Funding commitments</p></div>
      <div class="stat"><small>Disbursed</small><strong>${metrics.disbursed_applications}</strong><p>Paid beneficiaries</p></div>
      <div class="stat"><small>Fees Collected</small><strong>${money(metrics.verified_fees_collected_kobo)}</strong><p>Verified ₦1,000 fees</p></div>
      <div class="stat"><small>Grant Commitments</small><strong>${money(metrics.approved_grant_commitments_kobo)}</strong><p>Approved funding total</p></div>
      <div class="stat"><small>Disbursements Paid</small><strong>${money(metrics.recorded_disbursements_kobo)}</strong><p>Disbursed capital</p></div>
      <div class="stat"><small>Rejected Applications</small><strong>${metrics.rejected_applications}</strong><p>Decisions with reasons</p></div>
    </div>

    <section class="card">
      <div class="section-head" style="margin-top:0">
        <h2>Applications Directory</h2>
        <span class="hint">${applications.length} applications loaded</span>
      </div>

      <div class="toolbar">
        <input id="admin-search" aria-label="Search applicants" placeholder="Search by applicant name, business, state, reference code…">
        <select id="admin-filter-status" aria-label="Filter status">
          <option value="">All Statuses</option>
          <option value="draft">Draft</option>
          <option value="awaiting_payment">Awaiting Payment</option>
          <option value="submitted">Submitted</option>
          <option value="under_review">Under Review</option>
          <option value="approved">Approved</option>
          <option value="rejected">Rejected</option>
          <option value="disbursed">Disbursed</option>
        </select>
        <select id="admin-filter-cat" aria-label="Filter category">
          <option value="">All Categories</option>
          <option value="unregistered">Unregistered (₦250k)</option>
          <option value="registered">Registered (₦500k)</option>
        </select>
      </div>

      <div id="admin-table-container"></div>
    </section>

    <div id="admin-review-drawer"></div>
    <div id="admin-modal-host"></div>
  `;

  document.querySelector('#admin-refresh').onclick = loadAdmin;
  document.querySelector('#admin-logout').onclick = () => {
    adminToken = '';
    sessionStorage.removeItem('solex_admin_token');
    admin();
  };
  document.querySelector('#admin-export-csv').onclick = () => {
    window.location.href = `/api/v1/admin/export?format=csv&token=${encodeURIComponent(adminToken)}`;
  };
  document.querySelector('#admin-view-emails').onclick = showAdminEmails;
  document.querySelector('#admin-view-audit').onclick = showAdminAudit;

  const searchInput = document.querySelector('#admin-search');
  const statusFilter = document.querySelector('#admin-filter-status');
  const catFilter = document.querySelector('#admin-filter-cat');

  searchInput.oninput = renderAdminRows;
  statusFilter.onchange = renderAdminRows;
  catFilter.onchange = renderAdminRows;

  renderAdminRows();
  if (selectedAppID) reviewApp(selectedAppID);
}

function renderAdminRows() {
  const q = (document.querySelector('#admin-search')?.value || '').toLowerCase();
  const status = document.querySelector('#admin-filter-status')?.value || '';
  const cat = document.querySelector('#admin-filter-cat')?.value || '';

  const rows = applications.filter(a => {
    if (status && a.status !== status) return false;
    if (cat && a.category !== cat) return false;
    if (q) {
      const hay = [
        a.profile?.full_name, a.full_name,
        a.profile?.business_name, a.business_name,
        a.profile?.state_of_operation, a.state,
        a.reference_code, a.id
      ].join(' ').toLowerCase();
      if (!hay.includes(q)) return false;
    }
    return true;
  });

  const container = document.querySelector('#admin-table-container');
  if (!container) return;

  container.innerHTML = rows.length ? `
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>APPLICANT / BUSINESS</th>
            <th>TIER & AMOUNT</th>
            <th>LOCATION</th>
            <th>STATUS</th>
            <th>ACTION</th>
          </tr>
        </thead>
        <tbody>
          ${rows.map(a => `
            <tr>
              <td>
                <strong>${escapeHTML(a.profile?.full_name || a.full_name || 'Applicant')}</strong>
                <small>${escapeHTML(a.profile?.business_name || a.business_name || 'Business')}</small>
              </td>
              <td>
                ${escapeHTML(a.category === 'registered' ? 'Tier B (Registered)' : 'Tier A (Unregistered)')}
                <small><strong>${money(a.grant_amount_kobo)}</strong></small>
              </td>
              <td>
                ${escapeHTML(a.profile?.state_of_operation || a.state || 'N/A')}
                <small>${escapeHTML(a.profile?.lga_of_operation || a.lga || '')}</small>
              </td>
              <td>${badge(a.status)}</td>
              <td>
                <button class="btn secondary sm" data-review-id="${escapeHTML(a.id)}">Review Record →</button>
              </td>
            </tr>
          `).join('')}
        </tbody>
      </table>
    </div>
  ` : '<div class="empty">No applications match the current filter.</div>';

  container.querySelectorAll('[data-review-id]').forEach(b => {
    b.onclick = () => {
      reviewApp(b.dataset.reviewId);
      document.querySelector('#admin-review-drawer')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    };
  });
}

function reviewApp(id) {
  selectedAppID = id;
  const a = applications.find(x => x.id === id);
  const host = document.querySelector('#admin-review-drawer');
  if (!a || !host) return;

  const p = a.profile || {};
  const isReg = a.category === 'registered';
  const checks = a.checks || {};
  const docs = a.documents || [];

  host.innerHTML = `
    <section class="card" style="margin-top:24px">
      <div class="heading">
        <div>
          <span class="eyebrow">APPLICATION ASSESSMENT</span>
          <h2>${escapeHTML(p.full_name || a.full_name)}</h2>
          <p class="muted">Ref: <strong>${escapeHTML(a.reference_code || a.id)}</strong> · Category: <strong>${escapeHTML(a.category)}</strong> (${money(a.grant_amount_kobo)})</p>
        </div>
        ${badge(a.status)}
      </div>

      <div class="two-col">
        <div>
          <h3>Applicant & Business Dossier</h3>
          <div class="detail-list">
            <div><dt>Legal Name</dt><dd>${escapeHTML(p.full_name || a.full_name)}</dd></div>
            <div><dt>Date of Birth</dt><dd>${escapeHTML(p.date_of_birth || a.date_of_birth)}</dd></div>
            <div><dt>NIN (Identity Number)</dt><dd><code>${escapeHTML(p.nin || 'Not provided')}</code></dd></div>
            <div><dt>Phone & Email</dt><dd>${escapeHTML(p.phone || a.phone)}<br><small>${escapeHTML(p.email)}</small></dd></div>
            <div><dt>Residential Address</dt><dd>${escapeHTML(p.residential_address || a.residential_address)}</dd></div>
            <div><dt>Location</dt><dd>${escapeHTML(p.state_of_operation || a.state)}, ${escapeHTML(p.lga_of_operation || a.lga)}</dd></div>
            <div><dt>Business Name</dt><dd>${escapeHTML(p.business_name || a.business_name)}</dd></div>
            <div><dt>Business Type</dt><dd>${escapeHTML(p.business_type || a.business_type)}</dd></div>
            <div><dt>Business Address</dt><dd>${escapeHTML(p.business_address || a.business_address)}</dd></div>
            <div><dt>Operating History</dt><dd>${escapeHTML(p.operating_history || 'Not stated')}</dd></div>
            ${isReg ? `
              <div><dt>CAC Registration No.</dt><dd><strong>${escapeHTML(p.cac_number || 'Missing')}</strong></dd></div>
              <div><dt>Registered Name (CAC)</dt><dd>${escapeHTML(p.registered_business_name || 'Missing')}</dd></div>
            ` : ''}
            <div><dt>Bank Details</dt><dd>${escapeHTML(p.bank_name || a.bank_name)}<br>Acc: ${escapeHTML(p.account_number || a.account_number)} (${escapeHTML(p.account_name || a.account_name)})</dd></div>
          </div>

          <p class="compact"><strong>Purpose of Grant:</strong> ${escapeHTML(p.grant_purpose || a.business_description)}</p>
          <p class="compact"><strong>Intended Use of Funds:</strong> ${escapeHTML(p.intended_use || 'Not specified')}</p>

          <hr class="divider">
          <h3>Supporting Documents (${docs.length})</h3>
          <div>
            ${docs.length ? docs.map(d => `
              <div class="payment">
                <div>
                  <strong>${escapeHTML(String(d.kind).replace(/_/g, ' '))}</strong>
                  <small>${Math.ceil(d.size_bytes / 1024)} KB · Uploaded ${dateFmt(d.uploaded_at.slice(0, 10))}</small>
                </div>
                <button type="button" class="btn secondary sm" data-admin-doc="${escapeHTML(d.id)}">Preview Document</button>
              </div>
            `).join('') : '<p class="muted compact">No documents uploaded.</p>'}
          </div>
        </div>

        <div>
          <h3>Verification Checklist</h3>
          <p class="hint">Verify uploaded evidence before ticking checkboxes.</p>
          <form id="admin-checks-form">
            <label class="check"><input type="checkbox" name="phone" ${checks.phone ? 'checked' : ''}><span>Phone number format verified</span></label>
            <label class="check"><input type="checkbox" name="identity_and_age" ${checks.identity_and_age ? 'checked' : ''}><span>NIN & Age 18+ verified</span></label>
            <label class="check"><input type="checkbox" name="address" ${checks.address ? 'checked' : ''}><span>Residential / utility address verified</span></label>
            <label class="check"><input type="checkbox" name="business" ${checks.business ? 'checked' : ''}><span>Business operations established</span></label>
            <label class="check"><input type="checkbox" name="bank_account_ownership" ${checks.bank_account_ownership ? 'checked' : ''}><span>Nigerian bank account valid</span></label>
            ${isReg ? `
              <label class="check"><input type="checkbox" name="cac_registration" ${checks.cac_registration ? 'checked' : ''}><span>CAC certificate & number authenticated</span></label>
            ` : ''}
            <div data-error></div>
            <button class="btn secondary" style="width:100%;margin-top:10px">Save Verification Checklist</button>
          </form>

          <hr class="divider">
          <h3>Administrative Decision</h3>

          ${a.status !== 'approved' && a.status !== 'disbursed' ? `
            <form id="admin-approve-form" style="margin-bottom:20px">
              <label class="field">Scheduled One-Time Disbursement Date
                <input name="disbursement_date" type="date" required value="${a.disbursement_date || '2026-12-10'}">
              </label>
              <div data-error></div>
              <button class="btn" style="width:100%;margin-top:10px">Approve for ${money(a.grant_amount_kobo)}</button>
            </form>

            <form id="admin-reject-form">
              <label class="field">Reason for Rejection
                <textarea name="reason" required minlength="5" maxlength="500" placeholder="State clear administrative reason for rejection..."></textarea>
              </label>
              <div data-error></div>
              <button class="btn danger" style="width:100%;margin-top:10px">Reject Application</button>
            </form>
          ` : a.status === 'approved' ? `
            <div class="notice">
              Application approved for <strong>${money(a.grant_amount_kobo)}</strong> scheduled for <strong>${dateFmt(a.disbursement_date)}</strong>.
            </div>
            <button id="admin-disburse-btn" class="btn" style="width:100%">Record Grant Disbursement →</button>
          ` : `
            <div class="notice" style="background:#e1f0e1;border-color:#2d633a">
              <strong>Grant Disbursed:</strong> One-time grant payment of ${money(a.grant_amount_kobo)} has been recorded.
            </div>
          `}
        </div>
      </div>
    </section>
  `;

  // Document preview button binding
  host.querySelectorAll('[data-admin-doc]').forEach(b => {
    b.onclick = () => previewDocument(a.id, b.dataset.adminDoc, true);
  });

  // Checks form handler
  const checksForm = host.querySelector('#admin-checks-form');
  if (checksForm) {
    checksForm.onsubmit = e => {
      e.preventDefault();
      action(checksForm, async () => {
        const fd = new FormData(checksForm);
        const newChecks = {
          phone: fd.has('phone'),
          identity_and_age: fd.has('identity_and_age'),
          address: fd.has('address'),
          business: fd.has('business'),
          bank_account_ownership: fd.has('bank_account_ownership'),
          cac_registration: isReg ? fd.has('cac_registration') : false
        };
        await api(`/admin/applications/${encodeURIComponent(a.id)}/checks`, {
          method: 'PUT',
          token: adminToken,
          body: newChecks
        });
        toast('Checklist updated.');
        await loadAdmin();
      });
    };
  }

  // Approve form handler
  const approveForm = host.querySelector('#admin-approve-form');
  if (approveForm) {
    approveForm.onsubmit = e => {
      e.preventDefault();
      action(approveForm, async () => {
        const dateVal = new FormData(approveForm).get('disbursement_date');
        await api(`/admin/applications/${encodeURIComponent(a.id)}/approve`, {
          method: 'POST',
          token: adminToken,
          body: { disbursement_date: dateVal }
        });
        toast('Application approved.');
        await loadAdmin();
      });
    };
  }

  // Reject form handler
  const rejectForm = host.querySelector('#admin-reject-form');
  if (rejectForm) {
    rejectForm.onsubmit = e => {
      e.preventDefault();
      action(rejectForm, async () => {
        const reason = new FormData(rejectForm).get('reason').trim();
        await api(`/admin/applications/${encodeURIComponent(a.id)}/reject`, {
          method: 'POST',
          token: adminToken,
          body: { reason }
        });
        toast('Application rejected with reason.');
        await loadAdmin();
      });
    };
  }

  // Disburse button handler
  const disburseBtn = host.querySelector('#admin-disburse-btn');
  if (disburseBtn) {
    disburseBtn.onclick = async () => {
      if (!confirm(`Confirm disbursement of ${money(a.grant_amount_kobo)} to ${p.full_name}?`)) return;
      try {
        await api(`/admin/applications/${encodeURIComponent(a.id)}/disburse`, {
          method: 'POST',
          token: adminToken
        });
        toast('Disbursement recorded.');
        await loadAdmin();
      } catch (err) {
        toast(err.message);
      }
    };
  }
}

async function showAdminEmails() {
  const host = document.querySelector('#admin-modal-host');
  if (!host) return;
  try {
    const emails = await api('/admin/emails', { token: adminToken });
    const dialog = document.createElement('dialog');
    dialog.className = 'document-dialog';
    dialog.innerHTML = `
      <div class="heading">
        <h3>Dispatched & Queued Emails (${emails.length})</h3>
        <button class="btn secondary sm" type="button">Close</button>
      </div>
      <div style="max-height:60vh;overflow-y:auto">
        ${emails.length ? emails.map(m => `
          <div class="card" style="margin-bottom:12px;background:#fcfdfa">
            <p><strong>To:</strong> ${escapeHTML(m.to)} · <strong>Status:</strong> ${escapeHTML(m.status)} · <small>${escapeHTML(m.created_at)}</small></p>
            <p><strong>Subject:</strong> ${escapeHTML(m.subject)}</p>
            <pre style="white-space:pre-wrap;background:#fff;padding:12px;border:1px solid var(--border);border-radius:6px;font-size:12px">${escapeHTML(m.body)}</pre>
          </div>
        `).join('') : '<p class="muted">No emails queued or sent yet.</p>'}
      </div>
    `;
    document.body.append(dialog);
    dialog.querySelector('button').onclick = () => dialog.close();
    dialog.addEventListener('close', () => dialog.remove(), { once: true });
    dialog.showModal();
  } catch (err) {
    toast(err.message);
  }
}

async function showAdminAudit() {
  const host = document.querySelector('#admin-modal-host');
  if (!host) return;
  try {
    const logs = await api('/admin/audit', { token: adminToken });
    const dialog = document.createElement('dialog');
    dialog.className = 'document-dialog';
    dialog.innerHTML = `
      <div class="heading">
        <h3>Administrative Audit Trail (${logs.length})</h3>
        <button class="btn secondary sm" type="button">Close</button>
      </div>
      <div style="max-height:60vh;overflow-y:auto">
        <table>
          <thead>
            <tr>
              <th>TIMESTAMP</th>
              <th>ACTOR</th>
              <th>ACTION</th>
              <th>DETAILS</th>
            </tr>
          </thead>
          <tbody>
            ${logs.slice().reverse().map(l => `
              <tr>
                <td><small>${escapeHTML(new Date(l.at).toLocaleString())}</small></td>
                <td><code>${escapeHTML(l.actor)}</code></td>
                <td><strong>${escapeHTML(l.action)}</strong></td>
                <td><small>${escapeHTML(l.details || l.application_id || '')}</small></td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;
    document.body.append(dialog);
    dialog.querySelector('button').onclick = () => dialog.close();
    dialog.addEventListener('close', () => dialog.remove(), { once: true });
    dialog.showModal();
  } catch (err) {
    toast(err.message);
  }
}

// ----------------- ROUTING -----------------

async function route() {
  const epoch = ++viewEpoch;
  const page = location.hash.slice(1) || 'home';

  // Load programme metadata once
  if (!programme) {
    main.innerHTML = '<div class="empty">Loading Solex Programme details…</div>';
    try {
      programme = await api('/programme');
      const sideCommence = document.querySelector('#side-commence-info');
      if (sideCommence && programme.commencement_date) {
        sideCommence.textContent = `Commences ${dateFmt(programme.commencement_date)}`;
      }
    } catch (e) {
      main.innerHTML = `<div class="error">${escapeHTML(e.message)}</div><button class="btn" id="retry-btn">Retry</button>`;
      document.querySelector('#retry-btn').onclick = route;
      return;
    }
  }

  // Check auth status if not known
  if (currentUser === null) {
    try {
      const meData = await api('/auth/me');
      if (meData && meData.user) {
        currentUser = meData.user;
      }
    } catch (_) {}
  }

  if (epoch !== viewEpoch) return;

  renderNav();

  const pages = {
    home,
    about,
    eligibility,
    contact,
    register,
    login,
    dashboard,
    apply,
    admin
  };

  const handler = pages[page] || home;
  handler();
}

window.addEventListener('hashchange', route);
route();
