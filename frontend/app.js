/* ==========================================================================
   CHORECRAFT STATE MANAGEMENT & EXCLUSIVE LIVE API ENGINE
   ========================================================================== */

// Global session
let currentSession = null; // Contains user_id, username, role, choregroup_id, choregroup_name
let activeTasks = [];

// Resolve API base URL dynamically based on page loading domain
function getApiBase() {
  const API_HOST = window.location.hostname;
  return `http://${API_HOST}:8080/api`;
}

function getRelevantEmoji(title) {
  if (!title) return "📋";
  const lower = title.toLowerCase();
  
  // 🐈 Cat
  if (lower.includes("cat") || lower.includes("חתול")) return "🐈";

  // 🐕 Dog
  if (lower.includes("dog") || lower.includes("כלב")) return "🐕";

  // 🐾 Pet / Feed / Animal
  if (lower.includes("pet") || lower.includes("feed") ||
      lower.includes("להאכיל") || lower.includes("חיה")) return "🐾";
      
  // 🍽️ Dishes / Kitchen / Cooking
  if (lower.includes("dish") || lower.includes("plate") || lower.includes("kitchen") || lower.includes("cook") || 
      lower.includes("כלים") || lower.includes("צלחת") || lower.includes("מטבח") || lower.includes("לבשל") || lower.includes("אוכל")) return "🍽️";
      
  // 🛏️ Bed / Bedroom / Sheets
  if (lower.includes("bed") || lower.includes("room") || lower.includes("sheet") ||
      lower.includes("מיטה") || lower.includes("חדר") || lower.includes("סדין")) return "🛏️";
      
  // 🗑️ Trash / Garbage / Dump
  if (lower.includes("trash") || lower.includes("garbage") || lower.includes("bin") || lower.includes("waste") ||
      lower.includes("זבל") || lower.includes("פח") || lower.includes("לזרוק")) return "🗑️";
      
  // 👕 Clothes / Laundry
  if (lower.includes("clothe") || lower.includes("laundry") || lower.includes("fold") || lower.includes("wash") ||
      lower.includes("בגד") || lower.includes("כביסה") || lower.includes("לקפל")) return "👕";
      
  // 🛒 Shopping / Buy
  if (lower.includes("buy") || lower.includes("shop") || lower.includes("grocery") || lower.includes("market") ||
      lower.includes("לקנות") || lower.includes("קניות") || lower.includes("סופר") || lower.includes("מכולת")) return "🛒";
      
  // 🌱 Garden / Plants
  if (lower.includes("garden") || lower.includes("plant") || lower.includes("water") || lower.includes("grass") ||
      lower.includes("גינה") || lower.includes("עציץ") || lower.includes("להשקות") || lower.includes("דשא") || lower.includes("חצר")) return "🌱";
      
  // 🧸 Toys / Organize
  if (lower.includes("toy") || lower.includes("tidy") || lower.includes("organize") || lower.includes("play") ||
      lower.includes("צעצוע") || lower.includes("לסדר") || lower.includes("לשחק") || lower.includes("ארון")) return "🧸";
      
  // 🚗 Car / Vehicle
  if (lower.includes("car") || lower.includes("drive") ||
      lower.includes("אוטו") || lower.includes("רכב")) return "🚗";
      
  // 🧹 Sweep / Vacuum / Clean / Floor
  if (lower.includes("dust") || lower.includes("sweep") || lower.includes("vacuum") || lower.includes("floor") || lower.includes("mop") || lower.includes("clean") ||
      lower.includes("לטאטא") || lower.includes("לשאוב") || lower.includes("רצפה") || lower.includes("ספונג") || lower.includes("לנקות") || lower.includes("אבק")) return "🧹";
      
  // 📚 Study / Homework
  if (lower.includes("study") || lower.includes("homework") || lower.includes("school") || lower.includes("learn") ||
      lower.includes("שיעור") || lower.includes("ללמוד") || lower.includes("ספר") || lower.includes("מבחן")) return "📚";
      
  // 🧼 Shower / Soap / Teeth / Brush
  if (lower.includes("teeth") || lower.includes("brush") || lower.includes("shower") || lower.includes("bath") ||
      lower.includes("שיניים") || lower.includes("לצחצח") || lower.includes("מקלחת") || lower.includes("סבון")) return "🧼";
      
  return "📋";
}

// Parse chore title to extract emoji prepended in the text (for KISS visual dashboards)
function parseChoreTitle(fullTitle) {
  const emojiRegex = /^(\u00a9|\u00ae|[\u2000-\u3300]|\ud83c[\ud000-\udfff]|\ud83d[\ud000-\udfff]|\ud83e[\ud000-\udfff])/;
  const match = fullTitle ? fullTitle.match(emojiRegex) : null;
  if (match) {
    const emoji = match[0];
    const cleanTitle = fullTitle.replace(emoji, "").trim();
    return { emoji, title: cleanTitle };
  }
  const title = fullTitle || "Unnamed Task";
  const emoji = getRelevantEmoji(title);
  return { emoji, title };
}
// Toggle Assignee select availability based on Task Type
function toggleChoreType(type) {
  const assigneeSelect = document.getElementById("chore-assignee-input");
  const assigneeGroup = document.getElementById("chore-assignee-group");
  const pointsCoin = document.getElementById("chore-points-coin");
  const pointsSelectContainer = document.getElementById("points-select-container");
  
  if (type === "cooperative") {
    if (assigneeGroup) assigneeGroup.style.display = "none";
    if (assigneeSelect) assigneeSelect.required = false;
    if (pointsCoin) pointsCoin.classList.add("coop-theme");
    if (pointsSelectContainer) pointsSelectContainer.classList.add("coop-theme");
  } else {
    if (assigneeGroup) assigneeGroup.style.display = "block";
    if (assigneeSelect) assigneeSelect.required = false;
    if (pointsCoin) pointsCoin.classList.remove("coop-theme");
    if (pointsSelectContainer) pointsSelectContainer.classList.remove("coop-theme");
  }
}

// Custom points select dropdown logic
function initCustomPointsSelect() {
  const container = document.getElementById("points-select-container");
  const trigger = document.getElementById("points-select-trigger");
  const optionsList = document.getElementById("points-select-options");
  const hiddenInput = document.getElementById("chore-points-input");
  
  if (!container || !trigger || !optionsList || !hiddenInput) return;
  
  // Toggle dropdown visibility
  trigger.addEventListener("click", (e) => {
    e.stopPropagation();
    container.classList.toggle("open");
  });
  
  // Close dropdown when clicking outside
  document.addEventListener("click", () => {
    container.classList.remove("open");
  });
  
  // Option selection
  const options = optionsList.querySelectorAll(".custom-select-option");
  options.forEach(opt => {
    opt.addEventListener("click", (e) => {
      e.stopPropagation();
      const val = opt.getAttribute("data-value");
      setPointsSelectValue(val);
      container.classList.remove("open");
    });
  });
}

function setPointsSelectValue(val) {
  const container = document.getElementById("points-select-container");
  const trigger = document.getElementById("points-select-trigger");
  const hiddenInput = document.getElementById("chore-points-input");
  const options = document.querySelectorAll(".custom-select-option");
  if (!container || !trigger || !hiddenInput) return;
  
  hiddenInput.value = val;
  options.forEach(opt => {
    if (opt.getAttribute("data-value") === String(val)) {
      opt.classList.add("selected");
    } else {
      opt.classList.remove("selected");
    }
  });
  
  const isCooperative = container.classList.contains("coop-theme");
  const coinClass = isCooperative ? "points-coin coop-theme" : "points-coin";
  trigger.innerHTML = `
    <span>+${val} <span class="${coinClass}" id="chore-points-coin">🐱</span></span>
    <span class="arrow">▼</span>
  `;
}

/* ==========================================
   APP INITIALIZATION & ROUTING
   ========================================== */
document.addEventListener("DOMContentLoaded", async () => {
  // Check for delegated login token in query params
  const urlParams = new URLSearchParams(window.location.search);
  const delegatedToken = urlParams.get("login_token");
  if (delegatedToken) {
    // Clear query parameter from URL bar to keep it clean
    const nextURL = window.location.origin + window.location.pathname;
    window.history.replaceState({}, document.title, nextURL);
    await handleDelegatedLogin(delegatedToken);
  }

  // Check if session exists in localStorage
  const savedSession = localStorage.getItem("chorecraft_session");
  if (savedSession) {
    currentSession = JSON.parse(savedSession);
    
    // Validate the session is still alive against the server
    // by hitting a protected endpoint. If it fails (401/network error),
    // clear the stale session and return to the landing page.
    try {
      const choregroupID = currentSession.choregroup_id;
      const apiBase = getApiBase();
      const headers = { "Content-Type": "application/json" };
      
      const resp = await fetch(`${apiBase}/choregroups/${choregroupID}/statistics`, { headers, credentials: "include" });
      if (resp.status === 401 || resp.status === 403) {
        // Stale session — user no longer exists in DB
        currentSession = null;
        localStorage.removeItem("chorecraft_session");
        showSection("landing-page");
      } else {
        connectSSE();
        restoreSessionView();
      }
    } catch (err) {
      // Network error — server is down. Clear session and show landing page.
      currentSession = null;
      localStorage.removeItem("chorecraft_session");
      showSection("landing-page");
      showToast("⚠️ Cannot connect to server. Please try again.");
    }
  } else {
    showSection("landing-page");
  }
  
  initCustomPointsSelect();
  updateHeaderAuthBtn();
  updateLeaderboards();
  startTaskTimerTicker();
});


// Switch visible views (tabs)
function showSection(sectionId) {
  document.querySelectorAll(".view-section").forEach(sec => {
    sec.classList.remove("active");
  });
  
  const target = document.getElementById(sectionId);
  if (target) {
    target.classList.add("active");
  }
  
  window.scrollTo(0, 0);
}

// Open and Close Modals
function openAuthModal(initialTab = "login") {
  const modal = document.getElementById("auth-modal");
  if (modal) modal.classList.add("active");
  
  if (initialTab.includes("signup")) {
    toggleAuthForm("signup");
  } else {
    toggleAuthForm("login");
  }
  
  // Focus appropriate input field
  const targetId = initialTab.includes("signup") ? "p-signup-group" : "p-login-user";
  const input = document.getElementById(targetId);
  if (input) {
    setTimeout(() => input.focus(), 150); // allow backdrop slide transition
  }
}

function toggleAuthForm(state) {
  const signupForm = document.getElementById("parent-signup-form");
  const loginForm = document.getElementById("parent-login-form");
  const lookupForm = document.getElementById("kid-pin-lookup-form");
  const avatarForm = document.getElementById("kid-pin-avatar-form");
  const padForm = document.getElementById("kid-pin-pad-form");
  
  const forms = [signupForm, loginForm, lookupForm, avatarForm, padForm];
  forms.forEach(f => { if (f) f.style.display = "none"; });
  
  if (state === "signup" && signupForm) signupForm.style.display = "block";
  if (state === "login" && loginForm) loginForm.style.display = "block";
  if (state === "pin-lookup" && lookupForm) lookupForm.style.display = "block";
  if (state === "pin-avatar" && avatarForm) avatarForm.style.display = "block";
  if (state === "pin-pad" && padForm) padForm.style.display = "block";
}

function closeModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.classList.remove("active");
  }
}

let globalEventSource = null;

function connectSSE() {
  if (globalEventSource) {
    globalEventSource.close();
  }
  if (!currentSession || !currentSession.choregroup_id) return;

  const apiBase = getApiBase();
  const url = `${apiBase}/choregroups/${currentSession.choregroup_id}/events`;
  
  globalEventSource = new EventSource(url, { withCredentials: true });
  
  globalEventSource.onmessage = (event) => {
    if (event.data === "refresh") {
      console.log("SSE refresh event received. Reloading dashboards...");
      if (currentSession.role === "admin") {
        renderParentDashboard();
      } else {
        renderKidDashboard();
      }
      renderRewardsDashboard();
      updateLeaderboards();
      updateHeaderAuthBtn();
    }
  };
  
  globalEventSource.onerror = (err) => {
    console.error("SSE connection error:", err);
    globalEventSource.close();
    // Reconnect automatically after a delay
    setTimeout(connectSSE, 5000);
  };
}

function disconnectSSE() {
  if (globalEventSource) {
    globalEventSource.close();
    globalEventSource = null;
  }
}

function handleLogoClick() {
  if (currentSession) {
    // Disable going back to landing page once logged in
    return;
  }
  showSection("landing-page");
}

/* ==========================================================
   NETWORK CORE ENGINE (Swagger Integration via Fetch API)
   ========================================================== */

async function apiCall(endpoint, method = "GET", body = null) {
  const headers = {
    "Content-Type": "application/json"
  };
  
  const config = {
    method: method,
    headers: headers,
    credentials: "include"
  };
  
  if (body) {
    config.body = JSON.stringify(body);
  }
  
  const apiBase = getApiBase();
  
  try {
    const response = await fetch(`${apiBase}${endpoint}`, config);
    
    if (response.status === 204) {
      return null;
    }
    
    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(errorText || `API Error: ${response.status}`);
    }
    
    return await response.json();
  } catch (err) {
    console.error(`Network call to ${endpoint} failed:`, err);
    showToast(`⚠️ Could not connect to backend at ${apiBase}`);
    throw err;
  }
}

/* ==========================================================
   AUTHENTICATION LOGIC (Live Server Only)
   ========================================================== */

async function handleParentSignUp(e) {
  e.preventDefault();
  
  const groupName = document.getElementById("p-signup-group").value.trim();
  const adminName = document.getElementById("p-signup-admin").value.trim();
  const password = document.getElementById("p-signup-pass").value;
  
  if (!groupName || !adminName || !password) return;
  
  try {
    // POST /signup (SignUpRequest)
    const user = await apiCall("/signup", "POST", {
      choregroup_name: groupName,
      password: password,
      username: adminName
    });
    
    // Store session
    setSession({
      id: user.id,
      username: user.username,
      role: "admin",
      choregroup_id: user.choregroup_id,
      choregroup_name: groupName
    });
    closeModal("auth-modal");
    showToast("Sign up complete! Welcome " + adminName);
  } catch (err) {
    alert("Sign up failed: " + err.message);
  }
}

async function handleLogin(e) {
  e.preventDefault();
  
  const familyName = document.getElementById("p-login-group-name").value.trim();
  const username = document.getElementById("p-login-user").value.trim();
  const password = document.getElementById("p-login-pass").value;
  
  try {
    // POST /login (LoginRequest)
    const res = await apiCall("/login", "POST", {
      choregroup_name: familyName,
      username: username,
      password: password
    });
    
    setSession({
      id: res.user_id,
      username: res.username || username,
      role: res.role,
      choregroup_id: res.choregroup_id,
      choregroup_name: res.role === "admin" ? "My Household" : "Kid Space"
    });
    closeModal("auth-modal");
    showToast(`Welcome back ${res.username || username}!`);
  } catch (err) {
    alert("Invalid credentials on server!");
  }
}

function setSession(userObj) {
  currentSession = {
    user_id: userObj.id,
    username: userObj.username,
    role: userObj.role,
    choregroup_id: userObj.choregroup_id,
    choregroup_name: userObj.choregroup_name
  };
  localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
  connectSSE();
  restoreSessionView();
  updateHeaderAuthBtn();
  updateLeaderboards();
}

function restoreSessionView() {
  if (!currentSession) {
    showSection("landing-page");
    return;
  }
  
  if (currentSession.role === "admin") {
    showSection("parent-view");
  } else {
    showSection("kid-view");
  }
  
  // Restore tab from URL hash (preserves state across refreshes)
  navigateFromHash();
}

function logout() {
  disconnectSSE();
  currentSession = null;
  localStorage.removeItem("chorecraft_session");
  // Clear the hash so login page doesn't try to restore a tab
  history.replaceState(null, '', window.location.pathname);
  showSection("landing-page");
  updateHeaderAuthBtn();
  updateLeaderboards();
  showToast("Logged out successfully.");
}

function updateHeaderAuthBtn() {
  const container = document.getElementById("auth-status-container");
  if (!container) return;
  
  const headerStats = document.getElementById("header-user-stats");
  
  if (currentSession) {
    const isKid = currentSession.role === "user";
    if (isKid) {
      if (headerStats) {
        headerStats.innerHTML = `👋 Hi, <strong>${escapeHTML(currentSession.username)}</strong> (<strong>${currentSession.points || 0}</strong> <span class="points-coin">🐱</span> | <strong>${currentSession.cooperative_points || 0}</strong> <span class="points-coin coop-theme">🐱</span>)`;
        headerStats.style.display = "flex";
      }
      container.innerHTML = `
        <button class="btn btn-outline btn-sm" onclick="logout()">Logout</button>
      `;
    } else {
      if (headerStats) {
        headerStats.style.display = "none";
      }
      container.innerHTML = `
        <span class="user-greeting">👋 Hi, <strong>${escapeHTML(currentSession.username)}</strong> (Parent)</span>
        <button class="btn btn-outline btn-sm" onclick="logout()">Logout</button>
      `;
    }
  } else {
    if (headerStats) {
      headerStats.style.display = "none";
    }
    container.innerHTML = `
      <button class="btn btn-outline btn-sm" onclick="openAuthModal('login')">Log In</button>
      <button class="btn btn-primary btn-sm" onclick="openAuthModal('signup')">Sign Up</button>
    `;
  }
}

/* ==========================================================
   PARENT COMMAND CENTER LOGIC (List / Add / Approve / members)
   ========================================================== */

async function renderParentDashboard() {
  if (!currentSession || currentSession.role !== "admin") return;
  
  const groupId = currentSession.choregroup_id;
  
  let pendingSubmissions = [];
  let familyMembers = [];
  
  try {
    // 1. Fetch chores list: GET /choregroups/{groupID}/tasks
    const tasks = await apiCall(`/choregroups/${groupId}/tasks`);
    activeTasks = tasks || [];
    
    // 2. Fetch pending submissions list: GET /choregroups/{groupId}/submissions
    const allSubmissions = await apiCall(`/choregroups/${groupId}/submissions`);
    pendingSubmissions = (allSubmissions || []).filter(s => {
      const task = activeTasks.find(t => t.id === s.task_id);
      return s.status === "pending_approval" || (task && task.status === "pending_approval");
    });
    
    // 3. Fetch family members list: GET /choregroups/${groupId}/members
    const members = await apiCall(`/choregroups/${groupId}/members`);
    familyMembers = members || [];
    
    // Self-healing: Update session if username was missing in legacy storage
    if (!currentSession.username) {
      const me = familyMembers.find(m => m.id === currentSession.user_id || m.role === 'admin');
      if (me) {
        currentSession.username = me.username;
        localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
        updateHeaderAuthBtn();
      }
    }
  } catch (err) {
    console.error("Dashboard render failed:", err);
    return;
  }
  
  // Render Active Chores list
  const choresList = document.getElementById("parent-chores-list");
  choresList.innerHTML = "";
  
  const activeOnly = activeTasks.filter(t => t.status !== "done");
  
  activeOnly.sort((a, b) => {
    if (a.is_mandatory !== b.is_mandatory) return a.is_mandatory ? -1 : 1;
    if (a.status !== b.status) return a.status === "assigned" ? -1 : 1;
    return 0;
  });
  
  if (activeOnly.length === 0) {
    choresList.innerHTML = "";
  } else {
    activeOnly.forEach(task => {
      const { emoji, title } = parseChoreTitle(task.title);
      let displayTitle = task.type === "cooperative" ? `${title} (👥)` : title;
      if (task.is_mandatory) displayTitle = "🚨 " + displayTitle + " (Mandatory)";

      
      let assigneeHtml = "";
      if (task.type !== "cooperative") {
        let assigneeName = "Anyone / Unassigned";
        if (task.assigned_to_user_id) {
          const assignee = familyMembers.find(u => u.id === task.assigned_to_user_id);
          assigneeName = assignee ? assignee.username : "Unassigned";
        }
        assigneeHtml = `<p>Assigned to: <strong>${assigneeName}</strong></p>`;
      }
      
      const card = document.createElement("div");
      card.className = `chore-card ${task.status === 'pending_approval' ? 'submitted' : ''}`;
      
      let statusHtml = "";
      let extendHtml = "";
      if (task.status === "pending_approval") {
        statusHtml = `<div class="card-status-badge"><span>⏳ Pending Approval</span></div>`;
      } else if (task.status === "done") {
        statusHtml = `<div class="card-status-badge" style="background-color: var(--success-green-light); color: var(--success-green); border-color: var(--success-green);"><span>💚 Completed</span></div>`;
      } else if (task.status === "expired") {
        statusHtml = `<div class="card-status-badge" style="background-color: #ffe3e3; color: #e03131; border-color: #ffa8a8;"><span>🚨 Expired</span></div>`;
        extendHtml = `<button class="btn btn-outline btn-sm" onclick="handleExtendTask('${task.id}')" style="padding: 0.25rem 0.5rem; font-size: 0.8rem; height: auto;">⏳ Extend (+1h)</button>`;
      }
      
      const controlsHtml = `
        <div class="card-controls">
          ${task.status !== "pending_approval" && task.status !== "expired" ? `<button class="btn btn-outline btn-sm" onclick="openEditChoreModal('${task.id}')">✏️ Edit</button>` : ''}
          ${task.status === "expired" ? `<button class="btn btn-outline btn-sm" onclick="openEditChoreModal('${task.id}')">✏️ Edit</button>` : ''}
          <button class="btn btn-outline btn-sm" style="color: var(--destructive-red); border-color: var(--destructive-red);" onclick="deleteChore('${task.id}')">🗑️ Delete</button>
        </div>
      `;
      
      let timerHtml = "";
      if (task.status === "assigned" && task.expires_at) {
        timerHtml = `<div class="task-timer-badge" data-expires="${task.expires_at}" data-mandatory="${task.is_mandatory}" style="margin-top: 0.5rem; font-size: 0.85rem; color: #e65100; font-weight: bold;">⏳ Loading timer...</div>`;
      }

      const coinHtml = task.type === "cooperative" ? `<span class="points-coin coop-theme">🐱</span>` : `<span class="points-coin">🐱</span>`;
      card.innerHTML = `
        <div class="card-badge" ${task.is_mandatory ? 'style="background-color: var(--danger-red); color: white;"' : ''}>${task.is_mandatory ? '0' : '+' + task.points_reward} ${coinHtml}</div>
        <div class="card-emoji-container">${emoji}</div>
        <div class="card-details">
          <h3>${displayTitle}</h3>
          ${assigneeHtml}
          ${timerHtml}
        </div>
        <div class="card-actions-row">
          ${statusHtml}
          ${extendHtml}
          ${controlsHtml}
        </div>
      `;
      choresList.appendChild(card);
    });
  }
  
  // Render Pending Submissions
  const pendingList = document.getElementById("parent-pending-list");
  pendingList.innerHTML = "";
  
  const pendingContainer = document.getElementById("parent-pending-submissions-container");
  if (pendingSubmissions.length === 0) {
    if (pendingContainer) pendingContainer.style.display = "none";
    pendingList.innerHTML = `<p class="no-pending-msg">✨ All clear! No pending submissions.</p>`;
  } else {
    if (pendingContainer) pendingContainer.style.display = "block";
    pendingSubmissions.forEach(sub => {
      const task = activeTasks.find(t => t.id === sub.task_id);
      const kid = familyMembers.find(u => u.id === sub.submitted_by);
      if (!task || !kid) return;
      
      const { emoji, title } = parseChoreTitle(task.title);
      const displayTitle = task.type === "cooperative" ? `${title} (👥)` : title;
      
      const subItem = document.createElement("div");
      subItem.className = "submission-item";
      subItem.innerHTML = `
        <div class="sub-header">
          <span class="sub-who">👾 ${kid.username} completed:</span>
          <span class="sub-pts">+${task.points_reward} ${task.type === 'cooperative' ? '<span class="points-coin coop-theme">🐱</span>' : '<span class="points-coin">🐱</span>'}</span>
        </div>
        <div class="sub-chore">
          <span class="sub-chore-emoji">${emoji}</span>
          <span>${displayTitle}</span>
        </div>
        <div class="sub-actions">
          <button class="btn-approve" onclick="approveSubmission('${sub.id}', '${task.id}')">✔️ Approve</button>
          <button class="btn-reject" onclick="rejectSubmission('${sub.id}', '${task.id}')">❌ Reject</button>
        </div>
      `;
      pendingList.appendChild(subItem);
    });
  }
  
  // Render Family Members
  const membersList = document.getElementById("parent-members-list");
  membersList.innerHTML = "";
  
  // Sort family members: admins always go first, followed by kids alphabetically
  const sortedMembers = [...familyMembers].sort((a, b) => {
    if (a.role === "admin" && b.role !== "admin") return -1;
    if (a.role !== "admin" && b.role === "admin") return 1;
    return a.username.localeCompare(b.username);
  });
  
  sortedMembers.forEach(m => {
    const item = document.createElement("div");
    item.className = "member-item";
    const isMe = currentSession && m.id === currentSession.user_id;
    const avatarHtml = m.role === "admin" 
      ? `<span class="member-avatar admin-avatar">👤<span class="avatar-crown">👑</span></span>`
      : `<span class="member-avatar">👤</span>`;
      
    let actionBtnHtml = "";
    if (m.role === "user") {
      actionBtnHtml = `<button class="btn btn-primary btn-xs" onclick="generateKidAccessLink('${m.id}')" style="margin-left: auto; font-size: 0.75rem; padding: 0.25rem 0.5rem;">🔗 Link</button>`;
    }
      
    item.innerHTML = `
      <div class="member-info">
        ${avatarHtml}
        <span class="member-name">${m.username}${isMe ? ' (Me)' : ''}</span>
      </div>
      ${actionBtnHtml}
    `;
    membersList.appendChild(item);
  });
}

// Populate assignees in the Add Chore dialog
async function populateAssigneeSelect(selectedId = "") {
  const select = document.getElementById("chore-assignee-input");
  if (!select) return;
  select.innerHTML = '<option value="">-- Unassigned / Anyone --</option>';
  
  const groupId = currentSession.choregroup_id;
  let kids = [];
  
  try {
    const members = await apiCall(`/choregroups/${groupId}/members`);
    kids = members.filter(m => m.role === "user");
  } catch (err) {
    console.error("Failed to load members for assignee list:", err);
    return;
  }
  
  kids.forEach(kid => {
    const opt = document.createElement("option");
    opt.value = kid.id;
    opt.innerText = kid.username;
    if (kid.id === selectedId) opt.selected = true;
    select.appendChild(opt);
  });
}

function toggleMandatoryChore(checked) {
  const pointsGroup = document.getElementById("chore-points-group");
  if (pointsGroup) {
    pointsGroup.style.display = checked ? "none" : "block";
  }
}

async function openAddChoreModal() {
  document.getElementById("chore-modal-title").innerText = "Create New Chore";
  document.getElementById("chore-id-input").value = "";
  document.getElementById("chore-title-input").value = "";
  
  const mandatoryInput = document.getElementById("chore-mandatory-input");
  if (mandatoryInput) {
    mandatoryInput.checked = false;
    toggleMandatoryChore(false);
  }
  
  document.getElementById("chore-type-input").value = "individual";
  toggleChoreType("individual");
  setPointsSelectValue("5");
  
  await populateAssigneeSelect();
  document.getElementById("chore-modal").classList.add("active");
}

// Add Chore submit handler
async function handleSaveChore(e) {
  e.preventDefault();
  
  const taskId = document.getElementById("chore-id-input").value;
  const titleVal = document.getElementById("chore-title-input").value.trim();
  const points = parseInt(document.getElementById("chore-points-input").value);
  const type = document.getElementById("chore-type-input").value;
  const isMandatory = document.getElementById("chore-mandatory-input") ? document.getElementById("chore-mandatory-input").checked : false;
  
  let assigneeId = document.getElementById("chore-assignee-input").value;
  if (type === "cooperative" || !assigneeId) {
    assigneeId = null;
  }
  
  const timerVal = document.getElementById("chore-timer-input").value;
  let expiresAt = null;
  if (timerVal === "keep") {
    const task = activeTasks.find(t => t.id === taskId);
    expiresAt = task ? task.expires_at : null;
  } else if (timerVal !== "none") {
    let durationMs = 0;
    if (timerVal === "15m") durationMs = 15 * 60000;
    else if (timerVal === "30m") durationMs = 30 * 60000;
    else if (timerVal === "1h") durationMs = 60 * 60000;
    else if (timerVal === "2h") durationMs = 2 * 60 * 60000;
    else if (timerVal === "12h") durationMs = 12 * 60 * 60000;
    else if (timerVal === "24h") durationMs = 24 * 60 * 60000;
    expiresAt = new Date(Date.now() + durationMs).toISOString();
  }

  const fullTitle = titleVal;
  const groupId = currentSession.choregroup_id;
  
  try {
    if (taskId) {
      // PUT /choregroups/{choregroupID}/tasks/{taskID} (UpdateTaskRequest)
      await apiCall(`/choregroups/${groupId}/tasks/${taskId}`, "PUT", {
        assigned_to_user_id: assigneeId || null,
        points_reward: points,
        title: fullTitle,
        type: type,
        is_mandatory: isMandatory,
        expires_at: expiresAt
      });
      showToast("Chore updated successfully!");
    } else {
      // POST /choregroups/{groupID}/tasks (CreateTaskRequest)
      await apiCall(`/choregroups/${groupId}/tasks`, "POST", {
        assigned_to_user_id: assigneeId || null,
        points_reward: points,
        title: fullTitle,
        type: type,
        is_mandatory: isMandatory,
        expires_at: expiresAt
      });
      showToast("Chore created on backend server!");
    }
  } catch (err) {
    alert("Failed to save chore: " + err.message);
  }
  
  closeModal("chore-modal");
  renderParentDashboard();
  updateLeaderboards();
}

async function openEditChoreModal(taskId) {
  const task = activeTasks.find(t => t.id === taskId);
  if (!task) return;
  
  document.getElementById("chore-modal-title").innerText = "Edit Chore";
  document.getElementById("chore-id-input").value = taskId;
  
  const { emoji, title } = parseChoreTitle(task.title);
  document.getElementById("chore-title-input").value = title;
  
  const mandatoryInput = document.getElementById("chore-mandatory-input");
  if (mandatoryInput) {
    mandatoryInput.checked = task.is_mandatory || false;
    toggleMandatoryChore(task.is_mandatory || false);
  }
  
  // Set cooperative theme context before setting points dropdown value
  document.getElementById("chore-type-input").value = task.type;
  toggleChoreType(task.type);
  setPointsSelectValue(task.points_reward);
  
  const timerSelect = document.getElementById("chore-timer-input");
  const keepOption = document.getElementById("chore-timer-keep-option");
  const helper = document.getElementById("chore-timer-status-helper");
  
  if (task.expires_at) {
    if (keepOption) keepOption.style.display = "block";
    if (timerSelect) timerSelect.value = "keep";
    
    const expires = new Date(task.expires_at);
    const diff = expires - Date.now();
    if (diff <= 0) {
      if (helper) helper.innerHTML = `⚠️ Already expired on ${expires.toLocaleTimeString()}`;
    } else {
      const mins = Math.floor(diff / 60000);
      const hours = Math.floor(mins / 60);
      const remainingMins = mins % 60;
      if (helper) {
        helper.innerHTML = `⏳ ${hours > 0 ? hours + 'h ' : ''}${remainingMins}m remaining (expires at ${expires.toLocaleTimeString()})`;
      }
    }
  } else {
    if (keepOption) keepOption.style.display = "none";
    if (timerSelect) timerSelect.value = "none";
    if (helper) helper.innerHTML = "";
  }

  await populateAssigneeSelect(task.assigned_to_user_id);
  document.getElementById("chore-modal").classList.add("active");
}

async function deleteChore(taskId) {
  const groupId = currentSession.choregroup_id;
  try {
    await apiCall(`/choregroups/${groupId}/tasks/${taskId}`, "DELETE");
    showToast("Chore deleted successfully!");
    renderParentDashboard();
    updateLeaderboards();
  } catch (err) {
    alert("Failed to delete chore: " + err.message);
  }
}

function openAddUserModal() {
  document.getElementById("new-member-name").value = "";
  document.getElementById("new-member-pass").value = "family";
  document.getElementById("new-member-groupname").value = (currentSession && currentSession.choregroup_name !== "My Household") ? currentSession.choregroup_name : "";
  document.getElementById("new-member-role").value = "user";
  document.getElementById("add-user-modal").classList.add("active");
}

async function handleAddUserSubmit(e) {
  e.preventDefault();
  
  const name = document.getElementById("new-member-name").value.trim();
  const password = document.getElementById("new-member-pass").value;
  const role = document.getElementById("new-member-role").value;
  const groupName = currentSession?.choregroup_name || "";
  
  if (!name || !groupName) {
    alert("Could not determine your household. Please log in again.");
    return;
  }
  
  try {
    // POST /users (AddUserRequest)
    await apiCall("/users", "POST", {
      choregroup_name: groupName,
      password: password,
      user_role: role,
      username: name
    });
    showToast(`User ${name} added successfully!`);
  } catch (err) {
    alert("Failed to add member: " + err.message);
  }
  
  closeModal("add-user-modal");
  renderParentDashboard();
  updateLeaderboards();
}

/* ==========================================================
   SUBMISSIONS APPROVAL CONTROLS (PUT /status)
   ========================================================== */

async function approveSubmission(submissionId, taskId) {
  const groupId = currentSession.choregroup_id;
  
  try {
    // PUT /choregroups/{choregroupID}/tasks/{taskID}/status (UpdateSubmissionRequest)
    await apiCall(`/choregroups/${groupId}/tasks/${taskId}/status`, "PUT", {
      action: "approve"
    });
    showToast("🏆 Submission approved on server!");
  } catch (err) {
    alert("Failed to approve submission: " + err.message);
  }
  
  renderParentDashboard();
  updateLeaderboards();
}

async function rejectSubmission(submissionId, taskId) {
  const groupId = currentSession.choregroup_id;
  
  try {
    // PUT /choregroups/{choregroupID}/tasks/{taskID}/status (UpdateSubmissionRequest)
    await apiCall(`/choregroups/${groupId}/tasks/${taskId}/status`, "PUT", {
      action: "reject"
    });
    showToast("Submission returned to Kid.");
  } catch (err) {
    alert("Failed to reject: " + err.message);
  }
  
  renderParentDashboard();
}

/* ==========================================================
   KID VIEW EXECUTION PANEL (Tasks / submit / leaderboard)
   ========================================================== */

async function renderKidDashboard() {
  if (!currentSession || currentSession.role !== "user") return;
  
  const userId = currentSession.user_id;
  const groupId = currentSession.choregroup_id;
  
  let kidTasks = [];
  let userPoints = 0;
  
  try {
    // 1. Fetch chores list: GET /choregroups/{groupID}/tasks
    const tasks = await apiCall(`/choregroups/${groupId}/tasks`);
    kidTasks = (tasks || []).filter(t => !t.assigned_to_user_id || t.assigned_to_user_id === userId || t.type === 'cooperative');
    
    // 2. Fetch members list to extract point balance
    const members = await apiCall(`/choregroups/${groupId}/members`);
    const me = (members || []).find(m => m.id === userId);
    userPoints = me ? me.points : 0;

    // 3. Fetch statistics to extract group cooperative points for the header
    const statsData = await apiCall(`/choregroups/${groupId}/statistics`) || { users: [], cooperative_points: 0 };
    currentSession.cooperative_points = statsData.cooperative_points || 0;
  } catch (err) {
    console.error("Kid Dashboard render failed:", err);
    return;
  }
  
  // Set UI Details: Save fresh points and refresh header auth banner
  currentSession.points = userPoints;
  localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
  updateHeaderAuthBtn();
  
  // Render My Chores list
  const kidChoresList = document.getElementById("kid-chores-list");
  kidChoresList.innerHTML = "";
  
  const activeOnly = kidTasks.filter(t => t.status !== "done");
  // A mandatory task blocks non-mandatory tasks if it's assigned OR pending approval
  const hasPendingMandatory = activeOnly.some(t => t.is_mandatory && (t.status === "assigned" || t.status === "pending_approval"));
  
  activeOnly.sort((a, b) => {
    if (a.is_mandatory !== b.is_mandatory) return a.is_mandatory ? -1 : 1;
    if (a.status !== b.status) return a.status === "assigned" ? -1 : 1;
    return 0;
  });
  
  if (activeOnly.length === 0) {
    kidChoresList.innerHTML = "";
  } else {
    activeOnly.forEach(task => {
      const isDisabled = !task.is_mandatory && hasPendingMandatory;
      const card = document.createElement("div");
      card.className = `chore-card ${task.status === 'pending_approval' ? 'submitted' : ''} ${isDisabled ? 'disabled' : ''}`;
      
      let timerHtml = "";
      if (task.status === "assigned" && task.expires_at) {
        timerHtml = `<div class="task-timer-badge" data-expires="${task.expires_at}" data-mandatory="${task.is_mandatory}" style="margin-top: 0.5rem; font-size: 0.85rem; color: #e65100; font-weight: bold;">⏳ Loading timer...</div>`;
      }

      let actionHtml = "";
      if (task.status === "assigned") {
        actionHtml = `
          <button class="btn-action-submit" onclick="submitChoreForApproval('${task.id}')" ${isDisabled ? 'disabled title="Complete mandatory chores first!"' : ''}>
            ✔️
          </button>
        `;
      } else if (task.status === "pending_approval") {
        actionHtml = `
          <div class="card-status-badge">
            <span>⏳ Awaiting Parent Approval</span>
          </div>
        `;
      } else if (task.status === "done") {
        actionHtml = `
          <div class="card-status-badge" style="background-color: var(--success-green-light); color: var(--success-green); border-color: var(--success-green);">
            <span>💚 Completed & Approved!</span>
          </div>
        `;
      } else if (task.status === "expired") {
        actionHtml = `
          <div class="card-status-badge" style="background-color: #ffe3e3; color: #e03131; border-color: #ffa8a8; padding: 0.35rem 0.75rem; border-radius: var(--radius-md); font-size: 0.85rem; font-weight: bold;">
            <span>🚨 Expired</span>
          </div>
        `;
      }
      
      const { emoji, title } = parseChoreTitle(task.title);
      let displayTitle = task.type === "cooperative" ? `${title} (👥)` : title;
      if (task.is_mandatory) displayTitle = "🚨 " + displayTitle + " (Mandatory)";
      
      const coinHtml = task.type === "cooperative" ? `<span class="points-coin coop-theme">🐱</span>` : `<span class="points-coin">🐱</span>`;
      card.innerHTML = `
        <div class="card-badge" ${task.is_mandatory ? 'style="background-color: var(--danger-red); color: white;"' : ''}>${task.is_mandatory ? '0' : '+' + task.points_reward} ${coinHtml}</div>
        <div class="card-emoji-container">${emoji}</div>
        <div class="card-details">
          <h3 ${isDisabled ? 'style="color: #999;"' : ''}>${displayTitle}</h3>
          ${timerHtml}
        </div>
        ${actionHtml}
      `;
      kidChoresList.appendChild(card);
    });
  }
  
  await renderKidSidebarWidgets();
}

async function renderKidSidebarWidgets() {
  const groupId = currentSession.choregroup_id;
  let kids = [];
  
  try {
    const members = await apiCall(`/choregroups/${groupId}/members`);
    kids = members.filter(m => m.role === "user");
  } catch (err) {
    return;
  }
  
  const sortedKids = [...kids].sort((a,b) => b.points - a.points);
  
  const first = sortedKids[0] || { username: "-", points: 0 };
  const second = sortedKids[1] || { username: "-", points: 0 };
  const third = sortedKids[2] || { username: "-", points: 0 };
  
  const podiumContainer = document.getElementById("kid-podium-container");
  if (podiumContainer) {
    podiumContainer.innerHTML = `
      <div class="podium-col second">
        <div class="avatar-ring silver">🥈</div>
        <div class="podium-name">${second.username}</div>
        <div class="podium-bar">
          <span class="points-label">${second.points} <span class="points-coin">🐱</span></span>
        </div>
      </div>
      <div class="podium-col first">
        <div class="avatar-ring gold">👑</div>
        <div class="podium-name">${first.username}</div>
        <div class="podium-bar">
          <span class="points-label">${first.points} <span class="points-coin">🐱</span></span>
        </div>
      </div>
      <div class="podium-col third">
        <div class="avatar-ring bronze">🥉</div>
        <div class="podium-name">${third.username}</div>
        <div class="podium-bar">
          <span class="points-label">${third.points} <span class="points-coin">🐱</span></span>
        </div>
      </div>
    `;
  }
  
  const chatMessagesBox = document.getElementById("kid-chat-messages-box");
  if (chatMessagesBox && chatMessagesBox.children.length === 0) {
    chatMessagesBox.innerHTML = `
      <div class="msg ai-msg">
        <span class="msg-avatar">🤖</span>
        <div class="msg-bubble">Ask me how many points a custom reward should cost! (e.g. ice cream, playing iPad)</div>
      </div>
    `;
  }
}

// Submit chore (POST /tasks/{id}/submit)
async function submitChoreForApproval(taskId) {
  const groupId = currentSession.choregroup_id;
  
  try {
    // POST /choregroups/{choregroupID}/tasks/{taskID}/submit
    await apiCall(`/choregroups/${groupId}/tasks/${taskId}/submit`, "POST");
    showToast("Submitted to backend! ⏳");
  } catch (err) {
    alert("Submission failed: " + err.message);
  }
  
  renderKidDashboard();
  updateLeaderboards();
}

/* ==========================================================
   GAMIFIED LEADERBOARD (GET /leaderboard)
   ========================================================== */

async function updateLeaderboards() {
  let sortedKids = [];
  let allTasks = [];
  let cooperativeCoins = 0;
  
  if (currentSession) {
    try {
      const groupId = currentSession.choregroup_id;
      // 1. Fetch statistics: GET /choregroups/{choregroupID}/statistics
      const statsData = await apiCall(`/choregroups/${groupId}/statistics`);
      sortedKids = (statsData.users || []).filter(m => m.role === "user");
      cooperativeCoins = statsData.cooperative_points || 0;
      if (currentSession && currentSession.role === "user") {
        const me = sortedKids.find(k => k.id === currentSession.user_id);
        if (me) {
          currentSession.points = me.points;
        }
        currentSession.cooperative_points = cooperativeCoins;
        updateHeaderAuthBtn();
      }
      
      // 2. Fetch all tasks to compute completion rates and active chores
      const tasks = await apiCall(`/choregroups/${groupId}/tasks`);
      allTasks = tasks || [];
    } catch (err) {
      console.error("Stats fetch failed:", err);
    }
  }
  
  sortedKids.sort((a,b) => b.points - a.points);
  
  // Calculate metrics
  const kidsCoinsSum = sortedKids.reduce((sum, k) => sum + k.points, 0);
  const totalCoins = kidsCoinsSum + cooperativeCoins;
  const totalTasksCount = allTasks.length;
  const completedCount = allTasks.filter(t => t.status === "done").length;
  const activeCount = allTasks.filter(t => t.status !== "done").length;
  
  const completionRate = totalTasksCount > 0 
    ? Math.round((completedCount / totalTasksCount) * 100) 
    : 0;

  // Breakdown progress bars
  let maxCoins = Math.max(...sortedKids.map(k => k.points), 1);
  let breakdownHtml = sortedKids.map(kid => {
    const pct = Math.round((kid.points / maxCoins) * 100);
    return `
      <div class="stats-progress-row">
        <span class="progress-name">${kid.username}</span>
        <div class="progress-bar-container">
          <div class="progress-bar-fill" style="width: ${pct}%"></div>
        </div>
        <span class="progress-value">${kid.points} <span class="points-coin">🐱</span></span>
      </div>
    `;
  }).join("");
  
  if (breakdownHtml === "") {
    breakdownHtml = `<p class="text-muted" style="font-size: 0.85rem; text-align: center; margin: 0;">No kids registered yet.</p>`;
  }

  const statsHtml = `
    <div class="stats-container">
      <div class="stats-metric">
        <span class="stats-label"><span class="points-coin coop-theme">🐱</span> Cooperative Coins:</span>
        <span class="stats-value">${cooperativeCoins} <span class="points-coin coop-theme">🐱</span></span>
      </div>
      <div class="stats-contributions">
        <h4>Coins Breakdown:</h4>
        ${breakdownHtml}
      </div>
    </div>
  `;

  // Render to all containers if they exist
  const parentStats = document.getElementById("parent-stats-container");
  if (parentStats) {
    parentStats.innerHTML = statsHtml;
  }
  
  const kidStats = document.getElementById("kid-stats-container");
  if (kidStats) {
    kidStats.innerHTML = statsHtml;
  }

  const kidRewardsStats = document.getElementById("kid-rewards-stats-container");
  if (kidRewardsStats) {
    kidRewardsStats.innerHTML = statsHtml;
  }
}

/* ==========================================================
   AI REWARD APPRAISER CORE LOGIC
   ========================================================== */

function appraiseReward(isKidView = false) {
  const inputId = isKidView ? "kid-ai-input" : "ai-appraiser-input";
  const messagesBoxId = isKidView ? "kid-chat-messages-box" : "chat-messages-box";
  
  const input = document.getElementById(inputId);
  const chatMessages = document.getElementById(messagesBoxId);
  
  const requestText = input.value.trim();
  if (!requestText) return;
  
  const userMsg = document.createElement("div");
  userMsg.className = "msg kid-msg";
  userMsg.innerHTML = `
    <span class="msg-avatar">👾</span>
    <div class="msg-bubble">I want: "${requestText}"</div>
  `;
  chatMessages.appendChild(userMsg);
  
  input.value = "";
  chatMessages.scrollTop = chatMessages.scrollHeight;
  
  const loadingMsg = document.createElement("div");
  loadingMsg.className = "msg ai-msg";
  loadingMsg.innerHTML = `
    <span class="msg-avatar">🤖</span>
    <div class="msg-bubble">Thinking... 🤖🧠</div>
  `;
  chatMessages.appendChild(loadingMsg);
  chatMessages.scrollTop = chatMessages.scrollHeight;
  
  setTimeout(() => {
    chatMessages.removeChild(loadingMsg);
    
    const lowerText = requestText.toLowerCase();
    let estimatedPoints = 3;
    let reasoning = "Standard physical reward request. Moderate parenting effort needed.";
    
    if (lowerText.includes("video") || lowerText.includes("game") || lowerText.includes("roblox") || lowerText.includes("fortnite") || lowerText.includes("minecraft") || lowerText.includes("screen") || lowerText.includes("ipad") || lowerText.includes("tablet") || lowerText.includes("computer")) {
      estimatedPoints = 5;
      reasoning = "Requires screen time scheduling and configuration by parents. Screens are priced higher to encourage active physical play.";
    } else if (lowerText.includes("candy") || lowerText.includes("ice cream") || lowerText.includes("sugar") || lowerText.includes("sweet") || lowerText.includes("chocolate") || lowerText.includes("cookie")) {
      estimatedPoints = 4;
      reasoning = "Sugar reward. Requires parent verification for moderation. Balance with physical activity and healthy snacks.";
    } else if (lowerText.includes("toy") || lowerText.includes("lego") || lowerText.includes("doll") || lowerText.includes("buy") || lowerText.includes("plush")) {
      estimatedPoints = 25;
      reasoning = "Physical retail items require cash budgeting. High point requirements incentivize long-term chores commitment.";
    } else if (lowerText.includes("sleepover") || lowerText.includes("friend") || lowerText.includes("park") || lowerText.includes("zoo")) {
      estimatedPoints = 12;
      reasoning = "Social outings involve parental schedule coordination, travel, and supervision. Moderate point pricing reflects resource investment.";
    } else if (lowerText.includes("sleep") || lowerText.includes("bedtime") || lowerText.includes("stay up")) {
      estimatedPoints = 8;
      reasoning = "Altering sleep routines impacts physical recovery. Fairly high pricing keeps child's schedule consistent.";
    }
    
    const aiMsg = document.createElement("div");
    aiMsg.className = "msg ai-msg";
    
    if (isKidView) {
      aiMsg.innerHTML = `
        <span class="msg-avatar">🤖</span>
        <div class="msg-bubble ai-json-bubble compact-json">
          <strong>${estimatedPoints} Points</strong>: ${reasoning}
        </div>
      `;
    } else {
      aiMsg.innerHTML = `
        <span class="msg-avatar">🤖</span>
        <div class="msg-bubble ai-json-bubble">
          <div class="json-card-title">💡 Reward Proposal Appraisal</div>
          <div class="json-item"><strong>Estimated Value:</strong> <span class="pts-badge">${estimatedPoints} Points</span></div>
          <div class="json-item"><strong>Reasoning:</strong> ${reasoning}</div>
        </div>
      `;
    }
    
    chatMessages.appendChild(aiMsg);
    chatMessages.scrollTop = chatMessages.scrollHeight;
  }, 1000);
}

/* ==========================================================
   HERO INTERACTIVE PHONE DEMO (Visual Landing Page Simulation Only)
   ========================================================== */

function simulateHeroChoreCompletion(button) {
  const dbPtsCounter = document.getElementById("demo-pts-counter");
  const heroCard = document.getElementById("hero-demo-card");
  
  if (button.innerText.includes("Done") || button.innerText.includes("סיימתי")) {
    showToast("Demo Chore Completed! Points balance increased in kid view! 🎉");
    
    dbPtsCounter.innerText = "19 Pts";
    dbPtsCounter.style.backgroundColor = "var(--success-green)";
    dbPtsCounter.style.transform = "scale(1.15)";
    setTimeout(() => {
      dbPtsCounter.style.transform = "scale(1)";
    }, 300);
    
    heroCard.classList.add("submitted");
    button.innerText = "⏳ Awaiting Approval...";
    button.style.backgroundColor = "var(--warning-amber)";
    button.disabled = true;
    
    setTimeout(() => {
      showToast("Parent Admin approved the demo chore! +5 Points. 🏆");
      button.innerText = "💚 Completed!";
      button.style.backgroundColor = "var(--success-green-hover)";
      
      const podiumPts = document.getElementById("leaderboard-first-pts");
      if (podiumPts) {
        podiumPts.innerText = "19 Pts";
        podiumPts.style.fontWeight = "900";
      }
    }, 2500);
  }
}

/* ==========================================
   TOAST SYSTEM UTIL
   ========================================== */
function showToast(message) {
  const toast = document.getElementById("toast-message");
  if (!toast) return;
  toast.innerText = message;
  toast.classList.add("active");
  
  setTimeout(() => {
    toast.classList.remove("active");
  }, 4000);
}

/* ==========================================
   REWARDS STATE, NAVIGATION & API ENGINE
   ========================================== */

let currentParentTab = 'chores';
let currentKidTab = 'chores';
let currentParentRewardSubTab = 'available';
let currentKidRewardSubTab = 'available';

/* ---------- Hash-based routing ---------- */

/**
 * Parse the URL hash into { tab, subTab }.
 * Formats: #chores | #rewards | #rewards/pending | #rewards/fulfillment
 */
function getHashState() {
  const raw = window.location.hash.replace('#', '');
  const [tab = 'chores', subTab = 'available'] = raw.split('/');
  const validTabs = ['chores', 'rewards'];
  const validSubTabs = ['available', 'pending', 'fulfillment'];
  return {
    tab: validTabs.includes(tab) ? tab : 'chores',
    subTab: validSubTabs.includes(subTab) ? subTab : 'available'
  };
}

/**
 * Update the browser URL hash without triggering a hashchange event.
 * Uses replaceState so it doesn't pollute browser history on every sub-tab click.
 */
function updateHash(tab, subTab = 'available') {
  const hash = (tab === 'rewards' && subTab !== 'available') ? `rewards/${subTab}` : tab;
  history.replaceState(null, '', `#${hash}`);
}

/**
 * Read the current URL hash and navigate to the right tab/sub-tab.
 * Called on page load (after session restored) and on hashchange (back/forward).
 */
function navigateFromHash() {
  if (!currentSession) return;
  const { tab, subTab } = getHashState();
  const isParent = currentSession.role === 'admin';

  if (isParent) {
    // Apply main tab
    currentParentTab = tab;
    document.getElementById('parent-tab-chores').classList.toggle('active', tab === 'chores');
    document.getElementById('parent-tab-rewards').classList.toggle('active', tab === 'rewards');
    document.getElementById('parent-chores-content').style.display = tab === 'chores' ? 'block' : 'none';
    document.getElementById('parent-rewards-content').style.display = tab === 'rewards' ? 'block' : 'none';

    if (tab === 'chores') {
      renderParentDashboard();
    } else {
      // Apply sub-tab
      currentParentRewardSubTab = subTab;
      const subAvail = document.getElementById('parent-reward-sub-available');
      if (subAvail) subAvail.classList.toggle('active', subTab === 'available');
      const subPend = document.getElementById('parent-reward-sub-pending');
      if (subPend) subPend.classList.toggle('active', subTab === 'pending');
      const subFulfill = document.getElementById('parent-reward-sub-fulfillment');
      if (subFulfill) subFulfill.classList.toggle('active', subTab === 'fulfillment');
      const secAvail = document.getElementById('parent-rewards-available-section');
      if (secAvail) secAvail.style.display = subTab === 'available' ? 'block' : 'none';
      const secPend = document.getElementById('parent-rewards-pending-section');
      if (secPend) secPend.style.display = subTab === 'pending' ? 'block' : 'none';
      const secFulfill = document.getElementById('parent-rewards-fulfillment-section');
      if (secFulfill) secFulfill.style.display = subTab === 'fulfillment' ? 'block' : 'none';
      renderRewardsDashboard();
    }
  } else {
    // Kid
    currentKidTab = tab;
    document.getElementById('kid-tab-chores').classList.toggle('active', tab === 'chores');
    document.getElementById('kid-tab-rewards').classList.toggle('active', tab === 'rewards');
    document.getElementById('kid-chores-content').style.display = tab === 'chores' ? 'block' : 'none';
    document.getElementById('kid-rewards-content').style.display = tab === 'rewards' ? 'block' : 'none';

    if (tab === 'chores') {
      renderKidDashboard();
    } else {
      renderRewardsDashboard();
    }
  }
}

// React to browser back / forward button
window.addEventListener('hashchange', () => {
  if (currentSession) navigateFromHash();
});

/* ---------- Tab switch functions (now also update the URL hash) ---------- */

function switchParentTab(tab) {
  currentParentTab = tab;
  updateHash(tab, tab === 'rewards' ? currentParentRewardSubTab : 'available');
  document.getElementById('parent-tab-chores').classList.toggle('active', tab === 'chores');
  document.getElementById('parent-tab-rewards').classList.toggle('active', tab === 'rewards');
  
  document.getElementById('parent-chores-content').style.display = tab === 'chores' ? 'block' : 'none';
  document.getElementById('parent-rewards-content').style.display = tab === 'rewards' ? 'block' : 'none';
  
  if (tab === 'rewards') {
    renderRewardsDashboard();
  }
}

function switchKidTab(tab) {
  currentKidTab = tab;
  updateHash(tab, 'available');
  document.getElementById('kid-tab-chores').classList.toggle('active', tab === 'chores');
  document.getElementById('kid-tab-rewards').classList.toggle('active', tab === 'rewards');
  
  document.getElementById('kid-chores-content').style.display = tab === 'chores' ? 'block' : 'none';
  document.getElementById('kid-rewards-content').style.display = tab === 'rewards' ? 'block' : 'none';
  
  if (tab === 'chores') {
    renderKidDashboard();
  } else if (tab === 'rewards') {
    renderRewardsDashboard();
  }
}

function switchParentRewardSubTab(subTab) {
  currentParentRewardSubTab = subTab;
  updateHash('rewards', subTab);
  
  const subAvail = document.getElementById('parent-reward-sub-available');
  if (subAvail) subAvail.classList.toggle('active', subTab === 'available');
  const subFulfill = document.getElementById('parent-reward-sub-fulfillment');
  if (subFulfill) subFulfill.classList.toggle('active', subTab === 'fulfillment');
  
  const secAvail = document.getElementById('parent-rewards-available-section');
  if (secAvail) secAvail.style.display = subTab === 'available' ? 'block' : 'none';
  const secFulfill = document.getElementById('parent-rewards-fulfillment-section');
  if (secFulfill) secFulfill.style.display = subTab === 'fulfillment' ? 'block' : 'none';
  
  renderRewardsDashboard();
}


function escapeHTML(str) {
  if (!str) return '';
  return str
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

async function renderRewardsDashboard() {
  if (!currentSession) return;
  
  const choregroupID = currentSession.choregroup_id;
  const isParent = currentSession.role === "admin";
  
  try {
    const rewards = await apiCall(`/choregroups/${choregroupID}/rewards`) || [];
    const stats = await apiCall(`/choregroups/${choregroupID}/statistics`) || { users: [], cooperative_points: 0 };
    const userStats = stats.users.find(u => u.id === currentSession.user_id);
    const myPoints = userStats ? userStats.points : 0;
    const groupPoints = stats.cooperative_points || 0;
    
    const members = await apiCall(`/choregroups/${choregroupID}/members`) || [];
    const membersMap = {};
    members.forEach(m => {
      membersMap[m.id] = m.username;
    });

    if (!isParent && userStats) {
      currentSession.points = userStats.points;
      currentSession.cooperative_points = groupPoints;
      updateHeaderAuthBtn();
    }
    
    // Fetch all purchases to handle one-time rewards logic
    const allPurchases = await apiCall(`/choregroups/${choregroupID}/purchases`) || [];
    const purchasedRewardIds = new Set(allPurchases.filter(p => p.status !== 'rejected' && p.status !== 'cancelled').map(p => p.reward_id));
    let availableRewards = rewards.filter(r => !purchasedRewardIds.has(r.id));
    if (!isParent) {
      availableRewards = availableRewards.filter(r => r.type === 'cooperative' || !r.assigned_to_user_id || r.assigned_to_user_id === currentSession.user_id);
      availableRewards.sort((a, b) => {
        if (a.type !== b.type) return a.type === 'individual' ? -1 : 1;
        return a.cost - b.cost;
      });
    } else {
      availableRewards.sort((a, b) => {
        if (a.type !== b.type) return a.type === 'individual' ? -1 : 1;
        const aAssigned = a.assigned_to_user_id ? 1 : 0;
        const bAssigned = b.assigned_to_user_id ? 1 : 0;
        if (aAssigned !== bAssigned) return aAssigned - bAssigned;
        return a.cost - b.cost;
      });
    }
    
    if (isParent) {
      const fulfillmentPurchases = allPurchases.filter(p => p.status === 'approved');
      const countEl = document.getElementById('parent-fulfillment-rewards-count');
      if (countEl) countEl.innerText = fulfillmentPurchases.length;
      
      const listContainer = document.getElementById('parent-rewards-list');
      if (listContainer) {
        if (availableRewards.length === 0) {
          listContainer.innerHTML = `<div class="empty-state" style="padding: 3rem; text-align: center; color: var(--text-muted); font-weight: 700; font-size: 1.1rem;">🎁 No available rewards.</div>`;
        } else {
          listContainer.innerHTML = availableRewards.map(reward => {
            const isCoop = reward.type === 'cooperative';
            const cost = reward.cost;
            const isAssignedToOther = !isCoop && reward.assigned_to_user_id && reward.assigned_to_user_id !== currentSession.user_id;
            const isAssignedToMe = !isCoop && reward.assigned_to_user_id && reward.assigned_to_user_id === currentSession.user_id;
            
            let displayNote = '';
            if (isAssignedToOther) {
              const assignedName = membersMap[reward.assigned_to_user_id] || 'other user';
              displayNote = `<div class="reward-assigned-user">👤 For ${escapeHTML(assignedName)} only</div>`;
            } else if (isAssignedToMe) {
              displayNote = `<div class="reward-assigned-user" style="color: var(--success-green); background-color: var(--success-green-light)">⭐ Assigned to me!</div>`;
            }

            const costClass = isCoop ? 'reward-cost-display coop-theme' : 'reward-cost-display';
            const { emoji, title } = parseChoreTitle(reward.name);
            const adminButtons = `
              <button class="btn btn-outline btn-sm" onclick="openEditRewardModal('${reward.id}')">✏️ Edit</button>
              <button class="btn btn-outline btn-sm" style="color: var(--destructive-red); border-color: var(--destructive-red);" onclick="handleDeleteReward('${reward.id}')">🗑️ Delete</button>
            `;
            
            return `
              <div class="reward-card">
                <div class="reward-card-header">
                  <div class="reward-emoji-box">${emoji}</div>
                </div>
                <div class="reward-card-body">
                  <div class="reward-title">${escapeHTML(title)}</div>
                  <div class="reward-desc">${escapeHTML(reward.description || '')}</div>
                  ${displayNote}
                </div>
                <div class="reward-card-footer">
                  <div class="${costClass}">${cost} <span class="points-coin">🐱</span></div>
                  <div class="reward-actions">
                    ${adminButtons}
                  </div>
                </div>
              </div>
            `;
          }).join('');
        }
      }

      const fulfillContainer = document.getElementById('parent-rewards-fulfillment-list');
      const fulfillWrapper = document.getElementById('parent-rewards-fulfillment-container');
      if (fulfillContainer) {
        if (fulfillmentPurchases.length === 0) {
          if (fulfillWrapper) fulfillWrapper.style.display = 'none';
          fulfillContainer.innerHTML = `<div class="empty-state" style="padding: 3rem; text-align: center; color: var(--text-muted); font-weight: 700; font-size: 1.1rem;">📦 No rewards waiting for fulfillment. All clear!</div>`;
        } else {
          if (fulfillWrapper) fulfillWrapper.style.display = 'block';
          fulfillContainer.innerHTML = fulfillmentPurchases.map(purchase => {
            const reward = rewards.find(r => r.id === purchase.reward_id);
            const rewardName = reward ? reward.name : 'Unknown Reward';
            const buyer = membersMap[purchase.purchased_by_user_id] || 'a household member';
            
            return `
              <div class="pending-item" style="display: flex; justify-content: space-between; align-items: center; background-color: var(--card-bg); padding: 1.25rem; border: 2px solid var(--border-color); border-radius: var(--radius-lg); margin-bottom: 1rem;">
                <div class="pending-info">
                  <div class="pending-title" style="font-weight: 800; font-size: 1.1rem; color: var(--text-main);">🎁 ${escapeHTML(rewardName)}</div>
                  <div class="reward-purchase-meta">Purchased by <strong>${escapeHTML(buyer)}</strong></div>
                </div>
                <div class="pending-actions">
                  <button class="btn btn-primary btn-sm" onclick="handleFulfillPurchase('${purchase.id}')">📦 Mark as Fulfilled</button>
                </div>
              </div>
            `;
          }).join('');
        }
      }

    } else {
      // Kid Logic
      const myPendingPurchases = allPurchases.filter(p => p.purchased_by_user_id === currentSession.user_id && p.status !== 'fulfilled' && p.status !== 'rejected');
      const awaitingMyVotePurchases = allPurchases.filter(p => {
        if (p.status !== 'pending_approval') return false;
        const approvals = (typeof p.approvals === 'object' && p.approvals !== null) ? p.approvals : {};
        return approvals[currentSession.user_id] === 'pending';
      });

      const availableContainer = document.getElementById('kid-rewards-list');
      if (availableContainer) {
        if (availableRewards.length === 0) {
          availableContainer.innerHTML = `<div class="empty-state" style="padding: 3rem; text-align: center; color: var(--text-muted); font-weight: 700; font-size: 1.1rem;">🎁 No rewards available right now.</div>`;
        } else {
          availableContainer.innerHTML = availableRewards.map(reward => {
            const isCoop = reward.type === 'cooperative';
            const cost = reward.cost;
            const hasEnough = isCoop ? (groupPoints >= cost) : (myPoints >= cost);
            const isAssignedToOther = !isCoop && reward.assigned_to_user_id && reward.assigned_to_user_id !== currentSession.user_id;
            const isAssignedToMe = !isCoop && reward.assigned_to_user_id && reward.assigned_to_user_id === currentSession.user_id;
            
            let buyDisabled = !hasEnough || isAssignedToOther;
            let displayNote = '';
            if (isAssignedToOther) {
              const assignedName = membersMap[reward.assigned_to_user_id] || 'other user';
              displayNote = `<div class="reward-assigned-user">🔒 For ${escapeHTML(assignedName)} only</div>`;
            } else if (isAssignedToMe) {
              displayNote = `<div class="reward-assigned-user" style="color: var(--success-green); background-color: var(--success-green-light)">⭐ Assigned to me!</div>`;
            }

            const costClass = isCoop ? 'reward-cost-display coop-theme' : 'reward-cost-display';
            const { emoji, title } = parseChoreTitle(reward.name);
            const buyButtonText = isCoop ? 'Buy (Co-op)' : 'Buy';
            const buttonClass = isCoop ? 'btn btn-primary btn-sm coop-theme' : 'btn btn-primary btn-sm';
            const buyButton = `<button class="${buttonClass}" ${buyDisabled ? 'disabled' : ''} onclick="handleBuyReward('${reward.id}', '${reward.name.replace(/'/g, "\\'")}')">${buyButtonText}</button>`;
            
            return `
              <div class="reward-card ${buyDisabled ? 'disabled' : ''}">
                <div class="reward-card-header">
                  <div class="reward-emoji-box">${emoji}</div>
                </div>
                <div class="reward-card-body">
                  <div class="reward-title">${escapeHTML(title)}</div>
                  <div class="reward-desc">${escapeHTML(reward.description || '')}</div>
                  ${displayNote}
                </div>
                <div class="reward-card-footer">
                  <div class="${costClass}">${cost} <span class="points-coin">🐱</span></div>
                  <div class="reward-actions">
                    ${buyButton}
                  </div>
                </div>
              </div>
            `;
          }).join('');
        }
      }

      const myPendingContainer = document.getElementById('kid-rewards-my-pending-list');
      const myPendingWrapper = document.getElementById('kid-rewards-my-pending-container');
      if (myPendingContainer) {
        if (myPendingPurchases.length === 0) {
          if (myPendingWrapper) myPendingWrapper.style.display = 'none';
          myPendingContainer.innerHTML = `<div class="empty-state" style="padding: 1.5rem; text-align: center; color: var(--text-muted); font-size: 0.95rem;">📦 No pending rewards.</div>`;
        } else {
          if (myPendingWrapper) myPendingWrapper.style.display = 'block';
          myPendingContainer.innerHTML = myPendingPurchases.map(purchase => {
            const reward = rewards.find(r => r.id === purchase.reward_id);
            const rewardName = reward ? reward.name : 'Unknown Reward';
            const statusLabel = purchase.status === 'pending_approval' ? '⏳ Voting...' : '✅ Approved (Awaiting Parent)';
            return `
              <div class="pending-item" style="display: flex; flex-direction: column; gap: 0.75rem; background-color: var(--card-bg); padding: 1rem; border: 2px solid var(--border-color); border-radius: var(--radius-lg); margin-bottom: 0.75rem;">
                <div class="pending-info">
                  <div class="pending-title" style="font-weight: 800; font-size: 1rem; color: var(--text-main);">🎁 ${escapeHTML(rewardName)}</div>
                  <div class="reward-purchase-meta" style="font-size: 0.85rem; color: var(--text-muted);">${statusLabel}</div>
                </div>
                <div class="pending-actions" style="display: flex; width: 100%;">
                  <button class="btn btn-outline btn-sm" style="flex: 1; color: var(--destructive-red); border-color: var(--destructive-red); padding: 0.35rem;" onclick="handleCancelPurchase('${purchase.id}')">❌ Cancel</button>
                </div>
              </div>
            `;
          }).join('');
        }
      }

      const voteContainer = document.getElementById('kid-rewards-pending-list');
      const voteWrapper = document.getElementById('kid-rewards-pending-container');
      if (voteContainer) {
        if (awaitingMyVotePurchases.length === 0) {
          if (voteWrapper) voteWrapper.style.display = 'none';
          voteContainer.innerHTML = `<div class="empty-state" style="padding: 1.5rem; text-align: center; color: var(--text-muted); font-size: 0.95rem;">⏳ No rewards awaiting your vote.</div>`;
        } else {
          if (voteWrapper) voteWrapper.style.display = 'block';
          voteContainer.innerHTML = awaitingMyVotePurchases.map(purchase => {
            const reward = rewards.find(r => r.id === purchase.reward_id);
            const rewardName = reward ? reward.name : 'Unknown Reward';
            const requester = membersMap[purchase.purchased_by_user_id] || 'a household member';
            return `
              <div class="pending-item" style="display: flex; flex-direction: column; gap: 0.75rem; background-color: var(--card-bg); padding: 1rem; border: 2px solid var(--border-color); border-radius: var(--radius-lg); margin-bottom: 0.75rem;">
                <div class="pending-info">
                  <div class="pending-title" style="font-weight: 800; font-size: 1rem; color: var(--text-main);">🎁 ${escapeHTML(rewardName)}</div>
                  <div class="reward-purchase-meta" style="font-size: 0.85rem; color: var(--text-muted);">Initiated by <strong>${escapeHTML(requester)}</strong></div>
                </div>
                <div class="pending-actions" style="display: flex; gap: 0.5rem; width: 100%;">
                  <button class="btn btn-outline btn-sm" style="flex: 1; color: var(--success-green); border-color: var(--success-green); padding: 0.35rem;" onclick="handleVotePurchase('${purchase.id}', 'approved')">✔️ Approve</button>
                  <button class="btn btn-outline btn-sm" style="flex: 1; color: var(--destructive-red); border-color: var(--destructive-red); padding: 0.35rem;" onclick="handleVotePurchase('${purchase.id}', 'rejected')">❌ Reject</button>
                </div>
              </div>
            `;
          }).join('');
        }
      }
      
      updateLeaderboards();
    }
  } catch (err) {
    console.error("Failed to render rewards dashboard:", err);
  }
}

function toggleRewardType(type) {
  const assigneeGroup = document.getElementById("reward-assignee-group");
  const assigneeSelect = document.getElementById("reward-assignee-input");
  
  if (type === "cooperative") {
    if (assigneeGroup) assigneeGroup.style.display = "none";
    if (assigneeSelect) {
      assigneeSelect.value = "";
      assigneeSelect.required = false;
    }
  } else {
    if (assigneeGroup) assigneeGroup.style.display = "block";
  }
}

async function openAddRewardModal() {
  document.getElementById("reward-modal-title").innerText = "Create New Reward";
  document.getElementById("reward-id-input").value = "";
  document.getElementById("reward-name-input").value = "";
  document.getElementById("reward-description-input").value = "";
  document.getElementById("reward-cost-input").value = "";
  document.getElementById("reward-type-input").value = "individual";
  toggleRewardType("individual");
  
  const select = document.getElementById("reward-assignee-input");
  if (select) {
    select.innerHTML = '<option value="">-- Open to all members --</option>';
    try {
      const members = await apiCall(`/choregroups/${currentSession.choregroup_id}/members`) || [];
      const kids = members.filter(m => m.role === 'user');
      kids.forEach(member => {
        select.innerHTML += `<option value="${member.id}">${escapeHTML(member.username)}</option>`;
      });
    } catch (e) {
      console.error(e);
    }
  }
  
  document.getElementById("reward-modal").classList.add("active");
}

async function openEditRewardModal(rewardId) {
  document.getElementById("reward-modal-title").innerText = "Edit Reward";
  document.getElementById("reward-id-input").value = rewardId;
  
  try {
    const rewards = await apiCall(`/choregroups/${currentSession.choregroup_id}/rewards`) || [];
    const reward = rewards.find(r => r.id === rewardId);
    if (!reward) return;
    
    document.getElementById("reward-name-input").value = reward.name || '';
    document.getElementById("reward-description-input").value = reward.description || '';
    document.getElementById("reward-cost-input").value = reward.cost || '';
    document.getElementById("reward-type-input").value = reward.type || 'individual';
    toggleRewardType(reward.type);
    
    const select = document.getElementById("reward-assignee-input");
    if (select) {
      select.innerHTML = '<option value="">-- Open to all members --</option>';
      const members = await apiCall(`/choregroups/${currentSession.choregroup_id}/members`) || [];
      const kids = members.filter(m => m.role === 'user');
      kids.forEach(member => {
        const selectedAttr = (reward.assigned_to_user_id === member.id) ? 'selected' : '';
        select.innerHTML += `<option value="${member.id}" ${selectedAttr}>${escapeHTML(member.username)}</option>`;
      });
    }
    
    document.getElementById("reward-modal").classList.add("active");
  } catch (err) {
    alert("Could not load reward details: " + err.message);
  }
}

async function handleSaveReward(e) {
  e.preventDefault();
  
  const id = document.getElementById("reward-id-input").value;
  const name = document.getElementById("reward-name-input").value.trim();
  const description = document.getElementById("reward-description-input").value.trim();
  const cost = parseInt(document.getElementById("reward-cost-input").value);
  const type = document.getElementById("reward-type-input").value;
  const assigneeId = document.getElementById("reward-assignee-input").value;
  
  const payload = {
    name: name,
    description: description || null,
    cost: cost,
    type: type,
    assigned_to_user_id: (type === 'individual' && assigneeId) ? assigneeId : null
  };
  
  const choregroupID = currentSession.choregroup_id;
  
  try {
    if (id) {
      await apiCall(`/choregroups/${choregroupID}/rewards/${id}`, "PUT", payload);
      showToast("Reward updated successfully! 🎁");
    } else {
      await apiCall(`/choregroups/${choregroupID}/rewards`, "POST", payload);
      showToast("Reward created successfully! 🎁");
    }
    closeModal("reward-modal");
    renderRewardsDashboard();
  } catch (err) {
    alert("Failed to save reward: " + err.message);
  }
}

async function handleDeleteReward(rewardId) {
  if (!confirm("Are you sure you want to delete this reward? This cannot be undone.")) return;
  
  const choregroupID = currentSession.choregroup_id;
  try {
    await apiCall(`/choregroups/${choregroupID}/rewards/${rewardId}`, "DELETE");
    showToast("Reward deleted. 🗑️");
    renderRewardsDashboard();
  } catch (err) {
    alert("Failed to delete reward: " + err.message);
  }
}

async function handleBuyReward(rewardId, rewardName) {
  if (!confirm(`Are you sure you want to purchase "${rewardName}"?`)) return;
  
  const choregroupID = currentSession.choregroup_id;
  try {
    await apiCall(`/choregroups/${choregroupID}/purchases`, "POST", { reward_id: rewardId });
    showToast("Purchase initiated! 🛒");
    renderRewardsDashboard();
  } catch (err) {
    alert("Purchase failed: " + err.message);
  }
}

async function handleVotePurchase(purchaseId, vote) {
  const choregroupID = currentSession.choregroup_id;
  try {
    await apiCall(`/choregroups/${choregroupID}/purchases/${purchaseId}/approvals`, "POST", { vote: vote });
    showToast(`Vote submitted: ${vote}!`);
    renderRewardsDashboard();
  } catch (err) {
    alert("Failed to submit vote: " + err.message);
  }
}

async function handleFulfillPurchase(purchaseId) {
  const choregroupID = currentSession.choregroup_id;
  try {
    await apiCall(`/choregroups/${choregroupID}/purchases/${purchaseId}/status`, "PUT", { status: "fulfilled" });
    showToast("Reward marked as fulfilled! 📦");
    renderRewardsDashboard();
  } catch (err) {
    alert("Failed to fulfill purchase: " + err.message);
  }
}

async function handleCancelPurchase(purchaseId) {
  if (!confirm("Are you sure you want to cancel this pending reward? The points will be refunded.")) return;
  const choregroupID = currentSession.choregroup_id;
  try {
    await apiCall(`/choregroups/${choregroupID}/purchases/${purchaseId}`, "DELETE");
    showToast("Purchase cancelled and points refunded!");
    renderRewardsDashboard();
  } catch (err) {
    alert("Failed to cancel purchase: " + err.message);
  }
}

function startTaskTimerTicker() {
  setInterval(() => {
    document.querySelectorAll('[data-expires]').forEach(el => {
      const expiresStr = el.getAttribute('data-expires');
      if (!expiresStr) return;
      const isMandatory = el.getAttribute('data-mandatory') === 'true';
      const expires = new Date(expiresStr);
      const remaining = expires - Date.now();
      if (remaining <= 0) {
        if (isMandatory) {
          el.innerHTML = "⚠️ Overdue";
          el.style.color = "var(--destructive-red)";
        } else {
          el.innerHTML = "🚨 Expired";
          el.style.color = "#e03131";
        }
      } else {
        const diffMins = Math.floor(remaining / 60000);
        const hours = Math.floor(diffMins / 60);
        const mins = diffMins % 60;
        const secs = Math.floor((remaining % 60000) / 1000);
        el.innerHTML = `⏳ ${hours > 0 ? hours + 'h ' : ''}${mins}m ${secs}s left`;
      }
    });
  }, 1000);
}

async function handleExtendTask(taskId) {
  const groupId = currentSession.choregroup_id;
  try {
    const newExpiresAt = new Date(Date.now() + 60 * 60000).toISOString();
    const task = activeTasks.find(t => t.id === taskId);
    if (!task) {
      alert("Failed to find task to extend.");
      return;
    }
    
    await apiCall(`/choregroups/${groupId}/tasks/${taskId}`, "PUT", {
      assigned_to_user_id: task.assigned_to_user_id || null,
      points_reward: task.points_reward,
      title: task.title,
      type: task.type,
      is_mandatory: task.is_mandatory,
      expires_at: newExpiresAt
    });
    
    showToast("Task extended by 1 hour! ⏳");
    renderParentDashboard();
    updateLeaderboards();
  } catch (err) {
    alert("Failed to extend task: " + err.message);
  }
}

/* ==========================================================
   KID PIN & DELEGATED AUTHENTICATION LOGIC
   ========================================================== */

let pinLoginChoreGroupID = null;
let pinLoginUserID = null;
let pinLoginUsername = null;
let pinLoginAccumulated = "";

async function handleFindHousehold(e) {
  e.preventDefault();
  const familyName = document.getElementById("pin-lookup-family-name").value.trim();
  if (!familyName) return;
  
  try {
    const data = await apiCall(`/choregroups/lookup?name=${encodeURIComponent(familyName)}`);
    pinLoginChoreGroupID = data.id;
    
    const avatarList = document.getElementById("kid-avatar-list");
    avatarList.innerHTML = "";
    
    if (!data.members || data.members.length === 0) {
      avatarList.innerHTML = `<p style="grid-column: span 3; text-align: center; color: var(--text-muted);">No kid profiles found. Ask a parent to add you!</p>`;
    } else {
      const emojis = ["🐱", "🐶", "🦁", "🐰", "🐼", "🦊", "🐨", "🐸", "🐵"];
      data.members.forEach((kid, idx) => {
        const emoji = emojis[idx % emojis.length];
        const div = document.createElement("div");
        div.className = "avatar-item";
        div.style = "display: flex; flex-direction: column; align-items: center; cursor: pointer; padding: 0.5rem; border-radius: 8px; border: 2px solid transparent; transition: all 0.2s;";
        div.innerHTML = `
          <div style="font-size: 3rem; background: var(--bg-card); width: 80px; height: 80px; display: flex; align-items: center; justify-content: center; border-radius: 50%; border: 3px solid var(--primary); margin-bottom: 0.5rem; transition: transform 0.2s;">${emoji}</div>
          <span style="font-weight: 800; color: var(--text-main); font-size: 1.05rem;">${kid.username}</span>
        `;
        div.addEventListener("mouseenter", () => {
          div.querySelector("div").style.transform = "scale(1.1)";
        });
        div.addEventListener("mouseleave", () => {
          div.querySelector("div").style.transform = "scale(1)";
        });
        div.addEventListener("click", () => {
          selectKidForPinEntry(kid.id, kid.username);
        });
        avatarList.appendChild(div);
      });
    }
    
    toggleAuthForm("pin-avatar");
  } catch (err) {
    alert("Family not found! Check spelling and try again.");
  }
}

function selectKidForPinEntry(userID, username) {
  pinLoginUserID = userID;
  pinLoginUsername = username;
  pinLoginAccumulated = "";
  document.getElementById("pin-input-field").value = "";
  document.getElementById("pin-entry-title").innerText = `PIN for ${username}`;
  toggleAuthForm("pin-pad");
}

function pressPinNumber(num) {
  if (pinLoginAccumulated.length < 4) {
    pinLoginAccumulated += num;
    document.getElementById("pin-input-field").value = "•".repeat(pinLoginAccumulated.length);
  }
}

function clearPinNumber() {
  pinLoginAccumulated = "";
  document.getElementById("pin-input-field").value = "";
}

function goBackToAvatars() {
  toggleAuthForm("pin-avatar");
}

async function submitPinLogin() {
  if (pinLoginAccumulated.length < 4) {
    alert("Please enter a 4-digit PIN!");
    return;
  }
  
  try {
    const res = await apiCall("/login/pin", "POST", {
      user_id: pinLoginUserID,
      pin: pinLoginAccumulated
    });
    
    setSession({
      id: res.user_id,
      username: pinLoginUsername,
      role: res.role,
      choregroup_id: res.choregroup_id,
      choregroup_name: "Kid Space"
    });
    closeModal("auth-modal");
    showToast(`Welcome ${pinLoginUsername}!`);
  } catch (err) {
    alert("Incorrect PIN! Try again.");
    clearPinNumber();
  }
}

async function generateKidAccessLink(userID) {
  try {
    const groupId = currentSession.choregroup_id;
    const res = await apiCall(`/choregroups/${groupId}/members/${userID}/login-link`, "POST");
    const link = `${window.location.origin}${window.location.pathname}?login_token=${res.token}`;

    document.getElementById("generated-login-link-input").value = link;
    const modal = document.getElementById("login-link-modal");
    if (modal) modal.classList.add("active");
  } catch (err) {
    alert("Failed to generate access link: " + err.message);
  }
}

function copyToClipboardFallback(text) {
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.style.position = "fixed";
  ta.style.left = "-9999px";
  document.body.appendChild(ta);
  ta.select();
  ta.setSelectionRange(0, 99999);
  document.execCommand("copy");
  document.body.removeChild(ta);
}

function copyDelegatedLink() {
  const input = document.getElementById("generated-login-link-input");
  const link = input.value;
  input.select();
  input.setSelectionRange(0, 99999);

  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(link).then(() => {
      showToast("📋 Link copied to clipboard!");
    }).catch(() => {
      copyToClipboardFallback(link);
      showToast("📋 Link copied to clipboard!");
    });
  } else {
    copyToClipboardFallback(link);
    showToast("📋 Link copied to clipboard!");
  }
  closeModal("login-link-modal");
}

function getShareLink() {
  return document.getElementById("generated-login-link-input").value || "";
}

function shareViaWhatsApp() {
  const link = getShareLink();
  if (!link) return;
  const text = encodeURIComponent("Tap this link to log in to ChoreCraft! \ud83c\udfe0\u2728\n" + link);
  window.open(`https://wa.me/?text=${text}`, "_blank");
  closeModal("login-link-modal");
}

function shareViaSMS() {
  const link = getShareLink();
  if (!link) return;
  const text = encodeURIComponent("Log in to ChoreCraft: " + link);
  window.open(`sms:?body=${text}`, "_self");
  closeModal("login-link-modal");
}

function shareViaEmail() {
  const link = getShareLink();
  if (!link) return;
  const subject = encodeURIComponent("ChoreCraft - Kid Access Link");
  const body = encodeURIComponent("Tap this link to log in to ChoreCraft! \ud83c\udfe0\u2728\n\n" + link + "\n\nThis link is valid for 5 minutes.");
  window.open(`mailto:?subject=${subject}&body=${body}`, "_self");
  closeModal("login-link-modal");
}

async function handleDelegatedLogin(token) {
  try {
    const res = await apiCall("/login/delegated", "POST", { token: token });
    setSession({
      id: res.user_id,
      username: res.username || "Kid Space",
      role: res.role,
      choregroup_id: res.choregroup_id,
      choregroup_name: "Kid Space"
    });
    showToast("🔑 Logged in automatically!");
  } catch (err) {
    showToast("❌ Auto-login link expired or invalid!");
  }
}


// Bind handlers to the window object to support ES modules type="module" in index.html
window.handleLogoClick = handleLogoClick;
window.openAuthModal = openAuthModal;
window.logout = logout;
window.switchParentTab = switchParentTab;
window.switchKidTab = switchKidTab;
window.closeModal = closeModal;
window.toggleAuthForm = toggleAuthForm;
window.openAddChoreModal = openAddChoreModal;
window.openAddUserModal = openAddUserModal;
window.openAddRewardModal = openAddRewardModal;
window.handleParentSignUp = handleParentSignUp;
window.handleLogin = handleLogin;
window.handleSaveChore = handleSaveChore;
window.handleSaveReward = handleSaveReward;
window.handleAddUserSubmit = handleAddUserSubmit;
window.toggleChoreType = toggleChoreType;
window.toggleMandatoryChore = toggleMandatoryChore;
window.toggleRewardType = toggleRewardType;
window.handleExtendTask = handleExtendTask;
window.openEditChoreModal = openEditChoreModal;
window.deleteChore = deleteChore;
window.approveSubmission = approveSubmission;
window.rejectSubmission = rejectSubmission;
window.openEditRewardModal = openEditRewardModal;
window.handleDeleteReward = handleDeleteReward;
window.handleBuyReward = handleBuyReward;
window.handleVotePurchase = handleVotePurchase;
window.handleFulfillPurchase = handleFulfillPurchase;
window.handleCancelPurchase = handleCancelPurchase;
window.submitChoreForApproval = submitChoreForApproval;
window.simulateHeroChoreCompletion = simulateHeroChoreCompletion;
window.switchParentRewardSubTab = switchParentRewardSubTab;
window.handleFindHousehold = handleFindHousehold;
window.pressPinNumber = pressPinNumber;
window.clearPinNumber = clearPinNumber;
window.goBackToAvatars = goBackToAvatars;
window.submitPinLogin = submitPinLogin;
window.generateKidAccessLink = generateKidAccessLink;
window.copyDelegatedLink = copyDelegatedLink;
window.shareViaWhatsApp = shareViaWhatsApp;
window.shareViaSMS = shareViaSMS;
window.shareViaEmail = shareViaEmail;

