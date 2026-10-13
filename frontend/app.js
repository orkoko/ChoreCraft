/* ==========================================================================
   CHORECRAFT STATE MANAGEMENT & EXCLUSIVE LIVE API ENGINE
   ========================================================================== */

// Global session
let currentSession = null; // Contains user_id, username, role, choregroup_id, choregroup_name
let activeTasks = [];

const ALL_AVATAR_FRAME_IDS = ["none", "golden_sparkle", "fire_flame", "cosmic_nebula", "emerald_nature", "rainbow_wave", "royal_crown", "neon_cyber"];
const ALL_POINTS_THEME_IDS = ["default", "autumn_leaves", "golden_stars", "cherry_blossom", "cosmic_galaxy", "emerald_nature", "candy_party", "fire_dragon"];

// Resolve API base URL dynamically based on page loading domain
function getApiBase() {
  if (window.location.protocol === 'file:') {
    return 'http://localhost:8080/api';
  }
  return '/api';
}

function getRelevantEmoji(title) {
  if (!title) return "📋";
  const lower = title.toLowerCase();

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

function resolveIconUrl(url) {
  if (!url) return '';
  if (url.startsWith('http://') || url.startsWith('https://') || url.startsWith('data:')) {
    return url;
  }
  if (window.location.protocol === 'file:') {
    return `http://localhost:8080${url.startsWith('/') ? '' : '/'}${url}`;
  }
  return url;
}

// Render icon element (custom image URL if provided, otherwise fallback emoji)
function renderItemIcon(item, fallbackEmoji, title) {
  if (item && item.icon_url) {
    const src = resolveIconUrl(item.icon_url);
    const safeEmoji = escapeHTML(fallbackEmoji || "📋");
    const rawTitle = title || (item.title ? parseChoreTitle(item.title).title : (item.name ? parseChoreTitle(item.name).title : ""));
    const safeTitle = escapeHTML(rawTitle);
    const safeSrc = escapeHTML(src);
    return `<img src="${safeSrc}" class="card-icon-img zoomable-photo" alt="${safeTitle}" data-title="${safeTitle}" onerror="this.onerror=null; this.parentElement.innerHTML='${safeEmoji}';" style="cursor: pointer;" onclick="event.stopPropagation(); openImageModal('${safeSrc}', '${safeTitle}');" title="${safeTitle ? `Click to enlarge: ${safeTitle}` : 'Click to enlarge photo'}">`;
  }
  return fallbackEmoji || "📋";
}

let imageModalOpenTimestamp = 0;

function openImageModal(src, title) {
  // Only enlarge actual photos, never emoji icons
  if (!src || typeof src !== "string" || !src.trim()) return;

  const modal = document.getElementById("image-zoom-modal");
  const img = document.getElementById("image-zoom-modal-img");
  const titleEl = document.getElementById("image-zoom-modal-title");
  const spacerEl = document.getElementById("image-zoom-modal-spacer");

  if (!modal) return;

  // Extract clean display title, never show generic "photo"
  let displayTitle = "";
  if (title && typeof title === "string") {
    const trimmed = title.trim();
    const lower = trimmed.toLowerCase();
    if (lower && lower !== "photo" && lower !== "enlarged photo" && lower !== "icon preview" && lower !== "ai item preview") {
      const parsed = parseChoreTitle(trimmed);
      displayTitle = parsed.title || trimmed;
    }
  }

  if (titleEl) {
    if (displayTitle) {
      titleEl.textContent = displayTitle;
      titleEl.style.display = "block";
      if (spacerEl) spacerEl.style.display = "block";
    } else {
      titleEl.textContent = "";
      titleEl.style.display = "none";
      if (spacerEl) spacerEl.style.display = "none";
    }
  }

  if (img) {
    img.src = src;
    img.alt = displayTitle || "Enlarged Photo";
    img.style.display = "block";
  }

  imageModalOpenTimestamp = Date.now();
  modal.classList.add("active");
}

function closeImageModal() {
  if (Date.now() - imageModalOpenTimestamp < 150) return;
  const modal = document.getElementById("image-zoom-modal");
  if (modal) {
    modal.classList.remove("active");
  }
}

// Prevent text and drag selection on chore cards
document.addEventListener("selectstart", function(e) {
  if (e.target.closest(".chore-card, .reward-card, .pending-item, .submission-item")) {
    e.preventDefault();
  }
});

document.addEventListener("mousedown", function(e) {
  if (e.target.closest(".chore-card, .reward-card, .pending-item, .submission-item")) {
    if (!e.target.closest("button, input, select, textarea, a")) {
      e.preventDefault();
    }
  }
});

function handleChoreTap(e) {
  // 1. If modal is active, any click/tap anywhere on screen closes it (if opened > 150ms ago)
  const modal = document.getElementById("image-zoom-modal");
  if (modal && modal.classList.contains("active")) {
    if (Date.now() - imageModalOpenTimestamp > 150) {
      closeImageModal();
      e.stopPropagation();
      e.preventDefault();
    }
    return;
  }

  // 2. Ignore clicks on action buttons or controls (e.g. Approve, Reject, Edit, Delete, Submit buttons)
  const isActionButton = e.target.closest("button, input, select, textarea, a, .card-controls, .sub-actions, .btn-action-submit");
  if (isActionButton) {
    return; // Allow button clicks to process normally
  }

  // 3. Check if clicked item or container has a real photo URL (only photos, never emojis)
  const photoContainer = e.target.closest("[data-photo-url]");
  if (photoContainer && photoContainer.dataset.photoUrl && photoContainer.dataset.photoUrl.trim()) {
    const src = photoContainer.dataset.photoUrl.trim();
    let title = photoContainer.dataset.choreTitle || photoContainer.dataset.title || "";
    if (!title || title.toLowerCase() === "photo") {
      const h3 = photoContainer.querySelector("h3, .sub-chore span:nth-child(2)");
      if (h3 && h3.textContent) {
        title = parseChoreTitle(h3.textContent).title;
      }
    }
    e.stopPropagation();
    e.preventDefault();
    openImageModal(src, title);
    return;
  }

  // 4. Check if clicked target is a zoomable photo image itself
  if (e.target && e.target.tagName === "IMG" && e.target.classList.contains("zoomable-photo")) {
    const src = e.target.getAttribute("src");
    if (src) {
      e.stopPropagation();
      e.preventDefault();
      let title = e.target.getAttribute("data-title") || "";
      if (!title || title.toLowerCase() === "photo") {
        const alt = e.target.getAttribute("alt") || "";
        if (alt && alt.toLowerCase() !== "photo" && alt.toLowerCase() !== "enlarged photo") {
          title = alt;
        }
      }
      if (!title || title.toLowerCase() === "photo") {
        const container = e.target.closest("[data-chore-title], [data-title]");
        if (container) {
          title = container.dataset.choreTitle || container.dataset.title || "";
        }
      }
      if (!title || title.toLowerCase() === "photo") {
        const card = e.target.closest(".chore-card, .reward-card, .submission-item, .pending-item");
        if (card) {
          const h3 = card.querySelector("h3, .sub-chore span:nth-child(2)");
          if (h3 && h3.textContent) {
            title = parseChoreTitle(h3.textContent).title;
          }
        }
      }
      openImageModal(src, title);
      return;
    }
  }
}

// Global click and key listeners for desktop & mobile
document.addEventListener("click", handleChoreTap, true);
document.addEventListener("keydown", function(e) {
  if (e.key === "Escape") {
    closeImageModal();
  }
});

// Handle icon file selection and upload
async function handleIconUpload(event, type) {
  const file = event.target.files && event.target.files[0];
  if (!file) return;

  const formData = new FormData();
  formData.append("image", file);

  try {
    const groupId = currentSession ? currentSession.choregroup_id : null;
    const url = groupId ? `/choregroups/${groupId}/upload` : '/upload';
    
    const apiBase = getApiBase();
    const response = await fetch(`${apiBase}${url}`, {
      method: "POST",
      body: formData,
      credentials: "include"
    });

    if (!response.ok) {
      const errText = await response.text();
      throw new Error(errText || "Upload failed");
    }

    const data = await response.json();
    if (data && data.icon_url) {
      document.getElementById(`${type}-icon-url-input`).value = data.icon_url;
      const previewImg = document.getElementById(`${type}-icon-preview-img`);
      if (previewImg) previewImg.src = resolveIconUrl(data.icon_url);
      const previewBox = document.getElementById(`${type}-icon-preview-container`);
      if (previewBox) previewBox.style.display = "flex";
      showToast("Custom icon uploaded! 🖼️");
    }
  } catch (err) {
    alert("Failed to upload icon: " + err.message);
  } finally {
    if (event.target) event.target.value = "";
  }
}

// Clear custom icon upload selection
function clearIconUpload(type) {
  const fileInput = document.getElementById(`${type}-icon-file-input`);
  const urlInput = document.getElementById(`${type}-icon-url-input`);
  const previewImg = document.getElementById(`${type}-icon-preview-img`);
  const previewBox = document.getElementById(`${type}-icon-preview-container`);

  if (fileInput) fileInput.value = "";
  if (urlInput) urlInput.value = "";
  if (previewImg) previewImg.src = "";
  if (previewBox) previewBox.style.display = "none";
}

// Camera photo capture logic
let activeCameraStream = null;
let cameraTargetType = null;
let currentCameraFacingMode = "environment";

async function startCameraStream(facingMode = "environment") {
  const video = document.getElementById("camera-video");
  const errorMsg = document.getElementById("camera-error-msg");

  if (activeCameraStream) {
    activeCameraStream.getTracks().forEach(track => track.stop());
    activeCameraStream = null;
  }

  if (errorMsg) errorMsg.style.display = "none";
  if (video) video.style.display = "block";

  currentCameraFacingMode = facingMode;

  const facingConstraints = [
    { video: { facingMode: { exact: facingMode }, width: { ideal: 1280 }, height: { ideal: 720 } } },
    { video: { facingMode: { ideal: facingMode }, width: { ideal: 1280 }, height: { ideal: 720 } } },
    { video: { facingMode: facingMode } },
    { video: true }
  ];

  for (const constraints of facingConstraints) {
    try {
      activeCameraStream = await navigator.mediaDevices.getUserMedia(constraints);
      if (video) {
        video.srcObject = activeCameraStream;
      }
      return true;
    } catch (err) {
      console.warn("Camera constraint failed, trying fallback:", err);
    }
  }

  if (errorMsg) errorMsg.style.display = "flex";
  if (video) video.style.display = "none";
  return false;
}

async function openCameraModal(type) {
  cameraTargetType = type;
  const modal = document.getElementById("camera-modal");

  await startCameraStream("environment");

  if (modal) modal.classList.add("active");
}

function closeCameraModal() {
  if (activeCameraStream) {
    activeCameraStream.getTracks().forEach(track => track.stop());
    activeCameraStream = null;
  }
  const video = document.getElementById("camera-video");
  if (video) video.srcObject = null;

  const modal = document.getElementById("camera-modal");
  if (modal) modal.classList.remove("active");
}

async function snapCameraPhoto() {
  const video = document.getElementById("camera-video");
  const canvas = document.getElementById("camera-canvas");
  if (!video || !canvas || !cameraTargetType) return;

  const context = canvas.getContext("2d");
  const srcWidth = video.videoWidth || 640;
  const srcHeight = video.videoHeight || 480;

  // Resize to max 512px on longest side to reduce AI token usage
  const MAX_DIM = 512;
  let destWidth = srcWidth;
  let destHeight = srcHeight;
  if (srcWidth > MAX_DIM || srcHeight > MAX_DIM) {
    const scale = MAX_DIM / Math.max(srcWidth, srcHeight);
    destWidth = Math.round(srcWidth * scale);
    destHeight = Math.round(srcHeight * scale);
  }

  canvas.width = destWidth;
  canvas.height = destHeight;
  context.drawImage(video, 0, 0, destWidth, destHeight);

  // Export as JPEG at 70% quality (much smaller than PNG, fewer AI tokens)
  canvas.toBlob(async (blob) => {
    if (!blob) {
      alert("Failed to capture photo frame");
      return;
    }

    const file = new File([blob], `photo_${Date.now()}.jpg`, { type: "image/jpeg" });

    if (cameraTargetType && cameraTargetType.startsWith("ai-")) {
      const mode = cameraTargetType.replace("ai-", "") || "chore";
      closeCameraModal();
      await processAiImageFile(file, mode);
      return;
    }

    const formData = new FormData();
    formData.append("image", file);

    try {
      const groupId = currentSession ? currentSession.choregroup_id : null;
      const url = groupId ? `/choregroups/${groupId}/upload` : '/upload';
      
      const apiBase = getApiBase();
      const response = await fetch(`${apiBase}${url}`, {
        method: "POST",
        body: formData,
        credentials: "include"
      });

      if (!response.ok) {
        const errText = await response.text();
        throw new Error(errText || "Upload failed");
      }

      const data = await response.json();
      if (data && data.icon_url) {
        const type = cameraTargetType;
        document.getElementById(`${type}-icon-url-input`).value = data.icon_url;
        const previewImg = document.getElementById(`${type}-icon-preview-img`);
        if (previewImg) previewImg.src = resolveIconUrl(data.icon_url);
        const previewBox = document.getElementById(`${type}-icon-preview-container`);
        if (previewBox) previewBox.style.display = "flex";
        showToast("Photo captured and set as icon! 📸");
      }
    } catch (err) {
      alert("Failed to upload captured photo: " + err.message);
    } finally {
      closeCameraModal();
    }
  }, "image/jpeg", 0.70);
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
  
  const isAuto = (val === "auto" || val === "" || val === 0 || val === "0" || val === null || val === undefined);
  hiddenInput.value = isAuto ? "auto" : val;
  options.forEach(opt => {
    const optVal = opt.getAttribute("data-value");
    if ((isAuto && optVal === "auto") || (!isAuto && optVal === String(val))) {
      opt.classList.add("selected");
    } else {
      opt.classList.remove("selected");
    }
  });
  
  if (isAuto) {
    trigger.innerHTML = `
      <span>✨ AI Estimate <span class="points-coin">🤖</span></span>
      <span class="arrow">▼</span>
    `;
    return;
  }
  
  const isCooperative = container.classList.contains("coop-theme");
  const coinClass = isCooperative ? "points-coin coop-theme" : "points-coin";
  const coinEmoji = isCooperative ? "🐱🐱" : "🐱";
  trigger.innerHTML = `
    <span>+${val} <span class="${coinClass}">${coinEmoji}</span></span>
    <span class="arrow">▼</span>
  `;
}

/* ==========================================
   THEME MANAGEMENT (DARK / LIGHT MODE)
   ========================================== */
function initTheme() {
  const savedTheme = localStorage.getItem('chorecraft_theme');
  const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
  const theme = savedTheme || (prefersDark ? 'dark' : 'light');
  setTheme(theme, false);

  if (window.matchMedia) {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
      if (!localStorage.getItem('chorecraft_theme')) {
        setTheme(e.matches ? 'dark' : 'light', false);
      }
    });
  }
}

function setTheme(theme, save = true) {
  document.documentElement.setAttribute('data-theme', theme);
  if (save) {
    localStorage.setItem('chorecraft_theme', theme);
  }
  updateThemeControls(theme);

  // Detect if running as installed Home Screen PWA
  const isStandalone = (window.matchMedia && window.matchMedia('(display-mode: standalone)').matches) || window.navigator.standalone === true;

  // In standalone PWA, native status bar is black, so keep icons white
  const isDark = theme === 'dark';
  const color = (isStandalone || isDark) ? '#000000' : '#ffffff';
  const scheme = (isStandalone || isDark) ? 'dark' : 'light';

  document.documentElement.style.colorScheme = isDark ? 'dark' : 'light';

  // Remove and recreate meta tags to guarantee Chrome Android picks up dynamic changes
  document.querySelectorAll('meta[name="theme-color"]').forEach(el => el.remove());
  document.querySelectorAll('meta[name="color-scheme"]').forEach(el => el.remove());

  const schemeMeta = document.createElement('meta');
  schemeMeta.name = 'color-scheme';
  schemeMeta.id = 'meta-color-scheme';
  schemeMeta.content = scheme;
  document.head.appendChild(schemeMeta);

  const themeMeta = document.createElement('meta');
  themeMeta.name = 'theme-color';
  themeMeta.id = 'meta-theme-color';
  themeMeta.content = color;
  document.head.appendChild(themeMeta);

  // Also update iOS status bar style
  const appleMeta = document.querySelector('meta[name="apple-mobile-web-app-status-bar-style"]');
  if (appleMeta) {
    appleMeta.setAttribute('content', (isStandalone || isDark) ? 'black' : 'default');
  }
}

function toggleTheme() {
  const currentTheme = document.documentElement.getAttribute('data-theme') || 'light';
  const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
  setTheme(newTheme, true);
}

function handleThemeToggleClick(event) {
  if (event) {
    event.stopPropagation();
  }
  toggleTheme();
}

function handleThemeSwitchChange(event) {
  if (event) {
    event.stopPropagation();
  }
  const isDark = event.target.checked;
  setTheme(isDark ? 'dark' : 'light', true);
}

function updateThemeControls(theme) {
  const isDark = theme === 'dark';
  const checkbox = document.getElementById('theme-switch-checkbox');
  if (checkbox) {
    checkbox.checked = isDark;
  }
  const icon = document.getElementById('dropdown-theme-icon');
  if (icon) {
    icon.textContent = isDark ? '🌙' : '☀️';
  }
}

/* ==========================================
   APP INITIALIZATION & ROUTING
   ========================================== */
document.addEventListener("DOMContentLoaded", async () => {
  initTheme();
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
    if (currentSession && currentSession.role === "admin") {
      currentSession.unlocked_avatar_frames = [...ALL_AVATAR_FRAME_IDS];
      currentSession.unlocked_backgrounds = [...ALL_POINTS_THEME_IDS];
    }
    
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
        if (resp.ok) {
          try {
            const statsData = await resp.json();
            syncSessionFromStats(statsData);
          } catch (e) {
            console.error("Failed to parse initial statistics:", e);
          }
        }
        connectSSE();
        restoreSessionView();
        registerPushNotifications();
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
  
  const forms = [signupForm, loginForm];
  forms.forEach(f => { if (f) f.style.display = "none"; });
  
  if (state === "signup" && signupForm) signupForm.style.display = "block";
  if (state === "login" && loginForm) loginForm.style.display = "block";
}

function closeModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.classList.remove("active");
  }
}

function openModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.classList.add("active");
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
    if (event.data === "refresh" || event.data === "new_task") {
      console.log(`SSE ${event.data} event received. Reloading dashboards...`);
      
      if (event.data === "new_task" && currentSession && currentSession.role === "user") {
        if (window.Notification && Notification.permission === "granted") {
          try {
            new Notification("ChoreCraft", {
              body: "🔔 New chores are waiting for you! ✨",
              icon: "icon.svg"
            });
          } catch (e) {
            console.warn("Failed to trigger Notification API:", e);
          }
        }
        showToast("🔔 New chores have been added!");
      }

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
    
    const data = await response.json();
    if (endpoint.endsWith("/statistics") && data && data.users) {
      try {
        syncSessionFromStats(data);
      } catch (e) {
        console.error("Failed to sync session from stats:", e);
      }
    }
    return data;
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
  
  const familyName = "";
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
      choregroup_name: res.role === "admin" ? "My Household" : "User Space",
      profile_image_url: res.profile_image_url || null,
      avatar_frame: res.avatar_frame || "none",
      unlocked_avatar_frames: res.unlocked_avatar_frames || ["none"]
    });
    closeModal("auth-modal");
    showToast(`Welcome back ${res.username || username}!`);
  } catch (err) {
    alert("Invalid credentials on server!");
  }
}

function setSession(userObj) {
  const isAdmin = userObj.role === "admin";
  currentSession = {
    user_id: userObj.id || userObj.user_id,
    id: userObj.id || userObj.user_id,
    username: userObj.username,
    role: userObj.role,
    choregroup_id: userObj.choregroup_id,
    choregroup_name: userObj.choregroup_name,
    profile_image_url: userObj.profile_image_url || null,
    avatar_frame: userObj.avatar_frame || "none",
    unlocked_avatar_frames: isAdmin ? [...ALL_AVATAR_FRAME_IDS] : (userObj.unlocked_avatar_frames || ["none"]),
    points: userObj.points || 0,
    cooperative_points: userObj.cooperative_points || 0,
    points_background: userObj.points_background || "default",
    unlocked_backgrounds: isAdmin ? [...ALL_POINTS_THEME_IDS] : (userObj.unlocked_backgrounds || ["default", "autumn_leaves", "golden_stars"]),
    ai_instructions: userObj.ai_instructions || ""
  };
  localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
  connectSSE();
  restoreSessionView();
  updateHeaderAuthBtn();
  updateLeaderboards();
  registerPushNotifications();
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
    fetchAndSyncDailyBonus();
  }
  
  // Restore tab from URL hash (preserves state across refreshes)
  navigateFromHash();
  checkAndShowPushBanners();
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
  hidePushBanners();
  showToast("Logged out successfully.");
}

function syncSessionFromStats(statsData) {
  if (!currentSession || !statsData || !statsData.users) return;
  const currentUid = currentSession.user_id || currentSession.id;
  const me = statsData.users.find(u => u.id === currentUid);
  if (me) {
    currentSession.notifications_viewed = me.notifications_viewed;
    let changed = false;
    if (me.profile_image_url !== undefined && me.profile_image_url !== currentSession.profile_image_url) {
      currentSession.profile_image_url = me.profile_image_url;
      changed = true;
    }
    if (me.avatar_frame !== undefined && me.avatar_frame !== currentSession.avatar_frame) {
      currentSession.avatar_frame = me.avatar_frame;
      changed = true;
    }
    if (me.unlocked_avatar_frames !== undefined) {
      currentSession.unlocked_avatar_frames = (currentSession.role === "admin") ? [...ALL_AVATAR_FRAME_IDS] : me.unlocked_avatar_frames;
    }
    if (me.points_background !== undefined && me.points_background !== currentSession.points_background) {
      currentSession.points_background = me.points_background;
      changed = true;
    }
    if (me.unlocked_backgrounds !== undefined) {
      currentSession.unlocked_backgrounds = (currentSession.role === "admin") ? [...ALL_POINTS_THEME_IDS] : me.unlocked_backgrounds;
    }
    localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
    if (changed) {
      updateHeaderAuthBtn();
    }
    
    // Check if we need to hide the badge
    if (currentSession.notifications_viewed) {
      const badge = document.getElementById('notification-badge');
      if (badge) {
        badge.style.display = 'none';
      }
    }
  }
}

window.shouldShowNotificationBadge = () => {
  return currentSession && !currentSession.notifications_viewed;
};

window.markNotificationsRead = async () => {
  if (!currentSession) return;
  
  // Hide the badge locally
  const badge = document.getElementById('notification-badge');
  if (badge) {
    badge.style.display = 'none';
  }
  
  // Update current session
  currentSession.notifications_viewed = true;
  localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
  
  try {
    await apiCall(`/choregroups/${currentSession.choregroup_id}/notifications/view`, "POST");
  } catch (err) {
    console.error("Failed to sync notification read state:", err);
  }
};

function updateHeaderWheelBadge(spinsAvailable = 0) {
  const wheelBtn = document.getElementById("header-wheel-btn");
  const badge = document.getElementById("header-wheel-badge");
  if (!wheelBtn) return;

  if (currentSession && currentSession.role === "user") {
    wheelBtn.style.display = "inline-flex";
    if (spinsAvailable > 0) {
      wheelBtn.classList.add("has-spins");
      if (badge) {
        badge.textContent = spinsAvailable;
        badge.style.display = "flex";
      }
    } else {
      wheelBtn.classList.remove("has-spins");
      if (badge) {
        badge.style.display = "none";
      }
    }
  } else {
    wheelBtn.style.display = "none";
    if (badge) badge.style.display = "none";
  }
}

async function fetchAndSyncDailyBonus() {
  if (!currentSession || currentSession.role !== "user") return null;
  const groupId = currentSession.choregroup_id;
  if (!groupId) return null;
  try {
    const bonusData = await apiCall(`/choregroups/${groupId}/daily-bonus`);
    if (bonusData) {
      currentDailyBonus = bonusData;
      const spins = bonusData.spins_available || 0;
      currentSession.spins_available = spins;
      try {
        localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
      } catch (e) {}
      updateHeaderWheelBadge(spins);
      renderDailyBonusBar(bonusData);
      return bonusData;
    }
  } catch (err) {
    console.warn("Could not fetch daily bonus status:", err);
  }
  return null;
}

function renderAvatarHtml(profileImageUrl, isAdmin = false, customClass = "", avatarFrame = "none") {
  const isImage = profileImageUrl && (profileImageUrl.startsWith("/") || profileImageUrl.startsWith("http"));
  const crownHtml = isAdmin ? '<span class="avatar-crown">👑</span>' : '';
  let inner = '';
  if (isImage) {
    inner = `<span class="member-avatar ${customClass} ${isAdmin ? 'admin-avatar' : ''}"><img src="${escapeHTML(profileImageUrl)}" class="member-avatar-img" alt="Avatar" style="width:100%;height:100%;max-width:100%;max-height:100%;object-fit:cover;border-radius:50%;display:block;" />${crownHtml}</span>`;
  } else if (profileImageUrl && profileImageUrl.trim()) {
    inner = `<span class="member-avatar ${customClass} ${isAdmin ? 'admin-avatar' : ''}"><span class="avatar-emoji">${escapeHTML(profileImageUrl)}</span>${crownHtml}</span>`;
  } else {
    inner = `<span class="member-avatar ${customClass} ${isAdmin ? 'admin-avatar' : ''}">👤${crownHtml}</span>`;
  }
  const frameClass = (avatarFrame && avatarFrame !== 'none') ? `frame-${escapeHTML(avatarFrame)}` : 'frame-none';
  return `<span class="avatar-frame-badge ${frameClass}">${inner}</span>`;
}

function renderMiniAvatar(profileImageUrl, isAdmin = false, avatarFrame = "none") {
  const isImage = profileImageUrl && (profileImageUrl.startsWith("/") || profileImageUrl.startsWith("http"));
  const crownHtml = isAdmin ? '<span class="mini-crown">👑</span>' : '';
  let inner = '';
  if (isImage) {
    inner = `<span class="mini-avatar-wrap ${isAdmin ? 'admin-avatar' : ''}" style="width:100%;height:100%;max-width:100%;max-height:100%;"><img src="${escapeHTML(profileImageUrl)}" class="mini-avatar-img" alt="Avatar" style="width:100%;height:100%;max-width:100%;max-height:100%;object-fit:cover;border-radius:50%;display:block;" />${crownHtml}</span>`;
  } else if (profileImageUrl && profileImageUrl.trim()) {
    inner = `<span class="mini-avatar-wrap ${isAdmin ? 'admin-avatar' : ''}">${escapeHTML(profileImageUrl)}${crownHtml}</span>`;
  } else {
    inner = `<span class="mini-avatar-wrap ${isAdmin ? 'admin-avatar' : ''}">👤${crownHtml}</span>`;
  }
  const frameClass = (avatarFrame && avatarFrame !== 'none') ? `frame-${escapeHTML(avatarFrame)}` : 'frame-none';
  return `<span class="avatar-frame-badge ${frameClass}" style="width:100%;height:100%;max-width:100%;max-height:100%;">${inner}</span>`;
}

function updateHeaderAuthBtn() {
  const container = document.getElementById("auth-status-container");
  if (!container) return;
  
  const headerStats = document.getElementById("header-user-stats");
  const notificationCenter = document.getElementById("header-notification-center");
  const historyCenter = document.getElementById("header-history-center");
  
  if (currentSession) {
    if (notificationCenter) {
      notificationCenter.style.display = "inline-block";
    }
    if (historyCenter) {
      historyCenter.style.display = "inline-block";
    }
    const isKid = currentSession.role === "user";
    if (isKid) {
      const spins = (currentDailyBonus && typeof currentDailyBonus.spins_available === 'number')
        ? currentDailyBonus.spins_available
        : (typeof currentSession.spins_available === 'number' ? currentSession.spins_available : 0);
      updateHeaderWheelBadge(spins);
      if (headerStats) {
        const theme = (currentSession && currentSession.points_background) || "default";
        const decorHtml = getPointsBackgroundDecorHtml(theme);
        const themeClass = theme !== "default" ? `theme-${escapeHTML(theme)}` : "";
        headerStats.innerHTML = `
          <div class="user-coins-pill ${themeClass}" onclick="openPointsBackgroundModal()" title="Points Background Theme (Click to choose)">
            ${decorHtml}
            <div class="coin-chip individual" id="header-coin-chip-individual" title="Personal Coins: ${currentSession.points || 0}">
              <span class="points-coin">🐱</span>
              <span class="coin-amount" id="header-coin-amount-individual">${currentSession.points || 0}</span>
            </div>
            <div class="coin-pill-divider"></div>
            <div class="coin-chip cooperative" id="header-coin-chip-cooperative" title="Cooperative Coins: ${currentSession.cooperative_points || 0}">
              <span class="points-coin coop-theme">🐱🐱</span>
              <span class="coin-amount" id="header-coin-amount-cooperative">${currentSession.cooperative_points || 0}</span>
            </div>
          </div>
        `;
        headerStats.style.display = "flex";
      }
    } else {
      updateHeaderWheelBadge(0);
      if (headerStats) {
        headerStats.style.display = "none";
      }
    }
    const myFrame = (currentSession && currentSession.avatar_frame) || 'none';
    const hasFrameClass = (myFrame && myFrame !== 'none') ? 'has-frame' : '';
    container.innerHTML = `
      <div class="user-dropdown-container" id="header-user-dropdown-container">
        <button class="btn-user-trigger ${hasFrameClass}" id="user-trigger-btn" onclick="toggleUserDropdown(event)" title="${escapeHTML(currentSession.username)}" aria-label="User profile">
          ${renderMiniAvatar(currentSession.profile_image_url, currentSession.role === 'admin', myFrame)}
        </button>
        <div class="user-dropdown-menu" id="user-dropdown-menu">
          <div class="user-dropdown-profile-header">
            <div class="dropdown-avatar-display" onclick="openAvatarModal()" title="Click to change profile picture or frame">
              ${renderAvatarHtml(currentSession.profile_image_url, currentSession.role === 'admin', '', myFrame)}
            </div>
            <div class="user-dropdown-info-text">
              <strong>${escapeHTML(currentSession.username)}</strong>
              <span class="user-role-badge ${currentSession.role === 'admin' ? 'user-role-admin' : 'user-role-standard'}">${currentSession.role === "admin" ? 'Admin' : 'User'}</span>
            </div>
          </div>
          <div class="dropdown-theme-row" onclick="handleThemeToggleClick(event)" title="Toggle Dark Mode">
            <div class="dropdown-theme-label">
              <span id="dropdown-theme-icon">${(document.documentElement.getAttribute('data-theme') === 'dark') ? '🌙' : '☀️'}</span>
              <span>Dark Mode</span>
            </div>
            <label class="switch-toggle" onclick="event.stopPropagation();" style="margin: 0;">
              <input type="checkbox" id="theme-switch-checkbox" ${(document.documentElement.getAttribute('data-theme') === 'dark') ? 'checked' : ''} onchange="handleThemeSwitchChange(event)" />
              <span class="switch-slider"></span>
            </label>
          </div>
          ${currentSession.role === 'admin' ? `
          <div class="user-dropdown-info" style="border-top: 1px solid var(--border-color, #e5e7eb); padding-top: 8px; margin-top: 0;">
            <label for="ai-language-select" style="font-size: 0.8rem; color: var(--text-muted); display: block; margin-bottom: 4px; font-weight: 500;">
              🌐 Household AI Language (Admin)
            </label>
            <select id="ai-language-select" onchange="handleAiLanguageChange(event)" style="width: 100%; padding: 6px 10px; font-size: 0.85rem; border-radius: 6px; border: 1px solid var(--border-color); background-color: var(--input-bg); color: var(--text-main);">
              <option value="he" ${getAiLanguage() === 'he' ? 'selected' : ''}>Hebrew (עברית)</option>
              <option value="en" ${getAiLanguage() === 'en' ? 'selected' : ''}>English</option>
              <option value="es" ${getAiLanguage() === 'es' ? 'selected' : ''}>Spanish (Español)</option>
              <option value="fr" ${getAiLanguage() === 'fr' ? 'selected' : ''}>French (Français)</option>
              <option value="de" ${getAiLanguage() === 'de' ? 'selected' : ''}>German (Deutsch)</option>
            </select>
            <button class="dropdown-action-btn" onclick="openHouseholdAiModal()" style="margin-top: 8px; width: 100%; justify-content: flex-start; padding: 6px 8px; border-radius: 6px;">
              <span style="font-size: 1.05rem; margin-right: 6px;">🤖</span>
              <span>Household AI Rules</span>
            </button>
          </div>
          ` : ''}
          <button class="dropdown-action-btn" onclick="openChangePasswordModal('${currentSession.id || currentSession.user_id}', '${escapeHTML(currentSession.username)}')">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="margin-right: 8px; stroke: var(--primary-blue);">
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
              <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
            </svg>
            Change Password
          </button>
          <button class="dropdown-logout-btn" onclick="logout()">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="margin-right: 8px; stroke: var(--destructive-red);">
              <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path>
              <polyline points="16 17 21 12 16 7"></polyline>
              <line x1="21" y1="12" x2="9" y2="12"></line>
            </svg>
            Log Out
          </button>
        </div>
      </div>
    `;
  } else {
    updateHeaderWheelBadge(0);
    if (notificationCenter) {
      notificationCenter.style.display = "none";
    }
    if (historyCenter) {
      historyCenter.style.display = "none";
    }
    if (headerStats) {
      headerStats.style.display = "none";
    }
    container.innerHTML = `
      <div style="display: flex; align-items: center; gap: 0.5rem;">
        <button class="btn btn-outline btn-sm" onclick="openAuthModal('login')">Log In</button>
        <button class="btn btn-primary btn-sm" onclick="openAuthModal('signup')">Sign Up</button>
      </div>
    `;
  }
}

/* ==========================================================
   PARENT COMMAND CENTER LOGIC (List / Add / Approve / members)
   ========================================================== */

async function renderParentDashboard() {
  if (!currentSession || currentSession.role !== "admin") return;
  
  const groupId = currentSession.choregroup_id;
  
  setTabLoading("parent-chores-content", true);
  
  const choresList = document.getElementById("parent-chores-list");
  const pendingList = document.getElementById("parent-pending-list");
  
  let pendingSubmissions = [];
  let familyMembers = [];
  
  try {
    // 1. Fetch chores, submissions, and members concurrently in parallel (3x faster)
    const [tasks, allSubmissions, members] = await Promise.all([
      apiCall(`/choregroups/${groupId}/tasks`).catch(err => {
        console.error("Failed to fetch tasks:", err);
        return [];
      }),
      apiCall(`/choregroups/${groupId}/submissions`).catch(err => {
        console.error("Failed to fetch submissions:", err);
        return [];
      }),
      apiCall(`/choregroups/${groupId}/members`).catch(err => {
        console.error("Failed to fetch members:", err);
        return [];
      })
    ]);

    activeTasks = tasks || [];
    pendingSubmissions = (allSubmissions || []).filter(s => {
      const task = activeTasks.find(t => t.id === s.task_id);
      return s.status === "pending_approval" || (task && task.status === "pending_approval");
    });
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

    // Self-healing: Update session if choregroup_name is generic/legacy (e.g. "My Household")
    if (currentSession && (currentSession.choregroup_name === "My Household" || !currentSession.choregroup_name)) {
      try {
        const groupInfo = await apiCall(`/choregroups/${groupId}`);
        if (groupInfo && groupInfo.name) {
          currentSession.choregroup_name = groupInfo.name;
          localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
        }
      } catch (err) {
        console.warn("Could not self-heal choregroup name:", err);
      }
    }
  
  // Render Active Chores list
  if (choresList) choresList.innerHTML = "";
  
  const activeOnly = activeTasks.filter(t => t.status !== "done");
  
  activeOnly.sort((a, b) => {
    // 1. Pending approval at top, then expired tasks, then other chores
    const getTaskTier = (t) => {
      if (t.status === "pending_approval") return 1;
      if (t.status === "expired" || (t.status === "assigned" && t.is_expired)) return 2;
      return 3;
    };

    const tierA = getTaskTier(a);
    const tierB = getTaskTier(b);
    if (tierA !== tierB) return tierA - tierB;

    if (a.is_mandatory !== b.is_mandatory) return a.is_mandatory ? -1 : 1;

    const aTimed = !!(a.status === "assigned" && a.expires_at);
    const bTimed = !!(b.status === "assigned" && b.expires_at);
    if (aTimed !== bTimed) return aTimed ? -1 : 1;

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
      if (task.type !== "cooperative" && task.assigned_to_user_id) {
        const assignee = familyMembers.find(u => u.id === task.assigned_to_user_id);
        if (assignee) {
          assigneeHtml = `<p>Assigned to: <strong>${assignee.username}</strong></p>`;
        }
      }
      
      let dependencyHtml = "";
      if (task.parent_task_id) {
        const parentTask = activeTasks.find(t => t.id === task.parent_task_id);
        if (parentTask) {
          const { title: parentTitle } = parseChoreTitle(parentTask.title);
          dependencyHtml = `<p style="font-size: 0.8rem; color: var(--primary-blue); margin-top: 0.25rem;">🔗 Continuation of: <strong>${parentTitle}</strong></p>`;
        }
      }

      const iconUrl = task.icon_url ? resolveIconUrl(task.icon_url) : '';
      const card = document.createElement("div");
      card.id = `parent-task-${task.id}`;
      card.dataset.taskId = task.id;
      if (iconUrl) {
        card.dataset.photoUrl = iconUrl;
        card.dataset.choreTitle = title;
      }
      card.className = `chore-card ${task.status === 'pending_approval' ? 'submitted' : ''} ${iconUrl ? 'has-photo' : ''}`;
      
      let statusLineHtml = "";
      if (task.status === "expired") {
        statusLineHtml = `
          <div class="card-status-line" style="margin-top: 0.35rem; display: flex; align-items: center; gap: 0.35rem;">
            <span class="card-status-badge-inline" style="background-color: #ffe3e3; color: #e03131; border: 1px solid #ffa8a8; font-size: 0.8rem; font-weight: 800; padding: 0.2rem 0.6rem; border-radius: var(--radius-sm); display: inline-flex; align-items: center; gap: 0.25rem;">
              <span>🚨 Expired</span>
            </span>
          </div>
        `;
      } else if (task.status === "pending_approval") {
        statusLineHtml = `
          <div class="card-status-line" style="margin-top: 0.35rem; display: flex; align-items: center; gap: 0.35rem;">
            <span class="card-status-badge-inline" style="background-color: var(--warning-amber-light); color: var(--warning-amber); border: 1px solid var(--warning-amber); font-size: 0.8rem; font-weight: 800; padding: 0.2rem 0.6rem; border-radius: var(--radius-sm); display: inline-flex; align-items: center; gap: 0.25rem;">
              <span>⏳ Pending Approval</span>
            </span>
          </div>
        `;
      }
      
      let timerHtml = "";
      if (task.status === "assigned" && task.expires_at) {
        timerHtml = `<div class="task-timer-badge" data-expires="${task.expires_at}" data-mandatory="${task.is_mandatory}" style="margin-top: 0.5rem; font-size: 0.85rem; color: #e65100; font-weight: bold;">⏳ Loading timer...</div>`;
      }

      const unexpireBtnHtml = task.status === "expired" ? `
        <button class="btn btn-outline btn-sm btn-unexpire" onclick="handleRemoveExpiration('${task.id}')" title="Reactivate Chore (Remove Expiration)" aria-label="Reactivate Chore">
          🔄
        </button>
      ` : '';

      const coinHtml = task.type === "cooperative" ? `<span class="points-coin coop-theme">🐱🐱</span>` : `<span class="points-coin">🐱</span>`;
      card.innerHTML = `
        <div class="card-top">
          <div class="card-emoji-box">${renderItemIcon(task, emoji, title)}</div>
          <div class="card-details">
            <h3>${displayTitle}</h3>
            ${assigneeHtml}
            ${dependencyHtml}
            ${timerHtml}
            ${statusLineHtml}
          </div>
        </div>
        <div class="card-actions-row">
          <div style="display: flex; align-items: center; gap: 0.5rem; flex-shrink: 0;">
            ${task.is_mandatory ? '' : `
              <div class="card-points-badge">
                +${task.points_reward} ${coinHtml}
              </div>
            `}
            ${task.continuation_count > 0 ? `
              <div class="continuation-count-badge" style="color: #475569; font-size: 0.95rem; font-weight: bold; display: inline-flex; align-items: center; gap: 0.2rem; margin-left: 0.5rem;">
                +${task.continuation_count} 📋
              </div>
            ` : ''}
          </div>
          <div class="card-controls">
            ${unexpireBtnHtml}
            <button class="btn btn-outline btn-sm" onclick="openAddChoreModal('${task.id}')" title="Add Continuation Chore">🔗</button>
            ${task.status !== "pending_approval" ? `<button class="btn btn-outline btn-sm" onclick="openEditChoreModal('${task.id}')" title="Edit Chore">✏️</button>` : ''}
            <button class="btn btn-outline btn-sm" style="color: var(--destructive-red); border-color: var(--destructive-red);" onclick="deleteChore('${task.id}')" title="Delete Chore">🗑️</button>
          </div>
        </div>
      `;

      // card onclick removed to only allow image click

      choresList.appendChild(card);
    });
  }
  
  // Render Pending Submissions
  if (pendingList) pendingList.innerHTML = "";
  
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
      const iconUrl = task.icon_url ? resolveIconUrl(task.icon_url) : '';
      
      const subItem = document.createElement("div");
      if (iconUrl) {
        subItem.dataset.photoUrl = iconUrl;
        subItem.dataset.choreTitle = title;
      }
      subItem.className = iconUrl ? "submission-item has-photo" : "submission-item";

      const isMandatory = Boolean(task.is_mandatory || task.points_reward === 0);
      const ptsBadgeHtml = isMandatory
        ? `<span class="sub-pts sub-mandatory" title="Mandatory Chore" aria-label="Mandatory Chore">🚨</span>`
        : `<span class="sub-pts">+${task.points_reward} ${task.type === 'cooperative' ? '<span class="points-coin coop-theme">🐱🐱</span>' : '<span class="points-coin">🐱</span>'}</span>`;

      subItem.innerHTML = `
        <div class="sub-header">
          <span class="sub-who">👾 ${kid.username} completed:</span>
          ${ptsBadgeHtml}
        </div>
        <div class="sub-chore">
          <span class="sub-chore-emoji">${renderItemIcon(task, emoji, title)}</span>
          <span>${displayTitle}</span>
        </div>
        <div class="sub-actions">
          <button class="btn-approve" onclick="approveSubmission('${sub.id}', '${task.id}')">✔️ Approve</button>
          <button class="btn-reject" onclick="rejectSubmission('${sub.id}', '${task.id}')">❌ Reject</button>
        </div>
      `;

      subItem.onclick = (e) => {
        if (e.target.closest("button, input, select, textarea, a, .sub-actions")) {
          return;
        }
        if (iconUrl) {
          openImageModal(iconUrl, title);
        }
      };
      pendingList.appendChild(subItem);
    });
  }
  
  // Render Family Members
  const membersList = document.getElementById("parent-members-list");
  membersList.innerHTML = "";
  
  // Sort family members: logged-in user first, followed by other admins, then kids alphabetically
  const sortedMembers = [...familyMembers].sort((a, b) => {
    const isMeA = currentSession && a.id === currentSession.user_id;
    const isMeB = currentSession && b.id === currentSession.user_id;
    if (isMeA) return -1;
    if (isMeB) return 1;
    if (a.role === "admin" && b.role !== "admin") return -1;
    if (a.role !== "admin" && b.role === "admin") return 1;
    return a.username.localeCompare(b.username);
  });
  
  sortedMembers.forEach(m => {
    const item = document.createElement("div");
    item.className = "member-item";
    const currentUid = currentSession.user_id || currentSession.id;
    const isMe = currentSession && m.id === currentUid;
    if (isMe && currentSession) {
      let changed = false;
      if (currentSession.profile_image_url !== m.profile_image_url) {
        currentSession.profile_image_url = m.profile_image_url || null;
        changed = true;
      }
      if (m.avatar_frame !== undefined && currentSession.avatar_frame !== m.avatar_frame) {
        currentSession.avatar_frame = m.avatar_frame || 'none';
        changed = true;
      }
      if (m.unlocked_avatar_frames) {
        currentSession.unlocked_avatar_frames = m.unlocked_avatar_frames;
      }
      if (changed) {
        localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
        updateHeaderAuthBtn();
      }
    }
    const memberFrame = m.avatar_frame || 'none';
    const avatarHtml = `
      <div onclick="openAvatarModal('${m.id}', '${escapeHTML(m.username)}', '${escapeHTML(m.profile_image_url || '')}', ${m.role === 'admin'}, '${escapeHTML(memberFrame)}')" title="Click to change profile picture or frame" style="display: inline-flex;">
        ${renderAvatarHtml(m.profile_image_url, m.role === 'admin', 'clickable', memberFrame)}
      </div>
    `;
      
    let actionBtnHtml = !isMe ? `
      <div style="margin-left: auto; display: flex; gap: 0.4rem; align-items: center;">
        <button type="button" class="member-action-btn" onclick="openChangePasswordModal('${m.id}', '${escapeHTML(m.username)}')" title="Change Password" aria-label="Change Password">🔑</button>
        <button type="button" class="member-action-btn btn-primary" onclick="generateKidAccessLink('${m.id}', '${escapeHTML(m.username)}', '${m.role}')" title="Share Login Link" aria-label="Share Login Link"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><circle cx="18" cy="5" r="3" /><circle cx="6" cy="12" r="3" /><circle cx="18" cy="19" r="3" /><line x1="8.59" y1="13.51" x2="15.42" y2="17.49" /><line x1="15.41" y1="6.51" x2="8.59" y2="10.49" /></svg></button>
        <button type="button" class="member-action-btn btn-delete" onclick="deleteFamilyMember('${m.id}', '${escapeHTML(m.username)}')" title="Remove Member" aria-label="Remove Member">❌</button>
      </div>
    ` : '';
      
    item.innerHTML = `
      <div class="member-info">
        ${avatarHtml}
        <span class="member-name">${m.username}${isMe ? ' (Me)' : ''}</span>
      </div>
      ${actionBtnHtml}
    `;
    membersList.appendChild(item);
  });
  } catch (err) {
    console.error("Dashboard render failed:", err);
  } finally {
    setTabLoading("parent-chores-content", false);
  }
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

async function openAddChoreModal(parentTaskId = "") {
  document.getElementById("chore-id-input").value = "";
  document.getElementById("chore-parent-task-id-input").value = parentTaskId;
  
  const mandatoryInput = document.getElementById("chore-mandatory-input");
  if (mandatoryInput) {
    mandatoryInput.checked = false;
    toggleMandatoryChore(false);
  }
  
  document.getElementById("chore-type-input").value = "individual";
  toggleChoreType("individual");
  setPointsSelectValue("auto");
  
  const assigneeSelect = document.getElementById("chore-assignee-input");
  if (parentTaskId) {
    const parentTask = activeTasks.find(t => t.id === parentTaskId);
    const parentTitle = parentTask ? parseChoreTitle(parentTask.title).title : "Chore";
    document.getElementById("chore-modal-title").innerText = `Add Continuation for "${parentTitle}"`;
    document.getElementById("chore-title-input").value = "";
    
    await populateAssigneeSelect(parentTask ? parentTask.assigned_to_user_id : null);
    if (parentTask && parentTask.assigned_to_user_id) {
      assigneeSelect.disabled = true;
    } else {
      assigneeSelect.disabled = false;
    }
  } else {
    document.getElementById("chore-modal-title").innerText = "Create New Chore";
    document.getElementById("chore-title-input").value = "";
    await populateAssigneeSelect();
    assigneeSelect.disabled = false;
  }
  
  clearIconUpload("chore");
  document.getElementById("chore-modal").classList.add("active");
}

// Add Chore submit handler
async function handleSaveChore(e) {
  e.preventDefault();
  
  const taskId = document.getElementById("chore-id-input").value;
  const parentTaskId = document.getElementById("chore-parent-task-id-input").value || null;
  const titleVal = document.getElementById("chore-title-input").value.trim();
  const rawPoints = document.getElementById("chore-points-input") ? document.getElementById("chore-points-input").value : "auto";
  const points = (rawPoints === "auto" || !rawPoints) ? 0 : (parseInt(rawPoints) || 0);
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

  const iconUrl = document.getElementById("chore-icon-url-input") ? (document.getElementById("chore-icon-url-input").value || null) : null;
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
        expires_at: expiresAt,
        parent_task_id: parentTaskId,
        icon_url: iconUrl
      });
      showToast("Chore updated successfully!");
    } else {
      // POST /choregroups/{groupID}/tasks (CreateTaskRequest)
      const createdTask = await apiCall(`/choregroups/${groupId}/tasks`, "POST", {
        assigned_to_user_id: assigneeId || null,
        points_reward: points,
        title: fullTitle,
        type: type,
        is_mandatory: isMandatory,
        expires_at: expiresAt,
        parent_task_id: parentTaskId,
        icon_url: iconUrl
      });
      const coinsMsg = (createdTask && createdTask.points_reward) ? ` (+${createdTask.points_reward} 🐱)` : '';
      showToast(`Chore created successfully!${coinsMsg}`);
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
  document.getElementById("chore-parent-task-id-input").value = task.parent_task_id || "";
  
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
  
  const assigneeSelect = document.getElementById("chore-assignee-input");
  if (task.parent_task_id) {
    const parentTask = activeTasks.find(t => t.id === task.parent_task_id);
    if (parentTask && parentTask.assigned_to_user_id) {
      assigneeSelect.disabled = true;
    } else {
      assigneeSelect.disabled = false;
    }
  } else {
    assigneeSelect.disabled = false;
  }
  
  if (task.icon_url) {
    document.getElementById("chore-icon-url-input").value = task.icon_url;
    document.getElementById("chore-icon-preview-img").src = task.icon_url;
    document.getElementById("chore-icon-preview-container").style.display = "flex";
  } else {
    clearIconUpload("chore");
  }
  
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
  document.getElementById("new-member-pass").value = "";
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
    const newUser = await apiCall("/users", "POST", {
      choregroup_name: groupName,
      password: password,
      user_role: role,
      username: name
    });
    showToast(`User ${name} added successfully!`);
    closeModal("add-user-modal");
    renderParentDashboard();
    updateLeaderboards();

    if (newUser && newUser.id) {
      await generateKidAccessLink(newUser.id, newUser.username, newUser.role);
    }
  } catch (err) {
    alert("Failed to add member: " + err.message);
    closeModal("add-user-modal");
    renderParentDashboard();
    updateLeaderboards();
  }
}

async function deleteFamilyMember(userId, username) {
  const confirmed = confirm(`Are you sure you want to remove ${username} from the family? This will permanently delete their account and all chore history.`);
  if (!confirmed) return;

  const groupId = currentSession?.choregroup_id;
  if (!groupId) {
    alert("Could not determine your household. Please log in again.");
    return;
  }

  try {
    await apiCall(`/choregroups/${groupId}/members/${userId}`, "DELETE");
    showToast(`${username} removed successfully.`);
    renderParentDashboard();
    updateLeaderboards();
  } catch (err) {
    alert("Failed to remove member: " + err.message);
  }
}

function openChangePasswordModal(userId, username) {
  const modal = document.getElementById("change-password-modal");
  const title = document.getElementById("change-password-modal-title");
  const input = document.getElementById("change-password-input");
  const targetId = document.getElementById("change-password-target-id");

  if (!modal || !title || !input || !targetId) return;

  targetId.value = userId;
  input.value = "";

  if (currentSession && (currentSession.id === userId || currentSession.user_id === userId)) {
    title.textContent = "Change My Password";
  } else {
    title.textContent = `Change Password for ${username}`;
  }

  modal.classList.add("active");
}

async function handleSavePassword(e) {
  e.preventDefault();

  const userId = document.getElementById("change-password-target-id").value;
  const password = document.getElementById("change-password-input").value;
  const groupId = currentSession?.choregroup_id;

  if (!userId || !password || !groupId) {
    alert("Missing required data for changing password.");
    return;
  }

  try {
    await apiCall(`/choregroups/${groupId}/members/${userId}/password`, "PUT", {
      password: password
    });
    showToast("Password updated successfully!");
    closeModal("change-password-modal");
  } catch (err) {
    alert("Failed to update password: " + err.message);
  }
}

/* ==========================================================
   AVATAR / PROFILE PICTURE MANAGEMENT
   ========================================================== */

const AVATAR_PRESETS = {
  animals: ['🐶', '🐱', '🦊', '🐼', '🦁', '🐯', '🐨', '🐰', '🦄', '🦖', '🐬', '🦉', '🐵', '🐧', '🐸', '🐙', '🦋', '🐝'],
  characters: ['🧙‍♂️', '🦸‍♀️', '🥷', '👸', '🤴', '🧝', '🧛', '🧚', '🤖', '👽', '🤠', '👾', '🧟', '🧜‍♀️', '🧞', '🧑‍🚀', '💂', '🕵️'],
  cool: ['🎮', '🕹️', '🎲', '🎸', '🎨', '🛹', '⚽', '🏀', '🚀', '🏎️', '🥋', '🏆', '🎧', '🎯', '🎳', '🏄', '🥊', '🎪']
};

const AVATAR_FRAMES = [
  { id: "none", name: "Classic (None)", icon: "⚪", isSurprise: false },
  { id: "golden_sparkle", name: "Golden Sparkle", icon: "✨", isSurprise: true },
  { id: "fire_flame", name: "Dragon Flame", icon: "🔥", isSurprise: true },
  { id: "cosmic_nebula", name: "Cosmic Nebula", icon: "🌌", isSurprise: true },
  { id: "emerald_nature", name: "Emerald Nature", icon: "🌿", isSurprise: true },
  { id: "rainbow_wave", name: "Rainbow Aura", icon: "🌈", isSurprise: true },
  { id: "royal_crown", name: "Royal Crown", icon: "👑", isSurprise: true },
  { id: "neon_cyber", name: "Cyber Blitz", icon: "⚡", isSurprise: true },
];

let avatarModalState = {
  targetUserId: null,
  targetUsername: '',
  targetIsAdmin: false,
  selectedAvatarUrl: '',
  selectedFrame: 'none',
  currentTab: 'animals'
};

function openAvatarModal(userId = null, username = '', currentAvatarUrl = '', isAdmin = null, currentFrame = 'none') {
  if (!userId && currentSession) {
    userId = currentSession.user_id || currentSession.id;
    username = currentSession.username;
    currentAvatarUrl = currentSession.profile_image_url || '';
    isAdmin = currentSession.role === 'admin';
    currentFrame = currentSession.avatar_frame || 'none';
  } else if (userId) {
    if (!username || isAdmin === null) {
      if (currentSession && (currentSession.user_id === userId || currentSession.id === userId)) {
        username = currentSession.username;
        currentAvatarUrl = currentSession.profile_image_url || '';
        isAdmin = currentSession.role === 'admin';
        currentFrame = currentSession.avatar_frame || 'none';
      }
    }
  }

  avatarModalState.targetUserId = userId;
  avatarModalState.targetUsername = username || (currentSession ? currentSession.username : '');
  avatarModalState.targetIsAdmin = (isAdmin !== null) ? !!isAdmin : (currentSession && currentSession.role === 'admin');
  avatarModalState.selectedAvatarUrl = currentAvatarUrl || '';
  avatarModalState.selectedFrame = currentFrame || 'none';
  avatarModalState.currentTab = 'animals';
  
  const titleEl = document.getElementById("avatar-modal-title");
  if (titleEl) {
    const isMe = currentSession && (userId === currentSession.user_id || userId === currentSession.id);
    titleEl.innerText = isMe ? "Change Profile Picture & Frame" : `Picture & Frame for ${avatarModalState.targetUsername}`;
  }
  
  const captionEl = document.getElementById("avatar-modal-user-caption");
  if (captionEl) {
    const roleName = avatarModalState.targetIsAdmin ? 'admin' : 'user';
    captionEl.innerText = `${avatarModalState.targetUsername} • ${roleName}`;
  }

  updateAvatarPreview();
  switchAvatarTab('animals');
  
  const uploadStatus = document.getElementById("avatar-upload-status");
  if (uploadStatus) uploadStatus.innerText = "";
  
  // Close any open dropdowns
  const dropdown = document.getElementById("user-dropdown-menu");
  if (dropdown) dropdown.classList.remove("active");

  const modal = document.getElementById("avatar-modal");
  if (modal) {
    modal.classList.add("active");
  }
}

function updateAvatarPreview() {
  const previewContainer = document.getElementById("avatar-modal-preview");
  if (!previewContainer) return;
  
  const url = avatarModalState.selectedAvatarUrl;
  const isAdmin = avatarModalState.targetIsAdmin;
  const crownHtml = isAdmin ? '<span class="preview-crown">👑</span>' : '';
  const frame = avatarModalState.selectedFrame || 'none';
  const frameClass = (frame && frame !== 'none') ? `frame-${escapeHTML(frame)}` : 'frame-none';
  
  let innerContent = '';
  if (url && (url.startsWith('/') || url.startsWith('http'))) {
    innerContent = `<img src="${escapeHTML(url)}" alt="Preview" style="width:100%;height:100%;max-width:100%;max-height:100%;object-fit:cover;border-radius:50%;display:block;" />`;
  } else if (url && url.trim()) {
    innerContent = `<span>${escapeHTML(url)}</span>`;
  } else {
    innerContent = `<span>👤</span>`;
  }

  const inner = `
    <div class="avatar-preview-inner">
      ${innerContent}
      ${crownHtml}
    </div>
  `;
  previewContainer.className = `avatar-big-preview avatar-frame-badge ${frameClass}`;
  previewContainer.innerHTML = inner;
}

function renderAvatarFramesGrid(container) {
  container.className = "avatar-frames-grid";
  container.innerHTML = "";

  const isAdmin = (currentSession && currentSession.role === 'admin') || avatarModalState.targetIsAdmin;
  const unlocked = (currentSession && Array.isArray(currentSession.unlocked_avatar_frames))
    ? currentSession.unlocked_avatar_frames
    : ["none"];
  const currentEquipped = avatarModalState.selectedFrame || (currentSession ? currentSession.avatar_frame : "none");

  AVATAR_FRAMES.forEach(frame => {
    const isUnlocked = isAdmin || unlocked.includes(frame.id) || frame.id === "none";
    const isEquipped = currentEquipped === frame.id;
    
    const card = document.createElement("div");
    card.className = `avatar-frame-card ${isEquipped ? 'equipped' : ''} ${isUnlocked ? 'unlocked' : 'locked'}`;
    
    const avatarUrl = avatarModalState.selectedAvatarUrl || (currentSession ? currentSession.profile_image_url : '');
    const avatarInner = (avatarUrl && (avatarUrl.startsWith('/') || avatarUrl.startsWith('http')))
      ? `<img src="${escapeHTML(avatarUrl)}" class="member-avatar-img" alt="Avatar" style="width:100%;height:100%;max-width:100%;max-height:100%;border-radius:50%;object-fit:cover;display:block;" />`
      : `<span style="font-size: 1.3rem;">${escapeHTML(avatarUrl || '👤')}</span>`;

    let badgeText = "🔒 Locked";
    if (isEquipped) {
      badgeText = "Equipped ✓";
    } else if (isUnlocked) {
      badgeText = "Equip ✨";
    }

    card.innerHTML = `
      <div class="avatar-frame-card-preview">
        <span class="avatar-frame-badge frame-${frame.id}" style="width: 40px; height: 40px; display: inline-flex; align-items: center; justify-content: center;">
          ${avatarInner}
        </span>
      </div>
      <div class="avatar-frame-card-title">${frame.icon} ${escapeHTML(frame.name)}</div>
      <div class="avatar-frame-card-badge">${badgeText}</div>
    `;

    card.onclick = async () => {
      if (!isUnlocked) {
        showToast(`🔒 "${frame.name}" is locked! Unlock it via the Step 10 Daily Surprise or Lucky Wheel! 🎁`, "info");
        return;
      }
      avatarModalState.selectedFrame = frame.id;
      updateAvatarPreview();
      
      const targetUserId = avatarModalState.targetUserId || (currentSession && (currentSession.user_id || currentSession.id));
      if (currentSession && targetUserId) {
        try {
          const groupId = currentSession.choregroup_id;
          await apiCall(`/choregroups/${groupId}/members/${targetUserId}/avatar-frame`, "PUT", {
            avatar_frame: frame.id
          });
          if (currentSession.user_id === targetUserId || currentSession.id === targetUserId) {
            currentSession.avatar_frame = frame.id;
            localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
            updateHeaderAuthBtn();
          }
          showToast(`Equipped ${frame.name}! ✨`);
          renderAvatarFramesGrid(container);
          if (currentSession.role === "admin") {
            renderParentDashboard();
          } else {
            renderKidDashboard();
          }
        } catch (err) {
          console.error("Failed to equip avatar frame:", err);
          showToast("Failed to equip frame: " + err.message, "error");
        }
      }
    };

    container.appendChild(card);
  });
}

function switchAvatarTab(tabName) {
  avatarModalState.currentTab = tabName;
  ['animals', 'characters', 'cool', 'frames'].forEach(t => {
    const btn = document.getElementById(`avatar-tab-${t}`);
    if (btn) btn.classList.toggle('active', t === tabName);
  });
  
  const container = document.getElementById("avatar-grid-container");
  if (!container) return;
  container.innerHTML = "";

  if (tabName === 'frames') {
    renderAvatarFramesGrid(container);
    return;
  }

  container.className = "avatar-grid";

  // Picture upload slot as first button in grid (icon-only upload tile)
  const uploadSlotBtn = document.createElement("button");
  uploadSlotBtn.type = "button";
  uploadSlotBtn.className = "avatar-choice-btn upload-slot-btn";
  uploadSlotBtn.title = "Upload Picture from Device";
  uploadSlotBtn.setAttribute("aria-label", "Upload Picture from Device");
  uploadSlotBtn.innerHTML = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="17 8 12 3 7 8"></polyline><line x1="12" y1="3" x2="12" y2="15"></line></svg>`;
  uploadSlotBtn.onclick = () => document.getElementById('avatar-file-upload-input').click();
  container.appendChild(uploadSlotBtn);
  
  const list = AVATAR_PRESETS[tabName] || [];
  list.forEach(emoji => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = `avatar-choice-btn ${avatarModalState.selectedAvatarUrl === emoji ? 'selected' : ''}`;
    btn.innerText = emoji;
    btn.onclick = () => {
      avatarModalState.selectedAvatarUrl = emoji;
      updateAvatarPreview();
      document.querySelectorAll('.avatar-choice-btn').forEach(b => b.classList.remove('selected'));
      btn.classList.add('selected');
    };
    container.appendChild(btn);
  });
}

async function handleAvatarFileUpload(event) {
  const file = event.target.files && event.target.files[0];
  if (!file) return;
  
  const statusEl = document.getElementById("avatar-upload-status");
  if (statusEl) statusEl.innerText = "Uploading picture... ⏳";
  
  const formData = new FormData();
  formData.append("image", file);
  
  try {
    const res = await fetch(`${getApiBase()}/upload`, {
      method: "POST",
      body: formData,
      credentials: "include"
    });
    if (!res.ok) {
      throw new Error(`Upload failed: ${res.statusText}`);
    }
    const data = await res.json();
    const photoUrl = data.icon_url;
    avatarModalState.selectedAvatarUrl = photoUrl;
    updateAvatarPreview();
    document.querySelectorAll('.avatar-choice-btn').forEach(b => b.classList.remove('selected'));
    if (statusEl) statusEl.innerText = "Picture selected! Tap ✔ to save.";
  } catch (err) {
    console.error("Avatar upload failed:", err);
    if (statusEl) statusEl.innerText = "⚠️ Upload failed. Please try a different image.";
  } finally {
    event.target.value = "";
  }
}

function resetAvatarToDefault() {
  avatarModalState.selectedAvatarUrl = "";
  updateAvatarPreview();
  document.querySelectorAll('.avatar-choice-btn').forEach(b => b.classList.remove('selected'));
  const statusEl = document.getElementById("avatar-upload-status");
  if (statusEl) statusEl.innerText = "Default silhouette selected. Tap ✔ to save.";
}

async function saveSelectedAvatar() {
  const { targetUserId, selectedAvatarUrl, selectedFrame } = avatarModalState;
  const uid = targetUserId || (currentSession && (currentSession.user_id || currentSession.id));
  if (!uid || !currentSession) return;
  
  const saveBtn = document.getElementById("avatar-save-btn");
  const saveSvg = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>`;
  if (saveBtn) {
    saveBtn.disabled = true;
    saveBtn.innerHTML = '⏳';
  }
  
  try {
    const groupId = currentSession.choregroup_id;
    await apiCall(`/choregroups/${groupId}/members/${uid}/avatar`, "PUT", {
      avatar_url: selectedAvatarUrl
    });

    if (selectedFrame) {
      try {
        await apiCall(`/choregroups/${groupId}/members/${uid}/avatar-frame`, "PUT", {
          avatar_frame: selectedFrame
        });
      } catch (fErr) {
        console.warn("Avatar frame update in saveSelectedAvatar:", fErr);
      }
    }
    
    // If the edited user is current session user, update local currentSession
    const isMe = uid === currentSession.user_id || uid === currentSession.id;
    if (isMe) {
      currentSession.profile_image_url = selectedAvatarUrl || null;
      if (selectedFrame) {
        currentSession.avatar_frame = selectedFrame;
      }
      localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
      updateHeaderAuthBtn();
    }
    
    closeModal("avatar-modal");
    showToast("Profile updated! ✨");
    
    // Refresh family members or kid dashboard
    if (currentSession.role === "admin") {
      renderParentDashboard();
    } else {
      renderKidDashboard();
    }
  } catch (err) {
    console.error("Failed to save avatar:", err);
    alert("Could not update profile: " + err.message);
  } finally {
    if (saveBtn) {
      saveBtn.disabled = false;
      saveBtn.innerHTML = saveSvg;
    }
  }
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
    showToast("Submission returned to user.");
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
  
  setTabLoading("kid-chores-content", true);
  
  const kidChoresList = document.getElementById("kid-chores-list");
  
  let kidTasks = [];
  let userPoints = 0;
  let dailyBonus = null;
  let members = [];
  let me = null;
  
  try {
    // 1. Fetch chores, members, stats, and daily bonus concurrently in parallel (3-4x faster)
    const [tasks, fetchedMembers, statsData, fetchedDailyBonus] = await Promise.all([
      apiCall(`/choregroups/${groupId}/tasks`).catch(err => {
        console.error("Failed to fetch tasks:", err);
        return [];
      }),
      apiCall(`/choregroups/${groupId}/members`).catch(err => {
        console.error("Failed to fetch members:", err);
        return [];
      }),
      apiCall(`/choregroups/${groupId}/statistics`).catch(() => ({ users: [], cooperative_points: 0 })),
      apiCall(`/choregroups/${groupId}/daily-bonus`).catch(dbErr => {
        console.warn("Could not fetch daily bonus status:", dbErr);
        return null;
      })
    ]);

    kidTasks = (tasks || []).filter(t => !t.assigned_to_user_id || t.assigned_to_user_id === userId || t.type === 'cooperative');
    members = fetchedMembers || [];
    me = members.find(m => m.id === userId);
    userPoints = me ? me.points : (currentSession.points || 0);

    currentSession.cooperative_points = (statsData && statsData.cooperative_points) || 0;
    dailyBonus = fetchedDailyBonus;
    if (dailyBonus) {
      currentDailyBonus = dailyBonus;
      currentSession.spins_available = dailyBonus.spins_available || 0;
    }

    // Set UI Details: Save fresh points and refresh header auth banner
    currentSession.points = userPoints;
    if (me && me.profile_image_url !== undefined && me.profile_image_url !== currentSession.profile_image_url) {
      currentSession.profile_image_url = me.profile_image_url;
    }
    if (me && me.avatar_frame !== undefined && me.avatar_frame !== currentSession.avatar_frame) {
      currentSession.avatar_frame = me.avatar_frame;
    }
    if (me && me.unlocked_avatar_frames !== undefined) {
      currentSession.unlocked_avatar_frames = me.unlocked_avatar_frames;
    }
    if (me && me.points_background !== undefined) {
      currentSession.points_background = me.points_background;
    }
    if (me && me.unlocked_backgrounds !== undefined) {
      currentSession.unlocked_backgrounds = me.unlocked_backgrounds;
    }
    localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
    updateHeaderAuthBtn();

    // Render Daily Bonus Progress Bar
    renderDailyBonusBar(dailyBonus);
    
    // Render My Chores list
    if (kidChoresList) kidChoresList.innerHTML = "";
    
    const activeOnly = kidTasks.filter(t => t.status !== "done");
    // A mandatory task blocks non-mandatory tasks if it's assigned OR pending approval
    const hasPendingMandatory = activeOnly.some(t => t.is_mandatory && (t.status === "assigned" || t.status === "pending_approval"));
    
    // Toggle the Snap Chore button based on mandatory task status
    const kidSnapBtn = document.getElementById("kid-snap-chore-btn");
    if (kidSnapBtn) {
      if (hasPendingMandatory) {
        kidSnapBtn.disabled = true;
        kidSnapBtn.title = "Complete your mandatory chores first!";
        kidSnapBtn.style.opacity = "0.45";
        kidSnapBtn.style.cursor = "not-allowed";
      } else {
        kidSnapBtn.disabled = false;
        kidSnapBtn.title = "";
        kidSnapBtn.style.opacity = "1";
        kidSnapBtn.style.cursor = "pointer";
      }
    }
    
    activeOnly.sort((a, b) => {
      // 1. Pending approval at top, then expired tasks, then other chores
      const getTaskTier = (t) => {
        if (t.status === "pending_approval") return 1;
        if (t.status === "expired" || (t.status === "assigned" && t.is_expired)) return 2;
        return 3;
      };

      const tierA = getTaskTier(a);
      const tierB = getTaskTier(b);
      if (tierA !== tierB) return tierA - tierB;

      if (a.is_mandatory !== b.is_mandatory) return a.is_mandatory ? -1 : 1;

      const aTimed = !!(a.status === "assigned" && a.expires_at);
      const bTimed = !!(b.status === "assigned" && b.expires_at);
      if (aTimed !== bTimed) return aTimed ? -1 : 1;

      return 0;
    });
    
    if (activeOnly.length === 0) {
      kidChoresList.innerHTML = "";
    } else {
      activeOnly.forEach(task => {
        // Locked due to mandatory tasks
        const isLockedByMandatory = !task.is_mandatory && hasPendingMandatory;
        // Locked due to incomplete parent task
        let isLockedByParent = false;
        if (task.parent_task_id) {
          const parentTask = activeOnly.find(t => t.id === task.parent_task_id);
          if (parentTask && parentTask.status !== "done") {
            isLockedByParent = true;
          }
        }

        const isDisabled = isLockedByMandatory || isLockedByParent;

        const card = document.createElement("div");
        card.className = `chore-card ${isDisabled ? 'disabled' : ''}`;
        card.id = "kid-task-" + task.id;

        const { emoji, title } = parseChoreTitle(task.title);
        if (task.icon_url) {
          card.dataset.photoUrl = resolveIconUrl(task.icon_url);
          card.dataset.choreTitle = title;
        }

        let actionHtml = "";
        if (task.status === "assigned") {
          actionHtml = `<button class="btn btn-outline btn-submit-chore" onclick="event.stopPropagation(); submitChoreForApproval('${task.id}')" title="Submit Chore" aria-label="Submit Chore"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg></button>`;
        } else if (task.status === "pending_approval") {
          actionHtml = `<span class="badge badge-warning">⏳ Pending Approval</span>`;
        }

        let displayTitle = title;
        if (task.is_mandatory) {
          displayTitle = `<span class="mandatory-star-icon" title="Mandatory Chore">🚨</span> ` + title;
        }

        let timerHtml = "";
        if (task.status === "assigned" && task.expires_at) {
          timerHtml = `
            <div class="task-timer" data-expires="${task.expires_at}">
              <span class="timer-icon">⏳</span>
              <span class="timer-text">Calculating...</span>
            </div>
          `;
        } else if (task.status === "assigned" && task.is_expired) {
          timerHtml = `
            <div class="task-timer expired">
              <span>🚨 Expired</span>
            </div>
          `;
        }
        
        const coinHtml = task.type === "cooperative" ? `<span class="points-coin coop-theme">🐱🐱</span>` : `<span class="points-coin">🐱</span>`;

        card.innerHTML = `
          <div class="card-top">
            <div class="card-emoji-box">${renderItemIcon(task, emoji, title)}</div>
            <div class="card-details">
              <h3 ${isDisabled ? 'style="color: #999;"' : ''}>${displayTitle}</h3>
              ${timerHtml}
            </div>
          </div>
          <div class="card-actions-row">
            <div style="display: flex; align-items: center; gap: 0.5rem; flex-shrink: 0;">
              ${task.is_mandatory ? '' : `
                <div class="card-points-badge">
                  +${task.points_reward} ${coinHtml}
                  ${(dailyBonus && dailyBonus.double_coins_active) ? '<span class="task-double-badge" title="2x Coins Multiplier Active for this chore!">2×🪙</span>' : ''}
                </div>
              `}
              ${task.continuation_count > 0 ? `
                <div class="continuation-count-badge" style="color: #475569; font-size: 0.95rem; font-weight: bold; display: inline-flex; align-items: center; gap: 0.2rem; margin-left: 0.5rem;">
                  +${task.continuation_count} 📋
                </div>
              ` : ''}
            </div>
            <div class="card-controls">
              ${actionHtml}
            </div>
          </div>
        `;

        if (isDisabled) {
          const overlay = document.createElement("div");
          overlay.className = "locked-overlay";
          overlay.innerHTML = "🔒 Locked";
          card.appendChild(overlay);

          card.onclick = (e) => {
            e.stopPropagation();
            e.preventDefault();
            
            let targets = [];
            if (isLockedByMandatory) {
              // Prioritize mandatory chores
              targets = activeOnly.filter(t => t.is_mandatory && (t.status === "assigned" || t.status === "pending_approval"));
            } else if (isLockedByParent) {
              // Otherwise, target parent chore
              const parentTask = activeOnly.find(t => t.id === task.parent_task_id);
              if (parentTask) {
                targets = [parentTask];
              }
            }

            // Clear any existing flashing highlights first
            document.querySelectorAll('.flashing-highlight').forEach(el => {
              el.classList.remove('flashing-highlight');
            });

            targets.forEach(target => {
              const targetEl = document.getElementById("kid-task-" + target.id);
              if (targetEl) {
                targetEl.scrollIntoView({ behavior: "smooth", block: "center" });
                // Force animation restart by removing and re-adding after a frame
                requestAnimationFrame(() => {
                  targetEl.classList.add("flashing-highlight");
                });
                setTimeout(() => {
                  targetEl.classList.remove("flashing-highlight");
                }, 3000);
              }
            });
          };
        } else {
          // card onclick removed to only allow image click
        }

        kidChoresList.appendChild(card);
      });
    }
    
    await renderKidSidebarWidgets(members);
  } catch (err) {
    console.error("Kid Dashboard render failed:", err);
  } finally {
    setTabLoading("kid-chores-content", false);
  }
}

async function renderKidSidebarWidgets(existingMembers = null) {
  const groupId = currentSession.choregroup_id;
  let kids = [];
  
  try {
    const members = (existingMembers && existingMembers.length > 0)
      ? existingMembers
      : await apiCall(`/choregroups/${groupId}/members`);
    kids = members.filter(m => m.role === "user");
  } catch (err) {
    return;
  }
  
  const sortedKids = [...kids].sort((a,b) => b.points - a.points);
  
  const first = sortedKids[0] || { username: "-", points: 0 };
  const second = sortedKids[1] || { username: "-", points: 0 };
  const third = sortedKids[2] || { username: "-", points: 0 };
  
  const renderPodiumAvatar = (u, defaultEmoji) => {
    const frame = u.avatar_frame || 'none';
    if (u.profile_image_url) {
      return `<div style="display:inline-flex;margin-bottom:4px;">${renderAvatarHtml(u.profile_image_url, false, '', frame)}</div>`;
    }
    const ringClass = defaultEmoji === '👑' ? 'gold' : (defaultEmoji === '🥈' ? 'silver' : 'bronze');
    return `<div class="avatar-ring ${ringClass}"><span class="avatar-frame-badge frame-${frame}">${defaultEmoji}</span></div>`;
  };

  const podiumContainer = document.getElementById("kid-podium-container");
  if (podiumContainer) {
    podiumContainer.innerHTML = `
      <div class="podium-col second">
        ${renderPodiumAvatar(second, '🥈')}
        <div class="podium-name">${second.username}</div>
        <div class="podium-bar">
          <span class="points-label">${second.points} <span class="points-coin">🐱</span></span>
        </div>
      </div>
      <div class="podium-col first">
        ${renderPodiumAvatar(first, '👑')}
        <div class="podium-name">${first.username}</div>
        <div class="podium-bar">
          <span class="points-label">${first.points} <span class="points-coin">🐱</span></span>
        </div>
      </div>
      <div class="podium-col third">
        ${renderPodiumAvatar(third, '🥉')}
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

  const kidsCoinsHtml = sortedKids.map(k => `
    <div class="stats-metric" style="margin-top: 0.5rem; border-top: 1px dashed var(--border-color); padding-top: 0.5rem;">
      <span class="stats-label" style="font-weight: normal; color: var(--text-muted);">${escapeHTML(k.username)}'s Coins:</span>
      <span class="stats-value" style="font-size: 1rem;">${k.points} <span class="points-coin">🐱</span></span>
    </div>
  `).join("");

  const statsHtml = `
    <div class="stats-container">
      <div class="stats-metric">
        <span class="stats-label">Cooperative Coins:</span>
        <span class="stats-value">${cooperativeCoins} <span class="points-coin coop-theme">🐱🐱</span></span>
      </div>
      ${kidsCoinsHtml}
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
    let reasoning = "Standard physical reward request. Moderate admin effort needed.";
    
    if (lowerText.includes("video") || lowerText.includes("game") || lowerText.includes("roblox") || lowerText.includes("fortnite") || lowerText.includes("minecraft") || lowerText.includes("screen") || lowerText.includes("ipad") || lowerText.includes("tablet") || lowerText.includes("computer")) {
      estimatedPoints = 5;
      reasoning = "Requires screen time scheduling and configuration by admin. Screens are priced higher to encourage active physical play.";
    } else if (lowerText.includes("candy") || lowerText.includes("ice cream") || lowerText.includes("sugar") || lowerText.includes("sweet") || lowerText.includes("chocolate") || lowerText.includes("cookie")) {
      estimatedPoints = 4;
      reasoning = "Sugar reward. Requires admin verification for moderation. Balance with physical activity and healthy snacks.";
    } else if (lowerText.includes("toy") || lowerText.includes("lego") || lowerText.includes("doll") || lowerText.includes("buy") || lowerText.includes("plush")) {
      estimatedPoints = 25;
      reasoning = "Physical retail items require cash budgeting. High point requirements incentivize long-term chores commitment.";
    } else if (lowerText.includes("sleepover") || lowerText.includes("friend") || lowerText.includes("park") || lowerText.includes("zoo")) {
      estimatedPoints = 12;
      reasoning = "Social outings involve admin schedule coordination, travel, and supervision. Moderate point pricing reflects resource investment.";
    } else if (lowerText.includes("sleep") || lowerText.includes("bedtime") || lowerText.includes("stay up")) {
      estimatedPoints = 8;
      reasoning = "Altering sleep routines impacts physical recovery. Fairly high pricing keeps user's schedule consistent.";
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
    showToast("Demo Chore Completed! Points balance increased in user view! 🎉");
    
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
      showToast("Admin approved the demo chore! +5 Points. 🏆");
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
function showToast(message, duration = 4000) {
  const toast = document.getElementById("toast-message");
  if (!toast) return;
  toast.innerText = message;
  toast.classList.add("active");
  
  if (toast.dataset.timeoutId) {
    clearTimeout(parseInt(toast.dataset.timeoutId, 10));
  }
  
  const timeoutId = setTimeout(() => {
    toast.classList.remove("active");
  }, duration);
  toast.dataset.timeoutId = timeoutId;
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
function setTabLoading(tabContentId, isLoading) {
  const tabContent = document.getElementById(tabContentId);
  if (!tabContent) return;
  
  let loader = tabContent.querySelector('.tab-loader');
  if (!loader) {
    loader = document.createElement('div');
    loader.className = 'tab-loader';
    loader.innerHTML = `<div class="loading-spinner-container"><div class="loading-spinner"></div><span>Loading...</span></div>`;
    tabContent.insertBefore(loader, tabContent.firstChild);
  }
  
  loader.style.display = isLoading ? 'flex' : 'none';
  
  Array.from(tabContent.children).forEach(child => {
    if (child !== loader) {
      child.style.display = isLoading ? 'none' : '';
    }
  });
}

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
  
  if (tab === 'chores') {
    renderParentDashboard();
  } else if (tab === 'rewards') {
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

function formatDescriptionWithLinks(text) {
  if (!text) return '';
  let escaped = escapeHTML(text);

  // 1. Markdown links: [anchor](url)
  escaped = escaped.replace(/\[([^\]]+)\]\(((?:https?:\/\/|www\.)[^\s\)]+)\)/gi, (match, label, url) => {
    const fullUrl = url.toLowerCase().startsWith('www.') ? 'https://' + url : url;
    return `<a href="${fullUrl}" target="_blank" rel="noopener noreferrer" class="reward-link" onclick="event.stopPropagation();">${label}</a>`;
  });

  // 2. Raw http/https URLs (ensure not already inside an href attribute)
  escaped = escaped.replace(/(^|[^"'>=])(https?:\/\/[^\s<]+[^<.,:;"')\]\s])/gi, (match, prefix, url) => {
    return `${prefix}<a href="${url}" target="_blank" rel="noopener noreferrer" class="reward-link" onclick="event.stopPropagation();">${url}</a>`;
  });

  // 3. Raw www. URLs (not preceded by :// or already in href)
  escaped = escaped.replace(/(^|[^"'>=\/])(www\.[^\s<]+[^<.,:;"')\]\s])/gi, (match, prefix, url) => {
    return `${prefix}<a href="https://${url}" target="_blank" rel="noopener noreferrer" class="reward-link" onclick="event.stopPropagation();">${url}</a>`;
  });

  return escaped;
}

async function renderRewardsDashboard() {
  if (!currentSession) return;
  
  const choregroupID = currentSession.choregroup_id;
  const isParent = currentSession.role === "admin";
  
  const tabContentId = isParent ? "parent-rewards-content" : "kid-rewards-content";
  setTabLoading(tabContentId, true);
  
  const listContainer = document.getElementById('parent-rewards-list');
  const fulfillContainer = document.getElementById('parent-rewards-fulfillment-list');
  const availableContainer = document.getElementById('kid-rewards-list');
  const myPendingContainer = document.getElementById('kid-rewards-my-pending-list');
  const voteContainer = document.getElementById('kid-rewards-pending-list');
  
  try {
    const [rewardsData, statsData, membersData, purchasesData, fetchedDailyBonus] = await Promise.all([
      apiCall(`/choregroups/${choregroupID}/rewards`).catch(() => []),
      apiCall(`/choregroups/${choregroupID}/statistics`).catch(() => ({ users: [], cooperative_points: 0 })),
      apiCall(`/choregroups/${choregroupID}/members`).catch(() => []),
      apiCall(`/choregroups/${choregroupID}/purchases`).catch(() => []),
      !isParent ? apiCall(`/choregroups/${choregroupID}/daily-bonus`).catch(() => null) : Promise.resolve(null)
    ]);
    const rewards = rewardsData || [];
    const stats = statsData || { users: [], cooperative_points: 0 };
    const userStats = (stats.users || []).find(u => u.id === currentSession.user_id);
    const myPoints = userStats ? userStats.points : 0;
    const groupPoints = stats.cooperative_points || 0;
    
    const members = membersData || [];
    const membersMap = {};
    members.forEach(m => {
      membersMap[m.id] = m.username;
    });

    if (!isParent) {
      if (userStats) {
        currentSession.points = userStats.points;
        currentSession.cooperative_points = groupPoints;
      }
      if (fetchedDailyBonus) {
        currentDailyBonus = fetchedDailyBonus;
        currentSession.spins_available = fetchedDailyBonus.spins_available || 0;
      }
      localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
      updateHeaderAuthBtn();
      if (fetchedDailyBonus) {
        renderDailyBonusBar(fetchedDailyBonus);
      }
    }
    
    // Process purchases to handle one-time rewards logic
    const allPurchases = purchasesData || [];
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
            }

            const costClass = isCoop ? 'reward-cost-display coop-theme' : 'reward-cost-display';
            const { emoji, title } = parseChoreTitle(reward.name);
            const rewardIconUrl = reward.icon_url ? resolveIconUrl(reward.icon_url) : '';
            const adminButtons = `
              <button class="btn btn-outline btn-sm" onclick="openEditRewardModal('${reward.id}')">✏️</button>
              <button class="btn btn-outline btn-sm" style="color: var(--destructive-red); border-color: var(--destructive-red);" onclick="handleDeleteReward('${reward.id}')">🗑️</button>
            `;
            
            const coinHtml = isCoop ? `<span class="points-coin coop-theme">🐱🐱</span>` : `<span class="points-coin">🐱</span>`;
            return `
              <div id="parent-reward-${reward.id}" data-reward-id="${reward.id}" class="reward-card ${rewardIconUrl ? 'has-photo' : ''}" ${rewardIconUrl ? `data-photo-url="${rewardIconUrl}" data-chore-title="${escapeHTML(title)}"` : ''}>
                <div class="card-top">
                  <div class="card-emoji-box">${renderItemIcon(reward, emoji, title)}</div>
                  <div class="card-details">
                    <h3>${escapeHTML(title)}</h3>
                    <p class="reward-desc">${formatDescriptionWithLinks(reward.description || '')}</p>
                    ${displayNote}
                  </div>
                </div>
                <div class="card-actions-row">
                  <div class="card-points-badge ${costClass}">
                    ${cost} ${coinHtml}
                  </div>
                  <div class="card-controls">
                    ${adminButtons}
                  </div>
                </div>
              </div>
            `;
          }).join('');
        }
      }

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
            
            const cost = reward ? reward.cost : 0;
            const isCoop = reward && reward.type === 'cooperative';
            const costClass = isCoop ? 'reward-cost-display coop-theme' : 'reward-cost-display';
            const coinHtml = isCoop ? `<span class="points-coin coop-theme">🐱🐱</span>` : `<span class="points-coin">🐱</span>`;
            const { emoji, title } = parseChoreTitle(rewardName);
            const rewardIconUrl = (reward && reward.icon_url) ? resolveIconUrl(reward.icon_url) : '';
            
            return `
              <div class="pending-item ${rewardIconUrl ? 'has-photo' : ''}" ${rewardIconUrl ? `data-photo-url="${rewardIconUrl}" data-chore-title="${escapeHTML(title)}"` : ''}>
                <div class="card-top">
                  <div class="card-emoji-box">${renderItemIcon(reward, emoji, title)}</div>
                  <div class="card-details">
                    <h3>${escapeHTML(title)}</h3>
                    <p>Purchased by <strong>${escapeHTML(buyer)}</strong></p>
                    ${reward && reward.description ? `<p class="reward-desc">${formatDescriptionWithLinks(reward.description)}</p>` : ''}
                  </div>
                </div>
                <div class="card-actions-row">
                  <div class="card-points-badge ${costClass}">
                    ${cost} ${coinHtml}
                  </div>
                  <div class="card-controls">
                    <button class="btn btn-primary btn-sm" onclick="handleFulfillPurchase('${purchase.id}')">📦 Mark as Fulfilled</button>
                  </div>
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
            }

            const costClass = isCoop ? 'reward-cost-display coop-theme' : 'reward-cost-display';
            const { emoji, title } = parseChoreTitle(reward.name);
            const rewardIconUrl = reward.icon_url ? resolveIconUrl(reward.icon_url) : '';
            const coinHtml = isCoop ? `<span class="points-coin coop-theme">🐱🐱</span>` : `<span class="points-coin">🐱</span>`;
            const buttonClass = isCoop ? 'btn btn-primary btn-sm coop-theme' : 'btn btn-primary btn-sm';
            const buyButton = `<button class="${buttonClass}" ${buyDisabled ? 'disabled' : ''} onclick="handleBuyReward('${reward.id}', '${reward.name.replace(/'/g, "\\'")}')">${cost} ${coinHtml}</button>`;
            
            return `
              <div id="kid-reward-${reward.id}" data-reward-id="${reward.id}" class="reward-card ${buyDisabled ? 'disabled' : ''} ${rewardIconUrl ? 'has-photo' : ''}" ${rewardIconUrl ? `data-photo-url="${rewardIconUrl}" data-chore-title="${escapeHTML(title)}"` : ''}>
                <div class="card-top">
                  <div class="card-emoji-box">${renderItemIcon(reward, emoji, title)}</div>
                  <div class="card-details">
                    <h3>${escapeHTML(title)}</h3>
                    <p class="reward-desc">${formatDescriptionWithLinks(reward.description || '')}</p>
                    ${displayNote}
                  </div>
                </div>
                <div class="card-actions-row">
                  <div class="card-points-badge"></div>
                  <div class="card-controls">
                    ${buyButton}
                  </div>
                </div>
              </div>
            `;
          }).join('');
        }
      }

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
            const statusLabel = purchase.status === 'pending_approval' ? '⏳ Voting...' : '✅ Approved (Awaiting Admin)';
            const cost = reward ? reward.cost : 0;
            const isCoop = reward && reward.type === 'cooperative';
            const costClass = isCoop ? 'reward-cost-display coop-theme' : 'reward-cost-display';
            const coinHtml = isCoop ? `<span class="points-coin coop-theme">🐱🐱</span>` : `<span class="points-coin">🐱</span>`;
            const { emoji, title } = parseChoreTitle(rewardName);
            const rewardIconUrl = (reward && reward.icon_url) ? resolveIconUrl(reward.icon_url) : '';
            return `
              <div class="pending-item ${rewardIconUrl ? 'has-photo' : ''}" ${rewardIconUrl ? `data-photo-url="${rewardIconUrl}" data-chore-title="${escapeHTML(title)}"` : ''}>
                <div class="card-top">
                  <div class="card-emoji-box">${renderItemIcon(reward, emoji, title)}</div>
                  <div class="card-details">
                    <h3>${escapeHTML(title)}</h3>
                    <p>${statusLabel}</p>
                    ${reward && reward.description ? `<p class="reward-desc">${formatDescriptionWithLinks(reward.description)}</p>` : ''}
                  </div>
                </div>
                <div class="card-actions-row">
                  <div class="card-points-badge ${costClass}">
                    ${cost} ${coinHtml}
                  </div>
                  <div class="card-controls">
                    <button class="btn btn-outline btn-sm" style="color: var(--destructive-red); border-color: var(--destructive-red);" onclick="handleCancelPurchase('${purchase.id}')">❌ Cancel</button>
                  </div>
                </div>
              </div>
            `;
          }).join('');
        }
      }

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
            const cost = reward ? reward.cost : 0;
            const isCoop = reward && reward.type === 'cooperative';
            const costClass = isCoop ? 'reward-cost-display coop-theme' : 'reward-cost-display';
            const coinHtml = isCoop ? `<span class="points-coin coop-theme">🐱🐱</span>` : `<span class="points-coin">🐱</span>`;
            const { emoji, title } = parseChoreTitle(rewardName);
            const rewardIconUrl = (reward && reward.icon_url) ? resolveIconUrl(reward.icon_url) : '';
            return `
              <div class="pending-item ${rewardIconUrl ? 'has-photo' : ''}" ${rewardIconUrl ? `data-photo-url="${rewardIconUrl}" data-chore-title="${escapeHTML(title)}"` : ''}>
                <div class="card-top">
                  <div class="card-emoji-box">${renderItemIcon(reward, emoji, title)}</div>
                  <div class="card-details">
                    <h3>${escapeHTML(title)}</h3>
                    <p>Initiated by <strong>${escapeHTML(requester)}</strong></p>
                    ${reward && reward.description ? `<p class="reward-desc">${formatDescriptionWithLinks(reward.description)}</p>` : ''}
                  </div>
                </div>
                <div class="card-actions-row">
                  <div class="card-points-badge ${costClass}">
                    ${cost} ${coinHtml}
                  </div>
                  <div class="card-controls">
                    <button class="btn btn-outline btn-sm" style="color: var(--success-green); border-color: var(--success-green);" onclick="handleVotePurchase('${purchase.id}', 'approved')">✔️ Approve</button>
                    <button class="btn btn-outline btn-sm" style="color: var(--destructive-red); border-color: var(--destructive-red);" onclick="handleVotePurchase('${purchase.id}', 'rejected')">❌ Reject</button>
                  </div>
                </div>
              </div>
            `;
          }).join('');
        }
      }
      
      updateLeaderboards();
    }
    setTabLoading(tabContentId, false);
  } catch (err) {
    console.error("Failed to render rewards dashboard:", err);
    setTabLoading(tabContentId, false);
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
  
  clearIconUpload("reward");
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
    
    if (reward.icon_url) {
      document.getElementById("reward-icon-url-input").value = reward.icon_url;
      document.getElementById("reward-icon-preview-img").src = reward.icon_url;
      document.getElementById("reward-icon-preview-container").style.display = "flex";
    } else {
      clearIconUpload("reward");
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
  const iconUrl = document.getElementById("reward-icon-url-input") ? (document.getElementById("reward-icon-url-input").value || null) : null;
  
  const payload = {
    name: name,
    description: description || null,
    cost: cost,
    type: type,
    assigned_to_user_id: (type === 'individual' && assigneeId) ? assigneeId : null,
    icon_url: iconUrl
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

async function handleRemoveExpiration(taskId) {
  const groupId = currentSession.choregroup_id;
  try {
    const task = activeTasks.find(t => t.id === taskId);
    if (!task) {
      alert("Failed to find chore to update.");
      return;
    }
    
    await apiCall(`/choregroups/${groupId}/tasks/${taskId}`, "PUT", {
      assigned_to_user_id: task.assigned_to_user_id || null,
      points_reward: task.points_reward,
      title: task.title,
      type: task.type,
      is_mandatory: task.is_mandatory,
      expires_at: null,
      parent_task_id: task.parent_task_id || null,
      icon_url: task.icon_url || null
    });
    
    showToast("Expiration removed! Chore is now a normal task. ✨");
    await renderParentDashboard();
    updateLeaderboards();
  } catch (err) {
    alert("Failed to remove expiration: " + err.message);
  }
}

/* ==========================================================
   WEB PUSH NOTIFICATIONS
   ========================================================== */

async function registerPushNotifications() {
  if (!currentSession || !currentSession.choregroup_id) return;
  if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
    console.warn('Push notifications not supported in this browser.');
    return;
  }

  // Only auto-register if permission is already granted
  if (Notification.permission !== 'granted') {
    return;
  }

  try {
    // Register the service worker
    const registration = await navigator.serviceWorker.register('/sw.js');
    console.log('Service Worker registered:', registration.scope);

    // Check if already subscribed
    let subscription = await registration.pushManager.getSubscription();
    
    if (!subscription) {
      // Get the VAPID public key from the backend
      const vapidResp = await apiCall(`/choregroups/${currentSession.choregroup_id}/vapid-public-key`, "GET");
      if (!vapidResp || !vapidResp.public_key) {
        console.warn('No VAPID public key from server.');
        return;
      }

      // Convert the VAPID key from base64url to Uint8Array
      const vapidKey = urlBase64ToUint8Array(vapidResp.public_key);

      // Subscribe to push
      subscription = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: vapidKey
      });
      console.log('Push subscription created.');
    }

    // Send the subscription to the backend
    const subJSON = subscription.toJSON();
    await apiCall(`/choregroups/${currentSession.choregroup_id}/push-subscribe`, "POST", {
      endpoint: subJSON.endpoint,
      keys: {
        p256dh: subJSON.keys.p256dh,
        auth: subJSON.keys.auth
      }
    });
    console.log('Push subscription saved on server.');
  } catch (err) {
    console.error('Failed to register push notifications:', err);
  }
}

window.enablePushNotificationsClicked = async () => {
  try {
    const permission = await Notification.requestPermission();
    if (permission === 'granted') {
      showToast("🔔 Push notifications enabled! 🚀");
      await registerPushNotifications();
    } else {
      showToast("⚠️ Notification permission was denied.");
    }
  } catch (err) {
    console.error("Failed to enable push notifications:", err);
  }
  hidePushBanners();
};

window.dismissPushBanner = () => {
  sessionStorage.setItem("push_banner_dismissed", "true");
  hidePushBanners();
};

function hidePushBanners() {
  const pBanner = document.getElementById("push-banner-parent");
  const kBanner = document.getElementById("push-banner-kid");
  if (pBanner) pBanner.style.display = "none";
  if (kBanner) kBanner.style.display = "none";
}

function checkAndShowPushBanners() {
  if (!currentSession) return;
  
  if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
    return;
  }
  
  if (Notification.permission === 'granted' || sessionStorage.getItem("push_banner_dismissed") === "true") {
    return;
  }

  const pBanner = document.getElementById("push-banner-parent");
  const kBanner = document.getElementById("push-banner-kid");
  
  if (currentSession.role === "admin" && pBanner) {
    pBanner.style.display = "flex";
  } else if (currentSession.role === "user" && kBanner) {
    kBanner.style.display = "flex";
  }
}

// Helper: convert base64url string to Uint8Array for applicationServerKey
function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - base64String.length % 4) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const rawData = window.atob(base64);
  const outputArray = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}


/* ==========================================================
   KID DELEGATED AUTHENTICATION LOGIC
   ========================================================== */

window.shareAppWithFriend = async () => {
  const homepageLink = `${window.location.origin}${window.location.pathname}`;
  const shareTitle = "ChoreCraft - Fun Household Chore Management";
  const shareText = "Try ChoreCraft! It's an amazing way to manage household tasks and rewards: ";

  if (navigator.share) {
    try {
      await navigator.share({
        title: shareTitle,
        text: `${shareText}${homepageLink}`
      });
      showToast("App shared successfully! 🚀");
      return;
    } catch (err) {
      if (err.name === "AbortError") {
        return;
      }
      console.error("Web Share failed:", err);
    }
  }

  const text = encodeURIComponent(`${shareText}${homepageLink}`);
  window.location.href = `whatsapp://send?text=${text}`;
};

async function generateKidAccessLink(userID, username = "", role = "user") {
  try {
    const groupId = currentSession.choregroup_id;
    const res = await apiCall(`/choregroups/${groupId}/members/${userID}/login-link`, "POST");
    const link = `${window.location.origin}${window.location.pathname}?login_token=${res.token}`;

    const displayUsername = username || "Family Member";

    // Shorten the link using TinyURL with a graceful local fallback
    let finalLink = link;
    try {
      const response = await fetch(`https://tinyurl.com/api-create.php?url=${encodeURIComponent(link)}`);
      if (response.ok) {
        const text = await response.text();
        if (text && text.startsWith("http")) {
          finalLink = text.trim();
        }
      }
    } catch (err) {
      console.warn("TinyURL shortening failed, using original link:", err);
    }

    // Try native share if on mobile/supported browser first
    if (navigator.share) {
      const shareTitle = "ChoreCraft Access Link";
      const shareText = role === "admin"
        ? `Log in to ChoreCraft as ${displayUsername}:`
        : `Log in to ChoreCraft:`;
      try {
        await navigator.share({
          title: shareTitle,
          text: shareText,
          url: finalLink
        });
        showToast("Shared successfully! 🚀");
        return; // Success, bypass showing webpage modal
      } catch (err) {
        if (err.name !== "AbortError") {
          console.error("Web Share failed:", err);
        } else {
          // User aborted/canceled the native share sheet, stop showing the modal too
          return;
        }
      }
    }

    // Fallback: Set input value and show modal (for desktop)
    const input = document.getElementById("generated-login-link-input");
    if (input) {
      input.value = finalLink;
    }

    const titleEl = document.getElementById("login-link-modal-title");
    if (titleEl) {
      titleEl.innerHTML = `Share Profile 🚀`;
    }

    const modal = document.getElementById("login-link-modal");
    if (modal) {
      modal.classList.add("active");
    }

    const mobileShare = document.querySelector(".mobile-only-share");
    if (mobileShare) {
      const isMobile = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent);
      mobileShare.style.display = (isMobile || navigator.share) ? "block" : "none";
    }
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
  const text = encodeURIComponent("Tap this link to log in to ChoreCraft! 🏠✨\n" + link);
  window.location.href = `whatsapp://send?text=${text}`;
  setTimeout(() => {
    window.open(`https://api.whatsapp.com/send?text=${text}`, "_blank");
  }, 1200);
  closeModal("login-link-modal");
}

function shareViaSMS() {
  const link = getShareLink();
  if (!link) return;
  const text = encodeURIComponent("Log in to ChoreCraft: " + link);
  const ua = navigator.userAgent || "";
  const smsPrefix = /iP(hone|od|ad)/i.test(ua) ? "sms:&body=" : "sms:?body=";
  window.location.href = `${smsPrefix}${text}`;
  closeModal("login-link-modal");
}

function shareViaEmail() {
  const link = getShareLink();
  if (!link) return;
  const subject = encodeURIComponent("ChoreCraft - Access Link");
  const body = encodeURIComponent("Tap this link to log in to ChoreCraft! 🏠✨\n\n" + link + "\n\nThis link is valid for 5 minutes.");
  window.location.href = `mailto:?subject=${subject}&body=${body}`;
  closeModal("login-link-modal");
}

async function handleDelegatedLogin(token) {
  try {
    const res = await apiCall("/login/delegated", "POST", { token: token });
    setSession({
      id: res.user_id,
      username: res.username || (res.role === "admin" ? "Admin" : "User"),
      role: res.role,
      choregroup_id: res.choregroup_id,
      choregroup_name: res.role === "admin" ? "My Household" : "User Space",
      profile_image_url: res.profile_image_url || null
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
window.handleRemoveExpiration = handleRemoveExpiration;
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
window.startSubmission = submitChoreForApproval;
window.simulateHeroChoreCompletion = simulateHeroChoreCompletion;
window.switchParentRewardSubTab = switchParentRewardSubTab;
window.generateKidAccessLink = generateKidAccessLink;
window.copyDelegatedLink = copyDelegatedLink;
window.shareViaWhatsApp = shareViaWhatsApp;
window.shareViaSMS = shareViaSMS;
window.shareViaEmail = shareViaEmail;
window.enablePushNotificationsClicked = enablePushNotificationsClicked;
window.dismissPushBanner = dismissPushBanner;
window.deleteFamilyMember = deleteFamilyMember;
window.openChangePasswordModal = openChangePasswordModal;
window.handleSavePassword = handleSavePassword;
window.openAvatarModal = openAvatarModal;
window.openModal = openModal;
window.switchAvatarTab = switchAvatarTab;
window.handleAvatarFileUpload = handleAvatarFileUpload;
window.resetAvatarToDefault = resetAvatarToDefault;
window.saveSelectedAvatar = saveSelectedAvatar;
window.handleIconUpload = handleIconUpload;
window.clearIconUpload = clearIconUpload;
window.openCameraModal = openCameraModal;
window.closeCameraModal = closeCameraModal;
window.snapCameraPhoto = snapCameraPhoto;
window.getAiLanguage = getAiLanguage;
window.handleAiLanguageChange = handleAiLanguageChange;
window.openHouseholdAiModal = openHouseholdAiModal;
window.updateAiInstructionsCharCount = updateAiInstructionsCharCount;
window.clearHouseholdAiInstructions = clearHouseholdAiInstructions;
window.saveHouseholdAiInstructions = saveHouseholdAiInstructions;

// --- AI LANGUAGE PREFERENCE ---
function getAiLanguage() {
  return localStorage.getItem("chorecraft_ai_language") || "he";
}

async function handleAiLanguageChange(event) {
  const newLang = event.target.value;
  localStorage.setItem("chorecraft_ai_language", newLang);
  if (currentSession) {
    currentSession.ai_language = newLang;
    localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
    const groupId = currentSession.choregroup_id;
    if (groupId && currentSession.role === "admin") {
      try {
        await apiCall(`/choregroups/${groupId}/ai/language`, "PUT", { language: newLang });
        showToast("🌐 Household AI Language updated & saved");
        return;
      } catch (err) {
        console.error("Failed to update household AI language on server:", err);
      }
    }
  }
  showToast("🌐 AI Language updated");
}

// --- HOUSEHOLD AI CUSTOM INSTRUCTIONS ---
async function openHouseholdAiModal() {
  if (!currentSession || currentSession.role !== "admin") {
    showToast("⚠️ Only household admins can configure AI rules.");
    return;
  }
  const textarea = document.getElementById("household-ai-instructions-textarea");
  if (textarea) {
    textarea.value = currentSession.ai_instructions || "";
    updateAiInstructionsCharCount();
  }

  // Also fetch the freshest version from the server if choregroup_id is available
  if (currentSession.choregroup_id) {
    try {
      const group = await apiCall(`/choregroups/${currentSession.choregroup_id}`, "GET");
      if (group && group.ai_instructions !== undefined) {
        currentSession.ai_instructions = group.ai_instructions || "";
        localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
        if (textarea) {
          textarea.value = currentSession.ai_instructions;
          updateAiInstructionsCharCount();
        }
      }
    } catch (err) {
      console.warn("Could not fetch latest household AI rules:", err);
    }
  }

  openModal("household-ai-modal");
}

function updateAiInstructionsCharCount() {
  const textarea = document.getElementById("household-ai-instructions-textarea");
  const countEl = document.getElementById("ai-instructions-char-count");
  if (textarea && countEl) {
    const len = textarea.value.length;
    countEl.textContent = `${len} / 2000`;
    if (len > 1900) {
      countEl.style.color = "var(--destructive-red, #ef4444)";
    } else {
      countEl.style.color = "var(--text-muted)";
    }
  }
}

function clearHouseholdAiInstructions() {
  const textarea = document.getElementById("household-ai-instructions-textarea");
  if (textarea) {
    textarea.value = "";
    updateAiInstructionsCharCount();
    textarea.focus();
  }
}

async function saveHouseholdAiInstructions() {
  if (!currentSession || currentSession.role !== "admin") {
    showToast("⚠️ Only household admins can configure AI rules.");
    return;
  }
  const textarea = document.getElementById("household-ai-instructions-textarea");
  if (!textarea) return;
  const instructions = textarea.value.trim();
  if (instructions.length > 2000) {
    showToast("⚠️ Instructions too long (max 2000 characters).");
    return;
  }

  const saveBtn = document.getElementById("save-ai-instructions-btn");
  if (saveBtn) {
    saveBtn.disabled = true;
    saveBtn.textContent = "Saving...";
  }

  try {
    const res = await apiCall(`/choregroups/${currentSession.choregroup_id}/ai/instructions`, "PUT", {
      instructions: instructions
    });
    currentSession.ai_instructions = instructions;
    localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
    showToast("✨ Household AI Rules saved!");
    closeModal("household-ai-modal");
  } catch (err) {
    console.error("Failed to save household AI instructions:", err);
    showToast("⚠️ Failed to save AI rules: " + (err.message || "Unknown error"));
  } finally {
    if (saveBtn) {
      saveBtn.disabled = false;
      saveBtn.textContent = "Save Rules ✨";
    }
  }
}

// --- AI IMAGE-TO-CHORE / REWARD CREATION ---

let aiPickerTargetMode = 'chore';
let aiCurrentModalType = 'chore';

function triggerAiPhotoPicker(defaultMode = 'chore') {
  aiPickerTargetMode = defaultMode;
  openCameraModal('ai-' + defaultMode);
}

function openAiFileFallback() {
  closeCameraModal();
  const input = document.getElementById("ai-image-file-input");
  if (input) {
    input.value = "";
    input.click();
  }
}

async function handleAiFileSelected(event) {
  const file = event.target.files && event.target.files[0];
  if (!file) return;
  await processAiImageFile(file, aiPickerTargetMode);
}

// Compress high-resolution images client-side before uploading to optimize speed & payload size
async function compressImageFile(file) {
  if (!file || !file.type || !file.type.startsWith("image/")) {
    return file;
  }
  return new Promise((resolve) => {
    const img = new Image();
    const url = URL.createObjectURL(file);
    img.onload = () => {
      URL.revokeObjectURL(url);
      // Resize to max 400px for AI vision analysis — optimal balance for instant encoding & accuracy
      const maxDim = 400;
      let width = img.width;
      let height = img.height;
      if (width > maxDim || height > maxDim) {
        if (width > height) {
          height = Math.round((height * maxDim) / width);
          width = maxDim;
        } else {
          width = Math.round((width * maxDim) / height);
          height = maxDim;
        }
      }
      const canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      const ctx = canvas.getContext("2d");
      ctx.drawImage(img, 0, 0, width, height);
      canvas.toBlob(
        (blob) => {
          if (blob) {
            const compressed = new File([blob], file.name || `photo_${Date.now()}.jpg`, {
              type: "image/jpeg",
              lastModified: Date.now()
            });
            resolve(compressed);
          } else {
            resolve(file);
          }
        },
        "image/jpeg",
        0.60
      );
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      resolve(file);
    };
    img.src = url;
  });
}

async function processAiImageFile(file, defaultMode = 'chore') {
  const groupId = currentSession ? currentSession.choregroup_id : null;
  if (!groupId) {
    alert("Please log in first to use AI photo analysis.");
    return;
  }

  const isKid = currentSession && currentSession.role === "user";
  const isHebrew = (getAiLanguage() === 'he');

  const isChore = (defaultMode !== 'reward');
  const itemLabel = isChore ? (isHebrew ? 'מטלה' : 'Chore') : (isHebrew ? 'פרס' : 'Reward');
  const titleText = isHebrew ? `יוצר ${itemLabel}...` : `Creating ${itemLabel}...`;
  const badgeText = isHebrew ? `⚡ מעבד...` : `⚡ Analyzing...`;
  const initialStatus = isHebrew ? `📸 מעלה ומעבד את התמונה...` : `📸 Preparing & uploading photo...`;

  // Use the appropriate container for the loading placeholder based on role and mode
  let containerId;
  if (isKid) {
    containerId = isChore ? 'kid-chores-list' : 'kid-rewards-list';
  } else {
    containerId = isChore ? 'parent-chores-list' : 'parent-rewards-list';
  }
  const container = document.getElementById(containerId);
  const placeholderId = "ai-loading-card-" + Date.now();
  const TOTAL_TIMEOUT_SECONDS = 40;

  // Create sleek grayed-out loading placeholder card with animated shimmering progress bar
  if (container) {
    const loadingCardHtml = `
      <div id="${placeholderId}" class="reward-card ai-loading-card" style="opacity: 0.95; background: var(--card-bg, #ffffff); border: 2px dashed #8b5cf6; pointer-events: none; transition: all 0.3s ease; box-shadow: 0 4px 15px rgba(139, 92, 246, 0.15); margin-bottom: 1rem;">
        <div class="card-top" style="display: flex; gap: 0.85rem; align-items: center; padding: 0.85rem 1rem;">
          <div class="card-emoji-box ai-pulsing-emoji" style="display: flex; align-items: center; justify-content: center; font-size: 1.75rem; width: 48px; height: 48px; border-radius: 14px; background: linear-gradient(135deg, rgba(139,92,246,0.15), rgba(236,72,153,0.15)); flex-shrink: 0;">✨</div>
          <div class="card-details" style="flex: 1; min-width: 0;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.35rem;">
              <h3 style="margin: 0; font-size: 1rem; font-weight: 700; color: #8b5cf6;">${titleText}</h3>
              <span style="font-size: 0.78rem; font-weight: 700; color: #8b5cf6; background: rgba(139,92,246,0.12); padding: 2px 9px; border-radius: 999px; white-space: nowrap;">${badgeText}</span>
            </div>
            <p id="${placeholderId}-status" style="margin: 0 0 0.45rem 0; font-size: 0.8rem; color: var(--text-muted); line-height: 1.2;">
              ${initialStatus}
            </p>
            <div class="ai-progress-track">
              <div class="ai-progress-shimmer"></div>
            </div>
          </div>
        </div>
      </div>
    `;
    const tempWrapper = document.createElement('div');
    tempWrapper.innerHTML = loadingCardHtml.trim();
    if (container.firstChild) {
      container.insertBefore(tempWrapper.firstElementChild, container.firstChild);
    } else {
      container.appendChild(tempWrapper.firstElementChild);
    }
  }

  // Live timer interval to update dynamic status phases based on elapsed time
  let elapsedSec = 0;
  const progressInterval = setInterval(() => {
    elapsedSec++;
    const statusEl = document.getElementById(`${placeholderId}-status`);
    if (statusEl) {
      if (isHebrew) {
        if (elapsedSec < 3) {
          statusEl.textContent = "📸 מעלה ומעבד את התמונה...";
        } else if (elapsedSec < 8) {
          statusEl.textContent = isChore ? "🧠 Gemini AI מזהה חפצים ומטלות..." : "🧠 Gemini AI מזהה את הפרס...";
        } else {
          statusEl.textContent = isChore ? "🪙 מחשב מטבעות ומכין את המשימה..." : "🪙 מעריך שווי ומחשב מטבעות לפרס...";
        }
      } else {
        if (elapsedSec < 3) {
          statusEl.textContent = "📸 Preparing & uploading photo...";
        } else if (elapsedSec < 8) {
          statusEl.textContent = isChore ? "🧠 Gemini AI analyzing objects & chores..." : "🧠 Gemini AI identifying reward...";
        } else {
          statusEl.textContent = isChore ? "🪙 Calculating coins & finalizing details..." : "🪙 Appraising retail value & coins...";
        }
      }
    }
  }, 1000);

  showToast(isHebrew ? `✨ מנתח תמונה ויוצר ${itemLabel}...` : `✨ Analyzing photo & creating ${itemLabel}...`, 4000);

  const compressedFile = await compressImageFile(file);

  const formData = new FormData();
  formData.append("image", compressedFile);
  formData.append("language", getAiLanguage());
  formData.append("mode", isChore ? "chore" : "reward");

  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), TOTAL_TIMEOUT_SECONDS * 1000);

  try {
    const token = localStorage.getItem("chorecraft_token");
    const headers = {};
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    const apiBase = getApiBase();
    const response = await fetch(`${apiBase}/choregroups/${groupId}/ai/analyze-image`, {
      method: "POST",
      headers: headers,
      body: formData,
      credentials: "include",
      signal: controller.signal
    });
    clearTimeout(timeoutId);

    if (!response.ok) {
      const errText = await response.text();
      throw new Error(errText || "AI image analysis failed");
    }

    const data = await response.json();

    // If the AI says there's no real chore/reward in the image, show AI explanation in target language for extended time so child can read
    if (data.is_valid === false) {
      const fallbackMsg = isHebrew
        ? "לא זוהתה מטלה או פרס בתמונה זו! הכל נראה נקי ומסודר 😊"
        : "No chore or reward found in this photo! Everything looks tidy 😊";
      const explanation = data.invalid_reason || data.description || fallbackMsg;
      showToast(`😎 ${explanation}`, 10000); // 10s display time for child reading
      return;
    }

    // Directly create item based on active tab
    if (isChore) {
      const payload = {
        title: data.title || (isHebrew ? "✨ מטלה מהתמונה" : "✨ Task from Image"),
        type: "individual",
        points_reward: data.points_reward || 2,
        is_mandatory: false,
        icon_url: data.icon_url || null
      };
      // For kids, the backend auto-assigns individual chore to the creator with fair coin rewards
      const createdTask = await apiCall(`/choregroups/${groupId}/tasks`, "POST", payload);
      showToast(isHebrew ? "המטלה נוצרה בהצלחה מהתמונה! 🧹✨" : "Chore created from photo! 🧹✨");
      if (isKid) {
        await renderKidDashboard();
      } else {
        await renderParentDashboard();
      }

      // Smoothly scroll down to newly created chore card
      setTimeout(() => {
        const taskId = createdTask && (createdTask.id || createdTask.ID);
        let targetEl = null;
        if (taskId) {
          targetEl = document.getElementById(isKid ? `kid-task-${taskId}` : `parent-task-${taskId}`) ||
                     document.querySelector(`[data-task-id="${taskId}"]`);
        }
        if (targetEl) {
          targetEl.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
          targetEl.classList.add('newly-created-pulse');
          setTimeout(() => targetEl.classList.remove('newly-created-pulse'), 3500);
        }
      }, 150);
    } else {
      const payload = {
        name: data.title || (isHebrew ? "✨ פרס מהתמונה" : "✨ Reward from Image"),
        cost: data.points_reward || 2,
        type: "individual",
        icon_url: data.icon_url || null
      };
      const createdReward = await apiCall(`/choregroups/${groupId}/rewards`, "POST", payload);
      showToast(isHebrew ? "הפרס נוצר בהצלחה מהתמונה! 🎁✨" : "Reward created from photo! 🎁✨");
      await renderRewardsDashboard();

      // Smoothly scroll down to newly created reward card
      setTimeout(() => {
        const rewardId = createdReward && (createdReward.id || createdReward.ID);
        let targetEl = null;
        if (rewardId) {
          targetEl = document.getElementById(isKid ? `kid-reward-${rewardId}` : `parent-reward-${rewardId}`) ||
                     document.getElementById(`reward-card-${rewardId}`) ||
                     document.querySelector(`[data-reward-id="${rewardId}"]`);
        }
        if (targetEl) {
          targetEl.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
          targetEl.classList.add('newly-created-pulse');
          setTimeout(() => targetEl.classList.remove('newly-created-pulse'), 3500);
        }
      }, 150);
    }
  } catch (err) {
    clearTimeout(timeoutId);
    if (progressInterval) clearInterval(progressInterval);
    let msg = err.message || "";
    if (err.name === 'AbortError' || msg.includes('aborted') || msg.includes('abort') || msg.includes('timed out')) {
      showToast(isHebrew ? "🤖⏳ ניתוח התמונה לקח זמן רב מדי עקב עומס בשרתים. אנא המתינו 15-20 שניות ונסו שוב! 📸" : "🤖⏳ AI servers took too long (busy). Please wait 15-20 seconds and try again! 📸", 9000);
    } else if (msg.includes("429") || msg.includes("Quota") || msg.includes("rate limit") || msg.includes("RESOURCE_EXHAUSTED") || msg.includes("עמוסים") || msg.includes("busy") || msg.includes("תנומה")) {
      showToast(isHebrew ? "🤖⏳ שרתי ה-AI עמוסים כרגע (מגבלת קצב). אנא המתינו 15-20 שניות ונסו שוב! 😴✨" : "🤖⏳ AI servers are busy right now (rate limit). Please wait 15-20 seconds and try again! 😴✨", 9000);
    } else {
      showToast(msg.startsWith("🤖") ? msg : `🤖⚠️ ${msg || (isHebrew ? "ה-AI לא הצליח לנתח את התמונה כרגע. נסו שוב! 📸✨" : "The AI couldn't analyze the photo right now. Please try again! 📸✨")}`, 9000);
    }
  } finally {
    clearTimeout(timeoutId);
    if (progressInterval) clearInterval(progressInterval);
    const loader = document.getElementById(placeholderId);
    if (loader) loader.remove();
  }
}



function openAiCreateModalWithData(data) {
  const modal = document.getElementById("ai-create-modal");
  if (!modal) return;

  const mode = data.item_type || aiPickerTargetMode || 'chore';
  setAiModalType(mode);

  const isKid = currentSession && currentSession.role === "user";
  const isMandatory = (mode === 'chore' && (data.category === 'mandatory' || data.points_reward === 0));

  document.getElementById("ai-modal-image-preview").src = resolveIconUrl(data.icon_url);
  document.getElementById("ai-input-title").value = data.title || "";
  document.getElementById("ai-input-description").value = data.description || "";
  
  const ptsInput = document.getElementById("ai-input-points");
  if (ptsInput) {
    ptsInput.value = isMandatory ? 0 : (data.points_reward !== undefined ? data.points_reward : 1);
    if (isKid) {
      ptsInput.readOnly = true;
      ptsInput.style.backgroundColor = "var(--bg-muted, #f3f4f6)";
      ptsInput.style.cursor = "not-allowed";
    } else {
      ptsInput.readOnly = false;
      ptsInput.style.backgroundColor = "var(--input-bg)";
      ptsInput.style.cursor = "text";
    }
  }

  document.getElementById("ai-input-icon-url").value = data.icon_url || "";
  document.getElementById("ai-valuation-reasoning").textContent = data.valuation_reasoning || (isMandatory ? "משימת חובה (0 מטבעות)" : `$${(data.points_reward || 1) * 2} est. value`);

  const catSelect = document.getElementById("ai-input-category");
  if (catSelect) {
    if (isMandatory) {
      catSelect.value = 'mandatory';
    } else if (data.category) {
      catSelect.value = data.category;
    } else if (mode === 'chore') {
      catSelect.value = 'personal';
    }
    if (isKid) {
      catSelect.disabled = true;
    } else {
      catSelect.disabled = false;
    }
  }

  modal.classList.add("active");
}

function setAiModalType(type) {
  aiCurrentModalType = type;
  const choreBtn = document.getElementById("ai-type-chore-btn");
  const rewardBtn = document.getElementById("ai-type-reward-btn");
  const pointsLabel = document.getElementById("ai-points-label");
  const categoryLabel = document.getElementById("ai-category-label");
  const categorySelect = document.getElementById("ai-input-category");

  if (type === 'chore') {
    if (choreBtn) { choreBtn.classList.add("active", "btn-primary"); choreBtn.classList.remove("btn-outline"); }
    if (rewardBtn) { rewardBtn.classList.remove("active", "btn-primary"); rewardBtn.classList.add("btn-outline"); }
    if (pointsLabel) pointsLabel.textContent = "Coin Reward";
    if (categoryLabel) categoryLabel.textContent = "Chore Type";

    if (categorySelect) {
      categorySelect.innerHTML = `
        <option value="personal">Personal Chore (Single Assignee)</option>
        <option value="cooperative">Cooperative Chore (Group Pool)</option>
        <option value="mandatory">Mandatory Chore (0 Coins)</option>
      `;
    }
  } else {
    if (rewardBtn) { rewardBtn.classList.add("active", "btn-primary"); rewardBtn.classList.remove("btn-outline"); }
    if (choreBtn) { choreBtn.classList.remove("active", "btn-primary"); choreBtn.classList.add("btn-outline"); }
    if (pointsLabel) pointsLabel.textContent = "Coin Cost";
    if (categoryLabel) categoryLabel.textContent = "Reward Type";

    if (categorySelect) {
      categorySelect.innerHTML = `
        <option value="individual">Individual Reward (Single User)</option>
        <option value="cooperative">Cooperative Reward (Group Funded)</option>
      `;
    }
  }

  updateAiValuationHint();
}

function updateAiValuationHint() {
  const ptsInput = document.getElementById("ai-input-points");
  const reasoningDiv = document.getElementById("ai-valuation-reasoning");
  if (ptsInput && reasoningDiv) {
    const rawVal = ptsInput.value;
    const pts = parseInt(rawVal);
    if (rawVal === "0" || pts === 0) {
      reasoningDiv.textContent = "0 coins (Mandatory Task)";
    } else {
      const estPts = isNaN(pts) ? 1 : pts;
      const estUsd = estPts * 2;
      reasoningDiv.textContent = `~$${estUsd} est. value (1 coin = $2)`;
    }
  }
}

function closeAiCreateModal() {
  const modal = document.getElementById("ai-create-modal");
  if (modal) modal.classList.remove("active");
}

async function handleAiCreateSubmit(e) {
  e.preventDefault();

  const title = document.getElementById("ai-input-title").value.trim();
  const description = document.getElementById("ai-input-description").value.trim();
  const rawPts = parseInt(document.getElementById("ai-input-points").value);
  const category = document.getElementById("ai-input-category").value;
  const isMandatory = (aiCurrentModalType === 'chore' && (category === 'mandatory' || rawPts === 0));
  const points = isMandatory ? 0 : (isNaN(rawPts) ? 1 : Math.max(0, rawPts));
  const iconUrl = document.getElementById("ai-input-icon-url").value || null;
  const choregroupID = currentSession.choregroup_id;

  try {
    if (aiCurrentModalType === 'chore') {
      const payload = {
        title: title,
        type: (category === 'cooperative') ? 'cooperative' : 'individual',
        points_reward: points,
        is_mandatory: isMandatory,
        icon_url: iconUrl
      };
      const createdTask = await apiCall(`/choregroups/${choregroupID}/tasks`, "POST", payload);
      if (isMandatory) {
        showToast("⚠️ Mandatory chore created from photo!");
      } else {
        showToast("Chore created from AI photo! 🧹✨");
      }
      const isKid = currentSession && currentSession.role === "user";
      if (isKid) {
        await renderKidDashboard();
      } else {
        await renderParentDashboard();
      }
      setTimeout(() => {
        const taskId = createdTask && (createdTask.id || createdTask.ID);
        let targetEl = null;
        if (taskId) {
          targetEl = document.getElementById(isKid ? `kid-task-${taskId}` : `parent-task-${taskId}`) ||
                     document.querySelector(`[data-task-id="${taskId}"]`);
        }
        if (targetEl) {
          targetEl.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
          targetEl.classList.add('newly-created-pulse');
          setTimeout(() => targetEl.classList.remove('newly-created-pulse'), 3500);
        }
      }, 150);
    } else {
      const payload = {
        name: title,
        description: description || null,
        cost: points,
        type: (category === 'cooperative') ? 'cooperative' : 'individual',
        icon_url: iconUrl
      };
      const createdReward = await apiCall(`/choregroups/${choregroupID}/rewards`, "POST", payload);
      showToast("Reward created from AI photo! 🎁✨");
      await renderRewardsDashboard();
      setTimeout(() => {
        const rewardId = createdReward && (createdReward.id || createdReward.ID);
        let targetEl = null;
        if (rewardId) {
          targetEl = document.getElementById(isKid ? `kid-reward-${rewardId}` : `parent-reward-${rewardId}`) ||
                     document.getElementById(`reward-card-${rewardId}`) ||
                     document.querySelector(`[data-reward-id="${rewardId}"]`);
        }
        if (targetEl) {
          targetEl.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
          targetEl.classList.add('newly-created-pulse');
          setTimeout(() => targetEl.classList.remove('newly-created-pulse'), 3500);
        }
      }, 150);
    }

    closeAiCreateModal();
  } catch (err) {
    alert("Failed to create item: " + err.message);
  }
}

window.triggerAiPhotoPicker = triggerAiPhotoPicker;
window.openAiFileFallback = openAiFileFallback;
window.handleAiFileSelected = handleAiFileSelected;
window.setAiModalType = setAiModalType;
window.updateAiValuationHint = updateAiValuationHint;
window.closeAiCreateModal = closeAiCreateModal;
window.handleAiCreateSubmit = handleAiCreateSubmit;
window.openImageModal = openImageModal;
window.closeImageModal = closeImageModal;
window.toggleTheme = toggleTheme;
window.setTheme = setTheme;
window.initTheme = initTheme;
window.handleThemeToggleClick = handleThemeToggleClick;
window.handleThemeSwitchChange = handleThemeSwitchChange;

/* ==========================================================
   POINTS BACKGROUND THEMES SYSTEM
   ========================================================== */

const POINTS_THEMES = [
  { id: "default", name: "Default", badge: "🪙", isSurprise: false, lockSymbol: null, decor: null },
  { id: "autumn_leaves", name: "Autumn Leaves", badge: "🍁", isSurprise: false, lockSymbol: null, decor: "🍁" },
  { id: "golden_stars", name: "Golden Stars", badge: "✨", isSurprise: false, lockSymbol: null, decor: "✨" },
  { id: "cherry_blossom", name: "Cherry Blossom", badge: "🌸", isSurprise: true, lockSymbol: "🎁", decor: "🌸" },
  { id: "cosmic_galaxy", name: "Cosmic Galaxy", badge: "🌌", isSurprise: true, lockSymbol: "🎁", decor: "⭐" },
  { id: "emerald_nature", name: "Emerald Nature", badge: "🍃", isSurprise: true, lockSymbol: "🎁", decor: "🍃" },
  { id: "candy_party", name: "Candy Party", badge: "🍬", isSurprise: true, lockSymbol: "🎁", decor: "🍬" },
  { id: "fire_dragon", name: "Fire Dragon", badge: "🔥", isSurprise: true, lockSymbol: "🎁", decor: "🔥" },
];

function getPointsBackgroundDecorHtml(themeId) {
  switch (themeId) {
    case "autumn_leaves":
      return '<span class="pill-decor decor-tl">🍁</span><span class="pill-decor decor-tr">🍁</span><span class="pill-decor decor-bl">🍁</span><span class="pill-decor decor-br">🍁</span>';
    case "golden_stars":
      return '<span class="pill-decor decor-tl">✨</span><span class="pill-decor decor-tr">✨</span><span class="pill-decor decor-bl">✨</span><span class="pill-decor decor-br">✨</span>';
    case "cherry_blossom":
      return '<span class="pill-decor decor-tl">🌸</span><span class="pill-decor decor-tr">🌸</span><span class="pill-decor decor-bl">🌸</span><span class="pill-decor decor-br">🌸</span>';
    case "cosmic_galaxy":
      return '<span class="pill-decor decor-tl">⭐</span><span class="pill-decor decor-tr">✨</span><span class="pill-decor decor-bl">✨</span><span class="pill-decor decor-br">⭐</span>';
    case "emerald_nature":
      return '<span class="pill-decor decor-tl">🍃</span><span class="pill-decor decor-tr">🌿</span><span class="pill-decor decor-bl">🌿</span><span class="pill-decor decor-br">🍃</span>';
    case "candy_party":
      return '<span class="pill-decor decor-tl">🍬</span><span class="pill-decor decor-tr">🍭</span><span class="pill-decor decor-bl">🍭</span><span class="pill-decor decor-br">🍬</span>';
    case "fire_dragon":
      return '<span class="pill-decor decor-tl">🔥</span><span class="pill-decor decor-tr">✨</span><span class="pill-decor decor-bl">✨</span><span class="pill-decor decor-br">🔥</span>';
    default:
      return '';
  }
}

function triggerConfetti() {
  const emojis = ["🎁", "🌸", "🍁", "✨", "⭐", "🎉", "🍬", "🍃", "🔥"];
  for (let i = 0; i < 35; i++) {
    const el = document.createElement("div");
    el.textContent = emojis[Math.floor(Math.random() * emojis.length)];
    el.style.position = "fixed";
    el.style.left = `${5 + Math.random() * 90}%`;
    el.style.top = "-50px";
    el.style.fontSize = `${20 + Math.random() * 20}px`;
    el.style.zIndex = "9999";
    el.style.pointerEvents = "none";
    el.style.transition = `transform ${2.2 + Math.random() * 1.5}s cubic-bezier(0.25, 1, 0.5, 1), opacity ${2.2 + Math.random() * 1.5}s ease-out`;
    document.body.appendChild(el);

    const destX = (Math.random() - 0.5) * 260;
    const destY = window.innerHeight + 120;
    const rot = (Math.random() - 0.5) * 720;

    requestAnimationFrame(() => {
      el.style.transform = `translate(${destX}px, ${destY}px) rotate(${rot}deg)`;
      el.style.opacity = "0";
    });

    setTimeout(() => {
      if (el.parentNode) el.parentNode.removeChild(el);
    }, 4000);
  }
}

function isAllSurprisesUnlocked() {
  if (!currentSession) return false;
  if (currentSession.role === "admin") return true;
  const unlocked = Array.isArray(currentSession.unlocked_backgrounds)
    ? currentSession.unlocked_backgrounds
    : ["default", "autumn_leaves", "golden_stars"];
  const surpriseThemes = POINTS_THEMES.filter(t => t.isSurprise);
  const themesAllUnlocked = surpriseThemes.length > 0 && surpriseThemes.every(t => unlocked.includes(t.id));

  const unlockedFrames = Array.isArray(currentSession.unlocked_avatar_frames)
    ? currentSession.unlocked_avatar_frames
    : ["none"];
  const surpriseFrames = (typeof AVATAR_FRAMES !== "undefined") ? AVATAR_FRAMES.filter(f => f.isSurprise) : [];
  const framesAllUnlocked = surpriseFrames.length > 0 && surpriseFrames.every(f => unlockedFrames.includes(f.id));

  return themesAllUnlocked && framesAllUnlocked;
}

function drawOpenGiftBox(ctx, x, y) {
  ctx.save();
  // Sparkle stars bursting out from the open present
  ctx.font = "12px 'Segoe UI Emoji', 'Apple Color Emoji', sans-serif";
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.fillText("✨", x, y - 9);

  // Open lid floating tilted to the upper left
  ctx.save();
  ctx.translate(x - 1, y - 3);
  ctx.rotate(-0.38);
  ctx.fillStyle = "#f43f5e";
  if (ctx.roundRect) {
    ctx.beginPath();
    ctx.roundRect(-10, -3, 20, 5, 2);
    ctx.fill();
  } else {
    ctx.fillRect(-10, -3, 20, 5);
  }
  // Gold ribbon on lid
  ctx.fillStyle = "#fbbf24";
  ctx.fillRect(-2, -3, 4, 5);
  // Gold bow knot
  ctx.beginPath();
  ctx.arc(0, -4, 2.5, 0, Math.PI * 2);
  ctx.fillStyle = "#fef08a";
  ctx.fill();
  ctx.restore();

  // Box body
  ctx.fillStyle = "#e11d48";
  if (ctx.roundRect) {
    ctx.beginPath();
    ctx.roundRect(x - 9, y + 2, 18, 13, [0, 0, 3, 3]);
    ctx.fill();
  } else {
    ctx.fillRect(x - 9, y + 2, 18, 13);
  }
  // Gold vertical ribbon on body
  ctx.fillStyle = "#fbbf24";
  ctx.fillRect(x - 2, y + 2, 4, 13);
  ctx.restore();
}

function drawCoopCoin(ctx, x, y, r = 13) {
  const grad = ctx.createRadialGradient(x - 3, y - 3, 1, x, y, r);
  grad.addColorStop(0, "#e9d5ff");
  grad.addColorStop(0.35, "#c084fc");
  grad.addColorStop(0.7, "#8b5cf6");
  grad.addColorStop(1, "#581c87");

  ctx.save();
  ctx.beginPath();
  ctx.arc(x, y, r, 0, Math.PI * 2);
  ctx.fillStyle = grad;
  ctx.shadowColor = "rgba(0, 0, 0, 0.25)";
  ctx.shadowBlur = 4;
  ctx.fill();

  ctx.lineWidth = 1.3;
  ctx.strokeStyle = "#f3e8ff";
  ctx.stroke();

  ctx.font = `${Math.round(r * 0.72)}px 'Segoe UI Emoji', 'Apple Color Emoji', sans-serif`;
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.fillText("🐱🐱", x, y + 0.5);
  ctx.restore();
}

/* ==========================================================
   DAILY BONUS, LUCKY WHEEL & FLYING COINS ANIMATION
   ========================================================== */

let currentDailyBonus = null;
let isWheelSpinning = false;
let wheelCurrentAngle = 0;

const WHEEL_SLICES = [
  { index: 0, amountLabel: "", icon: "🪙",     type: "individual",  amount: 1, color: "#f59e0b" },
  { index: 1, amountLabel: "", icon: "coop",   type: "cooperative", amount: 1, color: "#8b5cf6" },
  { index: 2, amountLabel: "", icon: "🪙🪙",   type: "individual",  amount: 2, color: "#eab308" },
  { index: 3, amountLabel: "", icon: "🎁",     type: "surprise",    amount: 0, color: "#ec4899" },
  { index: 4, amountLabel: "", icon: "🪙",     type: "individual",  amount: 1, color: "#f59e0b" },
  { index: 5, amountLabel: "", icon: "coop",   type: "cooperative", amount: 2, color: "#7c3aed" },
  { index: 6, amountLabel: "", icon: "🪙🪙",   type: "individual",  amount: 2, color: "#eab308" },
  { index: 7, amountLabel: "", icon: "🚫",     type: "none",         amount: 0, color: "#334155" },
];

function renderDailyBonusBar(dailyBonus) {
  const container = document.getElementById("kid-daily-bonus-container");
  if (!dailyBonus || !currentSession || currentSession.role !== "user") {
    if (container) container.style.display = "none";
    return;
  }

  currentDailyBonus = dailyBonus;
  const spinsAvail = dailyBonus.spins_available || 0;
  currentSession.spins_available = spinsAvail;
  try {
    localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
  } catch (e) {}

  if (container) container.style.display = "block";

  const completed = dailyBonus.tasks_completed || 0;
  // Track starts at step 1 center and ends at step 10 center (9 intervals)
  const pct = completed <= 0 ? 0 : Math.min(100, Math.max(0, ((completed - 1) / 9) * 100));

  const fillBar = document.getElementById("daily-bonus-fill-bar");
  if (fillBar) {
    fillBar.style.width = `${pct}%`;
  }

  // Update step indicators 1..10
  for (let s = 1; s <= 10; s++) {
    const el = document.getElementById(`bonus-step-${s}`);
    if (el) {
      if (completed >= s) {
        el.classList.add("reached");
      } else {
        el.classList.remove("reached");
      }
    }
  }

  // Steps 1, 4, 7, 10 (Lucky Wheel Spins):
  // Glow gold while spins are available, turn green as done once spun
  const wheelSteps = [1, 4, 7, 10];
  const hasUnspentSpin = (dailyBonus.spins_available || 0) > 0;
  wheelSteps.forEach((stepNum) => {
    const el = document.getElementById(`bonus-step-${stepNum}`);
    if (el) {
      const isReached = completed >= stepNum;
      if (isReached && hasUnspentSpin) {
        el.classList.add("has-reward");
      } else {
        el.classList.remove("has-reward");
      }
    }
  });

  // Update header wheel badge count
  updateHeaderWheelBadge(spinsAvail);
}

function drawWheelCanvas() {
  const canvas = document.getElementById("wheel-canvas");
  if (!canvas) return;
  const ctx = canvas.getContext("2d");

  // High-DPI scaling for sharp rendering on retina mobile displays
  const dpr = window.devicePixelRatio || 1;
  const displaySize = 270;
  if (canvas.width !== displaySize * dpr || canvas.height !== displaySize * dpr) {
    canvas.width = displaySize * dpr;
    canvas.height = displaySize * dpr;
  }

  ctx.save();
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, displaySize, displaySize);

  const radius = displaySize / 2;
  const centerX = radius;
  const centerY = radius;
  const numSlices = WHEEL_SLICES.length;
  const sliceAngle = (2 * Math.PI) / numSlices;

  // 1. Draw each pie slice
  for (let i = 0; i < numSlices; i++) {
    const slice = WHEEL_SLICES[i];
    const startAngle = i * sliceAngle - Math.PI / 2;
    const endAngle = startAngle + sliceAngle;
    const midAngle = startAngle + sliceAngle / 2;

    // Wedge
    ctx.save();
    ctx.beginPath();
    ctx.moveTo(centerX, centerY);
    ctx.arc(centerX, centerY, radius - 4, startAngle, endAngle);
    ctx.closePath();
    ctx.fillStyle = slice.color;
    ctx.fill();

    // Crisp white divider stroke between slices
    ctx.lineWidth = 2;
    ctx.strokeStyle = "rgba(255, 255, 255, 0.7)";
    ctx.stroke();
    ctx.restore();

    // Prize Medallion on Slice
    ctx.save();
    ctx.translate(centerX, centerY);
    ctx.rotate(midAngle);

    const badgeX = 82;
    // White circular medallion for maximum contrast & polish
    ctx.save();
    ctx.beginPath();
    ctx.arc(badgeX, 0, 24, 0, Math.PI * 2);
    ctx.fillStyle = "#ffffff";
    ctx.shadowColor = "rgba(0, 0, 0, 0.25)";
    ctx.shadowBlur = 6;
    ctx.fill();

    // Thin accent ring
    ctx.lineWidth = 2;
    ctx.strokeStyle = slice.color;
    ctx.stroke();
    ctx.restore();

    // Prize Visual (purely visual: 1 or 2 coins, surprise gift, or red block - no numbers)
    ctx.save();
    if (slice.type === 'cooperative') {
      if (slice.amount >= 2) {
        // 2 Co-op Coins (slightly overlapping pair)
        drawCoopCoin(ctx, badgeX - 6, -3, 11);
        drawCoopCoin(ctx, badgeX + 6, 3, 11);
      } else {
        // 1 Co-op Coin (centered)
        drawCoopCoin(ctx, badgeX, 0, 13.5);
      }
    } else if (slice.type === 'individual') {
      if (slice.amount >= 2) {
        // 2 Individual Gold Coins (overlapping pair)
        ctx.font = "18px 'Segoe UI Emoji', 'Apple Color Emoji', sans-serif";
        ctx.textAlign = "center";
        ctx.textBaseline = "middle";
        ctx.fillText("🪙", badgeX - 6, -3);
        ctx.fillText("🪙", badgeX + 6, 3);
      } else {
        // 1 Individual Gold Coin (centered)
        ctx.font = "24px 'Segoe UI Emoji', 'Apple Color Emoji', sans-serif";
        ctx.textAlign = "center";
        ctx.textBaseline = "middle";
        ctx.fillText("🪙", badgeX, 0.5);
      }
    } else if (slice.type === 'surprise') {
      const allSurprisesClaimed = isAllSurprisesUnlocked();
      if (allSurprisesClaimed) {
        drawOpenGiftBox(ctx, badgeX, 0);
      } else {
        ctx.font = "24px 'Segoe UI Emoji', 'Apple Color Emoji', sans-serif";
        ctx.textAlign = "center";
        ctx.textBaseline = "middle";
        ctx.fillText("🎁", badgeX, 0.5);
      }
    } else {
      // 0 Coins: Red Block Symbol (centered)
      ctx.font = "24px 'Segoe UI Emoji', 'Apple Color Emoji', sans-serif";
      ctx.textAlign = "center";
      ctx.textBaseline = "middle";
      ctx.fillText("🚫", badgeX, 0.5);
    }
    ctx.restore();

    ctx.restore();
  }

  // 2. Perimeter Studs (golden arcade/casino lights)
  for (let i = 0; i < numSlices * 2; i++) {
    const pegAngle = (i * Math.PI) / numSlices;
    const pegX = centerX + (radius - 9) * Math.cos(pegAngle);
    const pegY = centerY + (radius - 9) * Math.sin(pegAngle);

    ctx.save();
    ctx.beginPath();
    ctx.arc(pegX, pegY, 3, 0, 2 * Math.PI);
    ctx.fillStyle = "#ffffff";
    ctx.shadowColor = "rgba(255, 255, 255, 0.9)";
    ctx.shadowBlur = 3;
    ctx.fill();

    ctx.beginPath();
    ctx.arc(pegX, pegY, 1.8, 0, 2 * Math.PI);
    ctx.fillStyle = "#fbbf24";
    ctx.fill();
    ctx.restore();
  }

  // 3. Sleek Center Axle Pin (purely decorative, rotates with wheel, no text or button)
  ctx.save();
  ctx.beginPath();
  ctx.arc(centerX, centerY, 15, 0, Math.PI * 2);
  ctx.fillStyle = "#ffffff";
  ctx.shadowColor = "rgba(0, 0, 0, 0.25)";
  ctx.shadowBlur = 4;
  ctx.fill();

  const hubGrad = ctx.createLinearGradient(centerX - 10, centerY - 10, centerX + 10, centerY + 10);
  hubGrad.addColorStop(0, "#fbbf24");
  hubGrad.addColorStop(0.6, "#f59e0b");
  hubGrad.addColorStop(1, "#d97706");
  ctx.beginPath();
  ctx.arc(centerX, centerY, 11, 0, Math.PI * 2);
  ctx.fillStyle = hubGrad;
  ctx.fill();

  ctx.beginPath();
  ctx.arc(centerX - 3, centerY - 3, 3, 0, Math.PI * 2);
  ctx.fillStyle = "rgba(255, 255, 255, 0.65)";
  ctx.fill();
  ctx.restore();

  ctx.restore();
}

async function openWheelModal(mode = 'daily') {
  const modal = document.getElementById("wheel-modal");
  if (!modal) return;

  if (!currentDailyBonus && currentSession && currentSession.role === "user") {
    await fetchAndSyncDailyBonus();
  }

  const titleEl = document.getElementById("wheel-modal-title");
  const resultBanner = document.getElementById("wheel-result-banner");
  const spinBtn = document.getElementById("wheel-spin-action-btn");
  const doneBtn = document.getElementById("wheel-done-btn");
  const spinsAvailNum = document.getElementById("wheel-spins-avail-num");

  if (resultBanner) resultBanner.style.display = "none";
  if (doneBtn) doneBtn.style.display = "none";

  const spinsLeft = (currentDailyBonus && typeof currentDailyBonus.spins_available === 'number')
    ? currentDailyBonus.spins_available
    : (typeof currentSession.spins_available === 'number' ? currentSession.spins_available : 0);
  if (spinsAvailNum) spinsAvailNum.textContent = spinsLeft;

  if (spinBtn) {
    spinBtn.style.display = "block";
    spinBtn.disabled = (spinsLeft <= 0);
  }

  if (titleEl) {
    if (currentDailyBonus && currentDailyBonus.has_claimed_welcome_spin === false) {
      titleEl.textContent = "🎉 Lucky Wheel";
    } else {
      titleEl.textContent = "🎡 Lucky Wheel";
    }
  }

  modal.classList.add("active");
  drawWheelCanvas();
}

function closeWheelModal() {
  if (isWheelSpinning) return;
  const modal = document.getElementById("wheel-modal");
  if (modal) modal.classList.remove("active");
}

async function handleSpinWheelClick() {
  if (isWheelSpinning) return;
  const spinsLeft = (currentDailyBonus && typeof currentDailyBonus.spins_available === 'number') ? currentDailyBonus.spins_available : 0;
  if (spinsLeft <= 0) return;

  const spinBtn = document.getElementById("wheel-spin-action-btn");
  const resultBanner = document.getElementById("wheel-result-banner");
  if (resultBanner) resultBanner.style.display = "none";

  if (spinBtn) spinBtn.disabled = true;
  isWheelSpinning = true;

  try {
    const groupId = currentSession ? currentSession.choregroup_id : null;
    const res = await apiCall(`/choregroups/${groupId}/wheel/spin`, "POST");

    const sliceIndex = res.slice_index;
    const numSlices = WHEEL_SLICES.length;
    const sliceDeg = 360 / numSlices; // 45 deg

    // Slice 0 starts at -90deg (top).
    // Center of slice i is at angle: i * 45 + 22.5 deg clockwise from top.
    // To align slice i with the top indicator, rotation angle:
    const sliceCenterAngle = sliceIndex * sliceDeg + sliceDeg / 2;
    const landingAngle = (360 - sliceCenterAngle) % 360;

    // Add at least 5 full spins
    const fullRotations = 5 * 360;
    const currentRotMod = wheelCurrentAngle % 360;
    let delta = (landingAngle - currentRotMod);
    if (delta < 0) delta += 360;
    wheelCurrentAngle += fullRotations + delta;

    const canvas = document.getElementById("wheel-canvas");
    if (canvas) {
      canvas.style.transition = "transform 5.2s cubic-bezier(0.15, 0.9, 0.2, 1.0)";
      canvas.style.transform = `rotate(${wheelCurrentAngle}deg)`;
    }

    // Wait for the wheel animation to conclude (5.3s)
    setTimeout(() => {
      isWheelSpinning = false;
      const doneBtn = document.getElementById("wheel-done-btn");
      if (spinBtn) spinBtn.style.display = "none";
      if (doneBtn) doneBtn.style.display = "block";

      const spinsAvailNum = document.getElementById("wheel-spins-avail-num");
      if (spinsAvailNum) spinsAvailNum.textContent = res.spins_remaining || 0;

      // Update result banner (clean icon & amount)
      const resultBanner = document.getElementById("wheel-result-banner");
      const resAmount = document.getElementById("wheel-result-amount");
      const resIcon = document.getElementById("wheel-result-icon");
      if (resultBanner) {
        resultBanner.className = "wheel-result-banner";
        resultBanner.style.display = "inline-flex";

        if (res.reward_type === "surprise") {
          resultBanner.classList.add("surprise-win");
          if (res.surprise_frame) {
            const foundFrame = (typeof AVATAR_FRAMES !== "undefined") ? AVATAR_FRAMES.find(f => f.id === res.surprise_frame) : null;
            const frameName = foundFrame ? foundFrame.name : res.surprise_frame;
            if (resAmount) resAmount.textContent = `Surprise! ${frameName} Frame`;
            if (resIcon) resIcon.textContent = "🖼️";

            setTimeout(() => {
              showSurpriseUnboxModal(null, () => {
                drawWheelCanvas();
              }, null, res.surprise_frame);
            }, 350);
          } else if (res.surprise_background) {
            const foundTheme = (typeof POINTS_THEMES !== "undefined") ? POINTS_THEMES.find(t => t.id === res.surprise_background) : null;
            const themeName = foundTheme ? (foundTheme.name || foundTheme.id) : res.surprise_background;
            if (resAmount) resAmount.textContent = `Surprise! ${themeName} Theme`;
            if (resIcon) resIcon.textContent = "🎁";

            setTimeout(() => {
              showSurpriseUnboxModal(res.surprise_background, () => {
                drawWheelCanvas();
              });
            }, 350);
          } else {
            if (resAmount) resAmount.textContent = `+${res.amount || 3}`;
            if (resIcon) resIcon.innerHTML = '<span class="points-coin">🐱</span>';
            animateCoinsFlight("individual", res.amount || 3, res.user_points, res.cooperative_points);
          }
        } else if (res.amount > 0) {
          if (res.reward_type === "cooperative") {
            resultBanner.classList.add("coop-win");
            if (resAmount) resAmount.textContent = `+${res.amount}`;
            if (resIcon) resIcon.innerHTML = '<span class="points-coin coop-theme">🐱🐱</span>';
          } else {
            resultBanner.classList.add("individual-win");
            if (resAmount) resAmount.textContent = `+${res.amount}`;
            if (resIcon) resIcon.innerHTML = '<span class="points-coin">🐱</span>';
          }
          // Trigger flying coins animation!
          animateCoinsFlight(res.reward_type, res.amount, res.user_points, res.cooperative_points);
        } else {
          resultBanner.classList.add("zero-points");
          if (resAmount) resAmount.textContent = "0";
          if (resIcon) resIcon.textContent = "🚫";
        }
      }

      // Refresh daily bonus bar and session storage
      const remaining = typeof res.spins_remaining === 'number' ? res.spins_remaining : 0;
      if (currentDailyBonus) {
        currentDailyBonus.spins_available = remaining;
        currentDailyBonus.has_claimed_welcome_spin = true;
        renderDailyBonusBar(currentDailyBonus);
      } else {
        updateHeaderWheelBadge(remaining);
      }
      if (currentSession) {
        currentSession.spins_available = remaining;
        try {
          localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
        } catch (e) {}
      }
    }, 5300);

  } catch (err) {
    console.error("Spin wheel error:", err);
    isWheelSpinning = false;
    if (spinBtn) spinBtn.disabled = false;
    alert(err.message || "Failed to spin the wheel. Please try again!");
  }
}

function animateCoinsFlight(rewardType, amount, newPoints, newCoopPoints) {
  const container = document.getElementById("flying-coins-container");
  if (!container) return;

  const wheelContainer = document.getElementById("wheel-canvas-container");
  const wheelRect = wheelContainer ? wheelContainer.getBoundingClientRect() : {
    left: window.innerWidth / 2 - 20,
    top: window.innerHeight / 2 - 20,
    width: 40,
    height: 40
  };

  const startX = wheelRect.left + wheelRect.width / 2;
  const startY = wheelRect.top + wheelRect.height / 2;

  // Determine target header element
  const targetId = rewardType === "cooperative" ? "header-coin-chip-cooperative" : "header-coin-chip-individual";
  const targetEl = document.getElementById(targetId) || document.getElementById("header-user-stats");
  
  let targetX = window.innerWidth / 2;
  let targetY = 30;
  if (targetEl) {
    const targetRect = targetEl.getBoundingClientRect();
    targetX = targetRect.left + targetRect.width / 2;
    targetY = targetRect.top + targetRect.height / 2;
  }

  const isCoop = (rewardType === "cooperative");
  const numCoins = Math.max(1, amount); // Exact correct number of coins!
  const duration = 1400; // ms: slower, graceful arc

  const indAmountEl = document.getElementById("header-coin-amount-individual");
  const coopAmountEl = document.getElementById("header-coin-amount-cooperative");

  for (let i = 0; i < numCoins; i++) {
    const delay = i * 240;
    setTimeout(() => {
      const coin = document.createElement("div");
      coin.className = "flying-coin-particle";
      coin.innerHTML = isCoop
        ? '<span class="points-coin coop-theme" style="width: 36px; height: 36px; font-size: 13px; box-shadow: 0 6px 18px rgba(139, 92, 246, 0.7);">🐱🐱</span>'
        : '<span class="points-coin" style="width: 36px; height: 36px; font-size: 18px; box-shadow: 0 6px 18px rgba(245, 158, 11, 0.7);">🐱</span>';
      coin.style.left = `${startX}px`;
      coin.style.top = `${startY}px`;
      coin.style.transform = `translate(-50%, -50%) scale(0.5)`;
      coin.style.opacity = "0";
      container.appendChild(coin);

      // Randomize spread for natural arc
      const spreadX = (Math.random() - 0.5) * 80;
      const spreadY = (Math.random() - 0.5) * 60;

      const startTime = performance.now();

      function step(currentTime) {
        const elapsed = currentTime - startTime;
        const progress = Math.min(elapsed / duration, 1);

        if (progress < 0.25) {
          const p = progress / 0.25;
          const scale = 0.5 + p * 0.9;
          coin.style.opacity = `${p}`;
          coin.style.transform = `translate(-50%, -50%) translate(${spreadX * p}px, ${spreadY * p}px) scale(${scale})`;
        } else {
          const p = (progress - 0.25) / 0.75;
          const currentX = (1 - p) * (startX + spreadX) + p * targetX;
          const currentY = Math.pow(1 - p, 2) * (startY + spreadY) + 2 * (1 - p) * p * (Math.min(startY, targetY) - 60) + Math.pow(p, 2) * targetY;
          const scale = 1.4 * (1 - p * 0.6);
          coin.style.transform = `translate(-50%, -50%) translate(${currentX - startX}px, ${currentY - startY}px) scale(${scale}) rotate(${p * 360}deg)`;
        }

        if (progress < 1) {
          requestAnimationFrame(step);
        } else {
          coin.remove();

          // Pulse target header counter as coin lands
          if (targetEl) {
            targetEl.classList.remove("coin-collect-pulse");
            void targetEl.offsetWidth; // Trigger reflow
            targetEl.classList.add("coin-collect-pulse");
            setTimeout(() => targetEl.classList.remove("coin-collect-pulse"), 700);
          }

          // Incrementally update counter for each arriving coin
          if (rewardType === "cooperative" && newCoopPoints !== undefined) {
            const currentStepVal = (newCoopPoints - amount) + (i + 1);
            if (coopAmountEl) coopAmountEl.textContent = currentStepVal;
          } else if (newPoints !== undefined) {
            const currentStepVal = (newPoints - amount) + (i + 1);
            if (indAmountEl) indAmountEl.textContent = currentStepVal;
          }

          if (i === numCoins - 1) {
            // Final coin arrived: finalize session values
            if (currentSession) {
              if (newPoints !== undefined) currentSession.points = newPoints;
              if (newCoopPoints !== undefined) currentSession.cooperative_points = newCoopPoints;
              localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
            }
            if (indAmountEl && newPoints !== undefined) indAmountEl.textContent = newPoints;
            if (coopAmountEl && newCoopPoints !== undefined) coopAmountEl.textContent = newCoopPoints;
          }
        }
      }

      requestAnimationFrame(step);
    }, delay);
  }
}

window.openWheelModal = openWheelModal;
window.closeWheelModal = closeWheelModal;
window.handleSpinWheelClick = handleSpinWheelClick;
window.drawWheelCanvas = drawWheelCanvas;

/* ==========================================================
   INTERACTIVE SURPRISE PRESENT UNBOXING SYSTEM
   ========================================================== */

let currentRevealedSurpriseThemeId = null;
let currentRevealedSurpriseFrameId = null;
let isSurpriseBoxOpening = false;
let surpriseUnboxCallback = null;

function showSurpriseUnboxModal(themeId, onComplete, bonusCoins, frameId = null) {
  currentRevealedSurpriseThemeId = themeId;
  currentRevealedSurpriseFrameId = frameId;
  surpriseUnboxCallback = onComplete;
  isSurpriseBoxOpening = false;

  const modal = document.getElementById("surprise-unbox-modal");
  const box = document.getElementById("gift-box-interactive");
  const lid = document.getElementById("gift-lid-wrapper");
  const rewardStage = document.getElementById("surprise-reward-stage");
  const titleEl = document.getElementById("surprise-theme-title");
  const previewContainer = document.getElementById("surprise-preview-container");
  const equipBtn = document.getElementById("surprise-equip-btn");

  if (!modal || !box || !lid) return;

  // Reset visual state for new reveal
  lid.classList.remove("lid-popped");
  box.style.display = "flex";
  // Restart CSS animation for energetic spring jump
  box.style.animation = "none";
  void box.offsetWidth;
  box.style.animation = "";

  if (rewardStage) rewardStage.style.display = "none";

  if (frameId) {
    const foundFrame = (typeof AVATAR_FRAMES !== "undefined") ? AVATAR_FRAMES.find(f => f.id === frameId) : null;
    const frameName = foundFrame ? foundFrame.name : frameId;
    const frameIcon = foundFrame ? foundFrame.icon : "🖼️";

    if (titleEl) {
      titleEl.textContent = `${frameIcon} ${frameName} Frame!`;
    }

    if (previewContainer) {
      const avatarUrl = currentSession ? currentSession.profile_image_url : '';
      const avatarInner = (avatarUrl && (avatarUrl.startsWith('/') || avatarUrl.startsWith('http')))
        ? `<img src="${escapeHTML(avatarUrl)}" class="member-avatar-img" alt="Avatar" style="width:100%;height:100%;border-radius:50%;object-fit:cover;" />`
        : `<span style="font-size: 2.2rem;">${escapeHTML(avatarUrl || '👤')}</span>`;

      previewContainer.innerHTML = `
        <div style="display: flex; flex-direction: column; align-items: center; gap: 10px;">
          <span class="avatar-frame-badge frame-${escapeHTML(frameId)}" style="width: 72px; height: 72px; display: inline-flex; align-items: center; justify-content: center;">
            ${avatarInner}
          </span>
          <p style="font-weight: 700; color: #3b82f6; margin-top: 0.25rem; font-size: 0.95rem;">
            New Avatar Frame Unlocked!
          </p>
        </div>
      `;
    }

    if (equipBtn) {
      equipBtn.textContent = "Equip Frame ✨";
      equipBtn.style.display = "inline-flex";
    }

    // Update session unlocked frames
    if (currentSession) {
      if (!Array.isArray(currentSession.unlocked_avatar_frames)) {
        currentSession.unlocked_avatar_frames = ["none"];
      }
      if (!currentSession.unlocked_avatar_frames.includes(frameId)) {
        currentSession.unlocked_avatar_frames.push(frameId);
      }
      try {
        localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
      } catch (e) {}
    }
  } else if (themeId) {
    const foundTheme = POINTS_THEMES.find(t => t.id === themeId);
    const themeName = foundTheme ? foundTheme.name : themeId;
    const themeBadge = foundTheme ? foundTheme.badge : "🎁";

    if (titleEl) {
      titleEl.textContent = `${themeBadge} ${themeName}`;
    }

    if (previewContainer) {
      previewContainer.innerHTML = `
        <div class="user-coins-pill theme-${themeId}">
          ${getPointsBackgroundDecorHtml(themeId)}
          <div class="coin-chip">
            <span class="coin-icon">🪙</span>
            <span class="coin-amount">15</span>
          </div>
          <div class="coin-chip coop-chip">
            <span class="coin-icon">🐱🐱</span>
            <span class="coin-amount">8</span>
          </div>
        </div>
      `;
    }

    if (equipBtn) {
      equipBtn.textContent = "Equip Theme ✨";
      equipBtn.style.display = "inline-flex";
    }

    // Update session unlocked list
    if (currentSession) {
      if (!Array.isArray(currentSession.unlocked_backgrounds)) {
        currentSession.unlocked_backgrounds = ["default", "autumn_leaves", "golden_stars"];
      }
      if (!currentSession.unlocked_backgrounds.includes(themeId)) {
        currentSession.unlocked_backgrounds.push(themeId);
      }
      try {
        localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
      } catch (e) {}
    }
  } else {
    // All surprises collected fallback coins reward
    const coins = bonusCoins || 5;
    if (titleEl) {
      titleEl.textContent = `✨ +${coins} Bonus Coins! ✨`;
    }
    if (previewContainer) {
      previewContainer.innerHTML = `
        <div style="font-size: 3.5rem; filter: drop-shadow(0 4px 12px rgba(245, 158, 11, 0.5)); animation: wheelBounce 1.5s infinite ease-in-out;">
          🪙
        </div>
        <p style="font-weight: 700; color: #f59e0b; margin-top: 0.5rem; font-size: 1.05rem;">
          All surprise cosmetics collected! Enjoy your bonus coins!
        </p>
      `;
    }
    if (equipBtn) {
      equipBtn.textContent = "Awesome! 🎉";
      equipBtn.style.display = "inline-flex";
    }
  }

  modal.style.display = "flex";
}

function handleOpenSurpriseBoxClick(e) {
  if (e && typeof e.stopPropagation === "function") e.stopPropagation();
  if (isSurpriseBoxOpening) return;
  isSurpriseBoxOpening = true;

  const lid = document.getElementById("gift-lid-wrapper");
  const box = document.getElementById("gift-box-interactive");
  const rewardStage = document.getElementById("surprise-reward-stage");

  if (lid) lid.classList.add("lid-popped");

  // Initial burst of celebratory confetti & sparkles!
  triggerConfetti();

  setTimeout(() => {
    if (box) box.style.display = "none";
    if (rewardStage) rewardStage.style.display = "flex";
    triggerConfetti();
  }, 400);
}

function handleSurpriseBackdropClick(e) {
  const box = document.getElementById("gift-box-interactive");
  const rewardStage = document.getElementById("surprise-reward-stage");
  // If box is showing, any click anywhere on screen opens it!
  if (box && box.style.display !== "none" && (!rewardStage || rewardStage.style.display === "none")) {
    handleOpenSurpriseBoxClick(e);
  } else if (e && (e.target.id === "surprise-unbox-modal" || e.target.classList.contains("surprise-unbox-backdrop"))) {
    handleDismissSurpriseModal();
  }
}

async function handleEquipSurpriseTheme() {
  const modal = document.getElementById("surprise-unbox-modal");
  if (modal) modal.style.display = "none";
  isSurpriseBoxOpening = false;

  if (currentRevealedSurpriseFrameId && currentSession) {
    try {
      const groupId = currentSession.choregroup_id;
      const userId = currentSession.user_id;
      await apiCall(`/choregroups/${groupId}/members/${userId}/avatar-frame`, "PUT", {
        avatar_frame: currentRevealedSurpriseFrameId
      });
      currentSession.avatar_frame = currentRevealedSurpriseFrameId;
      localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));
      showToast(`Equipped frame! ✨`);
    } catch (err) {
      console.error("Failed to equip avatar frame:", err);
    }
  } else if (currentRevealedSurpriseThemeId) {
    await selectPointsBackground(currentRevealedSurpriseThemeId);
  }

  if (typeof surpriseUnboxCallback === "function") {
    surpriseUnboxCallback();
  }
  drawWheelCanvas();
  updateHeaderAuthBtn();
  if (currentSession && currentSession.role === "admin") {
    renderParentDashboard();
  } else {
    renderKidDashboard();
  }
}

function handleDismissSurpriseModal() {
  const modal = document.getElementById("surprise-unbox-modal");
  if (modal) modal.style.display = "none";
  isSurpriseBoxOpening = false;

  if (typeof surpriseUnboxCallback === "function") {
    surpriseUnboxCallback();
  }
  drawWheelCanvas();
  updateHeaderAuthBtn();
}

async function handleMilestoneSurpriseClick() {
  if (!currentDailyBonus || !currentSession || currentSession.role !== "user") return;
  const completed = currentDailyBonus.tasks_completed || 0;
  if (completed < 10) {
    showToast(`Complete ${10 - completed} more tasks today to unlock the surprise! 🎁`, "info");
    return;
  }
  if (currentDailyBonus.surprise_claimed) {
    showToast("Today's surprise has already been claimed! Check back tomorrow! ✨", "info");
    return;
  }
  await claimDailySurprise();
}

async function claimDailySurprise() {
  if (!currentSession || !currentSession.choregroup_id) return;
  try {
    const res = await apiFetch(`/choregroups/${currentSession.choregroup_id}/daily-bonus/claim-surprise`, {
      method: "POST"
    });
    const data = await res.json();
    if (!res.ok) {
      showToast(data.error || data.message || "Could not claim surprise", "error");
      return;
    }

    if (currentDailyBonus) {
      currentDailyBonus.surprise_claimed = true;
      renderDailyBonusBar(currentDailyBonus);
    }

    if (data.user_points !== undefined && currentSession) {
      currentSession.points = data.user_points;
      updateHeaderAuthBtn();
    }

    // Trigger glowing unbox modal!
    if (data.frame_id) {
      showSurpriseUnboxModal(null, async () => {
        await refreshDailyBonus();
        await fetchAndRenderHistory();
      }, null, data.frame_id);
    } else if (data.theme_id) {
      showSurpriseUnboxModal(data.theme_id, async () => {
        await refreshDailyBonus();
        await fetchAndRenderHistory();
      });
    } else if (data.bonus_coins) {
      showSurpriseUnboxModal(null, async () => {
        await refreshDailyBonus();
        await fetchAndRenderHistory();
      }, data.bonus_coins);
    }
  } catch (err) {
    console.error("claimDailySurprise error:", err);
    showToast("Error claiming surprise: " + err.message, "error");
  }
}

window.handleOpenSurpriseBoxClick = handleOpenSurpriseBoxClick;
window.handleSurpriseBackdropClick = handleSurpriseBackdropClick;
window.handleEquipSurpriseTheme = handleEquipSurpriseTheme;
window.handleDismissSurpriseModal = handleDismissSurpriseModal;
window.showSurpriseUnboxModal = showSurpriseUnboxModal;
window.isAllSurprisesUnlocked = isAllSurprisesUnlocked;
window.handleMilestoneSurpriseClick = handleMilestoneSurpriseClick;
window.claimDailySurprise = claimDailySurprise;

function openPointsBackgroundModal() {
  if (!currentSession) return;
  const currentTheme = currentSession.points_background || "default";

  // Update preview numbers
  const indEl = document.getElementById("preview-coin-amount-individual");
  const coopEl = document.getElementById("preview-coin-amount-cooperative");
  if (indEl) indEl.textContent = currentSession.points || 0;
  if (coopEl) coopEl.textContent = currentSession.cooperative_points || 0;

  updatePointsBgPreview(currentTheme);
  renderPointsBackgroundGrid();
  openModal("points-bg-modal");
}

function updatePointsBgPreview(themeId) {
  const pill = document.getElementById("points-bg-preview-pill");
  if (!pill) return;

  POINTS_THEMES.forEach(t => {
    pill.classList.remove(`theme-${t.id}`);
  });
  if (themeId && themeId !== "default") {
    pill.classList.add(`theme-${themeId}`);
  }

  pill.querySelectorAll(".pill-decor").forEach(el => el.remove());
  const decorHtml = getPointsBackgroundDecorHtml(themeId);
  if (decorHtml) {
    pill.insertAdjacentHTML("afterbegin", decorHtml);
  }
}

function handleLockedCardClick(cardEl) {
  if (!cardEl) return;
  cardEl.classList.remove("shake");
  void cardEl.offsetWidth; // Trigger reflow
  cardEl.classList.add("shake");
  setTimeout(() => cardEl.classList.remove("shake"), 450);

  // Ensure preview stays on current acquired background and never shows locked theme
  if (currentSession && currentSession.points_background) {
    updatePointsBgPreview(currentSession.points_background);
  }
}

function renderPointsBackgroundGrid() {
  const grid = document.getElementById("points-bg-themes-grid");
  if (!grid || !currentSession) return;

  const isAdmin = currentSession.role === "admin";
  const currentBg = currentSession.points_background || "default";
  const unlocked = Array.isArray(currentSession.unlocked_backgrounds)
    ? currentSession.unlocked_backgrounds
    : ["default", "autumn_leaves", "golden_stars"];

  grid.innerHTML = POINTS_THEMES.map(theme => {
    const isUnlocked = isAdmin || unlocked.includes(theme.id);
    const isActive = (currentBg === theme.id);
    const cardClass = isActive ? "active" : (isUnlocked ? "unlocked" : "locked");

    let indicatorHtml = "";
    let clickAttr = "";

    if (isActive) {
      indicatorHtml = '<span class="points-card-indicator active">✓</span>';
      clickAttr = `selectPointsBackground('${theme.id}')`;
    } else if (isUnlocked) {
      clickAttr = `selectPointsBackground('${theme.id}')`;
    } else {
      const symbol = theme.lockSymbol || "🎡";
      indicatorHtml = `<span class="points-card-indicator locked-symbol">${symbol}</span>`;
      clickAttr = "handleLockedCardClick(this)";
    }

    const hoverAttr = isUnlocked
      ? `onmouseenter="updatePointsBgPreview('${theme.id}')" onmouseleave="updatePointsBgPreview((currentSession &amp;&amp; currentSession.points_background) || 'default')"`
      : "";

    return `
      <div class="points-bg-card ${cardClass}"
           onclick="${clickAttr}"
           ${hoverAttr}>
        <div class="points-bg-card-badge">${theme.badge}</div>
        ${indicatorHtml}
      </div>
    `;
  }).join("");
}

async function selectPointsBackground(themeId) {
  if (!currentSession) return;
  const groupId = currentSession.choregroup_id;
  const userId = currentSession.id || currentSession.user_id;

  try {
    const res = await apiCall(`/choregroups/${groupId}/members/${userId}/points-background`, "PUT", {
      background: themeId
    });

    currentSession.points_background = res.points_background || themeId;
    if (res.unlocked_backgrounds) {
      currentSession.unlocked_backgrounds = res.unlocked_backgrounds;
    }
    localStorage.setItem("chorecraft_session", JSON.stringify(currentSession));

    updateHeaderAuthBtn();
    updatePointsBgPreview(currentSession.points_background);
    renderPointsBackgroundGrid();
  } catch (err) {
    console.error("Failed to update points background:", err);
    showToast(err.message || "Error updating background");
  }
}

window.POINTS_THEMES = POINTS_THEMES;
window.getPointsBackgroundDecorHtml = getPointsBackgroundDecorHtml;
window.openPointsBackgroundModal = openPointsBackgroundModal;
window.updatePointsBgPreview = updatePointsBgPreview;
window.renderPointsBackgroundGrid = renderPointsBackgroundGrid;
window.selectPointsBackground = selectPointsBackground;
window.triggerConfetti = triggerConfetti;
window.handleLockedCardClick = handleLockedCardClick;

/* ==========================================================================
   HOUSEHOLD ACTIVITY HISTORY MODULE
   ========================================================================== */

let historyWeekOffset = 0; // 0 = current week, -1 = last week, etc.
let historyCurrentFilter = {
  userId: "",
  category: "all"
};

// Activity type to category mapping
function getActivityCategory(type) {
  if (!type) return "chores";
  if (type.startsWith("chore_")) return "chores";
  if (type.startsWith("reward_")) return "rewards";
  if (type.startsWith("wheel_")) return "wheel";
  if (type.startsWith("theme_")) return "themes";
  if (type.startsWith("member_")) return "members";
  return "chores";
}

// Visual badge, emoji, and styling per activity
function getActivityVisuals(event) {
  const type = event.activity_type || "";
  switch (type) {
    case "chore_created":
      return { icon: "📋", badgeClass: "cat-badge-chore", actionLabel: "Chore Created" };
    case "chore_submitted":
      return { icon: "📤", badgeClass: "cat-badge-chore", actionLabel: "Chore Submitted" };
    case "chore_approved":
      return { icon: "✅", badgeClass: "cat-badge-chore", actionLabel: "Chore Approved" };
    case "chore_rejected":
      return { icon: "❌", badgeClass: "cat-badge-chore", actionLabel: "Chore Rejected" };
    case "chore_deleted":
    case "task_deleted":
      return { icon: "🗑️📋", badgeClass: "cat-badge-chore", actionLabel: "Chore Deleted" };
    case "reward_purchased":
      return { icon: "🎁", badgeClass: "cat-badge-reward", actionLabel: "Reward Purchased" };
    case "reward_fulfilled":
      return { icon: "🏆", badgeClass: "cat-badge-reward", actionLabel: "Reward Fulfilled" };
    case "reward_cancelled":
      return { icon: "↩️", badgeClass: "cat-badge-reward", actionLabel: "Purchase Cancelled" };
    case "wheel_spun":
      return { icon: "🎡", badgeClass: "cat-badge-wheel", actionLabel: "Lucky Wheel" };
    case "theme_changed":
      return { icon: "🎨", badgeClass: "cat-badge-theme", actionLabel: "Theme Changed" };
    case "member_joined":
      return { icon: "👤", badgeClass: "cat-badge-member", actionLabel: "Member Joined" };
    default:
      return { icon: "✨", badgeClass: "cat-badge-chore", actionLabel: "Activity" };
  }
}

// Relative time formatter
function formatActivityRelativeTime(isoDateStr) {
  if (!isoDateStr) return "";
  const d = new Date(isoDateStr);
  const now = new Date();
  const diffMs = now - d;
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);
  const diffDays = Math.floor(diffHour / 24);

  if (diffSec < 60) return "Just now";
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHour < 24) return `${diffHour}h ago`;
  if (diffDays === 1) return `Yesterday ${d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
}

// Format exact time e.g. 14:30
function formatActivityTime(isoDateStr) {
  if (!isoDateStr) return "";
  const d = new Date(isoDateStr);
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

// Format Day Group Header e.g. "Today • Monday, Sep 14"
function formatActivityDayHeader(dateKey) {
  const [year, month, day] = dateKey.split('-').map(Number);
  const date = new Date(year, month - 1, day);
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(today.getDate() - 1);

  const isToday = date.getFullYear() === today.getFullYear() &&
                  date.getMonth() === today.getMonth() &&
                  date.getDate() === today.getDate();

  const isYesterday = date.getFullYear() === yesterday.getFullYear() &&
                      date.getMonth() === yesterday.getMonth() &&
                      date.getDate() === yesterday.getDate();

  const dayName = date.toLocaleDateString('en-US', { weekday: 'long' });
  const monthDay = date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });

  if (isToday) return `Today • ${dayName}, ${monthDay}`;
  if (isYesterday) return `Yesterday • ${dayName}, ${monthDay}`;
  return `${dayName} • ${monthDay}, ${year}`;
}

// Calculate week range (Sunday to Saturday) based on offsetWeeks
function getHistoryWeekRange(offsetWeeks = 0) {
  const now = new Date();
  const dayOfWeek = now.getDay(); // 0 = Sun, 1 = Mon, etc.
  const start = new Date(now);
  start.setDate(now.getDate() - dayOfWeek + (offsetWeeks * 7));
  start.setHours(0, 0, 0, 0);

  const end = new Date(start);
  end.setDate(start.getDate() + 6);
  end.setHours(23, 59, 59, 999);

  return { start, end };
}

function extractCleanItemName(rawStr) {
  if (!rawStr) return "";
  let clean = String(rawStr).trim();
  clean = clean.replace(/^[a-zA-Z0-9_\u0590-\u05FF-]+\s+(completed chore|submitted chore|deleted chore|purchased reward|deleted reward|created reward):\s*/i, '');
  clean = clean.replace(/\s*\([+-]?\d+\s*coins?\)\s*$/i, '');
  clean = clean.replace(/^["'“](.*)["'”]$/, '$1');
  return clean.trim();
}

function renderAuditFlowRow(evt, timeFormat = 'relative') {
  try {
    const username = evt.user_username || (currentSession ? currentSession.username : 'User');
    const role = evt.user_role || (currentSession ? currentSession.role : 'user');
    const isAdmin = role === 'admin';

    // 1. user_profile_image: user_name
    let avatarHtml = '';
    const profileImg = evt.user_profile_image;
    if (profileImg && (profileImg.startsWith('/') || profileImg.startsWith('http'))) {
      avatarHtml = `<span class="audit-user-avatar"><img src="${escapeHTML(profileImg)}" alt="${escapeHTML(username)}" /></span>`;
    } else if (profileImg && profileImg.trim()) {
      avatarHtml = `<span class="audit-user-avatar">${escapeHTML(profileImg)}</span>`;
    } else {
      avatarHtml = `<span class="audit-user-avatar">${isAdmin ? '👑' : '👤'}</span>`;
    }

    // 2. action_icon: action_name
    // Supported audit actions: task completed, task deleted, wheel spin, reward obtained, reward created, reward deleted
    const actType = (evt.activity_type || '').toLowerCase();
    let actionIcon = '✨';
    let actionName = 'action';
    let outcomeIcon = '';
    let outcomeDetails = '';
    let outcomeClass = '';

    const meta = evt.metadata || {};

    if (actType === 'task_completed' || actType === 'chore_approved') {
      actionIcon = '✅';
      const itemName = meta.title || extractCleanItemName(evt.description) || 'task';
      actionName = `<span class="audit-target-name">"${escapeHTML(itemName)}"</span>`;
      if (typeof meta.points === 'number') {
        if (meta.points === 0) {
          outcomeIcon = '🚨';
          outcomeDetails = '';
          outcomeClass = 'outcome-mandatory';
        } else {
          outcomeIcon = '🪙';
          outcomeDetails = `+${meta.points}`;
          outcomeClass = 'outcome-positive';
        }
      }
    } else if (actType === 'task_deleted' || actType === 'chore_deleted') {
      actionIcon = '🗑️📋';
      const itemName = meta.title || extractCleanItemName(evt.description) || 'task';
      actionName = `<span class="audit-target-name">"${escapeHTML(itemName)}"</span>`;
    } else if (actType === 'wheel_spin' || actType === 'wheel_spun') {
      actionIcon = '🎡';
      actionName = 'spun wheel';
      if (meta.surprise_frame) {
        outcomeIcon = '🖼️';
        const foundFrame = (typeof AVATAR_FRAMES !== "undefined") ? AVATAR_FRAMES.find(f => f.id === meta.surprise_frame) : null;
        const frameLabel = foundFrame ? (foundFrame.name || foundFrame.id) : meta.surprise_frame;
        outcomeDetails = `Surprise: ${escapeHTML(frameLabel)}`;
        outcomeClass = 'outcome-surprise';
      } else if (meta.surprise_background) {
        outcomeIcon = '🎁';
        const foundTheme = (typeof POINTS_THEMES !== "undefined") ? POINTS_THEMES.find(t => t.id === meta.surprise_background) : null;
        const themeLabel = foundTheme ? (foundTheme.name || foundTheme.id) : meta.surprise_background;
        outcomeDetails = `Surprise: ${escapeHTML(themeLabel)}`;
        outcomeClass = 'outcome-surprise';
      } else if (typeof meta.amount === 'number' && meta.amount > 0) {
        outcomeIcon = '🪙';
        outcomeDetails = `+${meta.amount}`;
        outcomeClass = 'outcome-positive';
      } else if (meta.label) {
        outcomeIcon = '🚫';
        outcomeDetails = `${escapeHTML(meta.label)}`;
      }
    } else if (actType === 'reward_obtained' || actType === 'reward_purchased' || actType === 'reward_fulfilled') {
      actionIcon = '🎁';
      const itemName = meta.name || extractCleanItemName(evt.description) || 'reward';
      actionName = `<span class="audit-target-name">"${escapeHTML(itemName)}"</span>`;
      if (typeof meta.cost === 'number' && meta.cost > 0) {
        outcomeIcon = '🪙';
        outcomeDetails = `-${meta.cost}`;
        outcomeClass = 'outcome-negative';
      }
    } else if (actType === 'reward_created') {
      actionIcon = '➕';
      const itemName = meta.name || extractCleanItemName(evt.description) || 'reward';
      actionName = `created reward <span class="audit-target-name">"${escapeHTML(itemName)}"</span>`;
      if (typeof meta.cost === 'number' && meta.cost > 0) {
        outcomeIcon = '🪙';
        outcomeDetails = `${meta.cost}`;
      }
    } else if (actType === 'reward_deleted') {
      actionIcon = '🗑️🎁';
      const itemName = meta.name || extractCleanItemName(evt.description) || 'reward';
      actionName = `<span class="audit-target-name">"${escapeHTML(itemName)}"</span>`;
    } else {
      actionIcon = '📝';
      actionName = escapeHTML(evt.title || 'action');
    }

    // 3. outcome_icon: outcome_details if needed (placed on line 2 right side)
    let outcomeHtml = '';
    if (outcomeIcon) {
      const outcomeTooltip = (outcomeClass === 'outcome-mandatory') ? 'title="Mandatory Chore"' : '';
      outcomeHtml = `
        <span class="audit-seg-outcome ${outcomeClass}" ${outcomeTooltip}>
          <span>${outcomeIcon}</span>${outcomeDetails ? `<span>${outcomeDetails}</span>` : ''}
        </span>
      `;
    }

    // 4. Timestamp (placed on line 1 right side)
    const timeStr = (timeFormat === 'time')
      ? formatActivityTime(evt.created_at)
      : formatActivityRelativeTime(evt.created_at);

    return `
      <div class="history-audit-row">
        <div class="audit-row-line1">
          <span class="audit-seg-user">
            ${avatarHtml}
            <span class="audit-user-name">${escapeHTML(username)}</span>
          </span>
          <span class="audit-row-time" title="${escapeHTML(evt.created_at || '')}">${timeStr}</span>
        </div>
        <div class="audit-row-line2">
          <span class="audit-seg-action">
            <span class="audit-action-icon">${actionIcon}</span>
            <span class="audit-action-text">${actionName}</span>
          </span>
          ${outcomeHtml}
        </div>
      </div>
    `;
  } catch (err) {
    console.error("Error rendering audit flow row:", err, evt);
    return `
      <div class="history-audit-row">
        <div class="audit-row-line1">
          <span class="audit-seg-user">
            <span class="audit-user-avatar">👤</span>
            <span class="audit-user-name">${escapeHTML(evt.user_username || 'User')}</span>
          </span>
          <span class="audit-row-time">${formatActivityRelativeTime(evt.created_at)}</span>
        </div>
        <div class="audit-row-line2">
          <span class="audit-seg-action">
            <span class="audit-action-icon">📝</span>
            <span class="audit-action-text">${escapeHTML(evt.title || evt.description || 'Activity')}</span>
          </span>
        </div>
      </div>
    `;
  }
}

// Load the 3 most recent activities for the header dropdown
async function loadRecentHistory() {
  if (!currentSession) return;
  const listEl = document.getElementById("history-dropdown-list");
  if (!listEl) return;

  try {
    const events = await apiCall(`/choregroups/${currentSession.choregroup_id}/history/recent`);
    if (!events || events.length === 0) {
      listEl.innerHTML = `
        <div class="dropdown-item">
          <div class="dropdown-item-icon">✨</div>
          <div class="dropdown-item-content">
            <strong>No Recent Activity</strong>
            <span>Completed chores and actions will appear here.</span>
          </div>
        </div>
      `;
      return;
    }

    listEl.innerHTML = events.map(evt => {
      try {
        return renderAuditFlowRow(evt, 'relative');
      } catch (e) {
        console.warn("Failed rendering history row:", e, evt);
        return "";
      }
    }).join("");
  } catch (err) {
    console.error("Failed to load recent history:", err);
    listEl.innerHTML = `
      <div class="dropdown-item">
        <div class="dropdown-item-icon">⚠️</div>
        <div class="dropdown-item-content">
          <span>Failed to load activity history.</span>
        </div>
      </div>
    `;
  }
}

let historyViewMode = "all"; // "all" or "week"

function setHistoryViewMode(mode) {
  historyViewMode = mode;
  if (mode === "week") {
    historyWeekOffset = 0;
  }
  fetchAndRenderFullHistory();
}

// Open Full History Modal
async function openFullHistoryModal(event) {
  if (event) event.stopPropagation();

  // Close dropdown if open and blur trigger
  const menu = document.getElementById('history-dropdown-menu');
  if (menu) menu.classList.remove('active');
  const trigger = document.getElementById('history-trigger-btn');
  if (trigger) {
    trigger.classList.remove('is-active');
    trigger.setAttribute('aria-expanded', 'false');
    trigger.blur();
  }

  const modal = document.getElementById("full-history-modal");
  if (!modal) return;

  historyViewMode = "all"; // Default to all history so "Show more..." displays the full history
  historyWeekOffset = 0;
  historyCurrentFilter.userId = "";
  historyCurrentFilter.category = "all";

  // Setup filters based on role
  const userFilterContainer = document.getElementById("history-user-filter-container");
  const userSelect = document.getElementById("history-user-filter");
  const categorySelect = document.getElementById("history-category-filter");

  if (categorySelect) categorySelect.value = "all";

  if (currentSession && currentSession.role === "admin") {
    if (userFilterContainer) userFilterContainer.style.display = "flex";
    // Populate user filter options
    if (userSelect) {
      userSelect.innerHTML = `<option value="">All Members</option>`;
      try {
        const members = await apiCall(`/choregroups/${currentSession.choregroup_id}/members`);
        if (members && members.length > 0) {
          members.forEach(m => {
            const opt = document.createElement("option");
            opt.value = m.id;
            opt.textContent = `${m.username} (${m.role === 'admin' ? 'Admin' : 'User'})`;
            userSelect.appendChild(opt);
          });
        }
      } catch (e) {
        console.warn("Could not populate members filter:", e);
      }
      userSelect.value = "";
    }
  } else {
    // Regular users only see their own activities, lock filter to their user ID
    if (currentSession) {
      historyCurrentFilter.userId = currentSession.user_id || currentSession.id;
    }
    if (userFilterContainer) userFilterContainer.style.display = "none";
  }

  // Open modal using system openModal (adds .active class and enables pointer events)
  openModal("full-history-modal");
  await fetchAndRenderFullHistory();
}

function closeFullHistoryModal() {
  closeModal("full-history-modal");
  const trigger = document.getElementById('history-trigger-btn');
  if (trigger) {
    trigger.classList.remove('is-active');
    trigger.setAttribute('aria-expanded', 'false');
    trigger.blur();
  }
}

function navigateHistoryWeek(delta) {
  historyWeekOffset += delta;
  if (historyWeekOffset > 0) historyWeekOffset = 0; // Don't navigate into future weeks
  fetchAndRenderFullHistory();
}

function jumpToCurrentWeek() {
  historyWeekOffset = 0;
  fetchAndRenderFullHistory();
}

function onHistoryFilterChange() {
  const userSelect = document.getElementById("history-user-filter");
  const categorySelect = document.getElementById("history-category-filter");

  historyCurrentFilter.userId = userSelect ? userSelect.value : "";
  historyCurrentFilter.category = categorySelect ? categorySelect.value : "all";

  fetchAndRenderFullHistory();
}

// Fetch and render the timeline for full history modal
async function fetchAndRenderFullHistory() {
  if (!currentSession) return;
  const container = document.getElementById("history-timeline-container");
  const rangeText = document.getElementById("history-range-text");
  const prevBtn = document.getElementById("history-prev-week-btn");
  const nextBtn = document.getElementById("history-next-week-btn");
  const thisWeekBtn = document.getElementById("history-this-week-btn");
  const modeAllBtn = document.getElementById("history-mode-all-btn");
  const modeWeekBtn = document.getElementById("history-mode-week-btn");

  if (!container) return;

  // Toggle Mode Buttons & Controls
  if (modeAllBtn && modeWeekBtn) {
    if (historyViewMode === "all") {
      modeAllBtn.classList.add("active");
      modeWeekBtn.classList.remove("active");
      if (prevBtn) prevBtn.style.display = "none";
      if (nextBtn) nextBtn.style.display = "none";
      if (thisWeekBtn) thisWeekBtn.style.display = "none";
      if (rangeText) rangeText.textContent = "All Recorded Activities";
    } else {
      modeAllBtn.classList.remove("active");
      modeWeekBtn.classList.add("active");
      if (prevBtn) prevBtn.style.display = "inline-flex";
      if (nextBtn) nextBtn.style.display = "inline-flex";

      const { start, end } = getHistoryWeekRange(historyWeekOffset);
      const startStr = start.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
      const endStr = end.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
      const isThisWeek = historyWeekOffset === 0;

      if (rangeText) {
        rangeText.textContent = `${startStr} – ${endStr}${isThisWeek ? ' (Current Week)' : ''}`;
      }

      if (nextBtn) {
        nextBtn.disabled = isThisWeek;
        nextBtn.style.opacity = isThisWeek ? "0.4" : "1";
        nextBtn.style.cursor = isThisWeek ? "not-allowed" : "pointer";
      }
      if (thisWeekBtn) {
        thisWeekBtn.style.display = isThisWeek ? "none" : "inline-block";
      }
    }
  }

  container.innerHTML = `
    <div style="text-align: center; padding: 2.5rem 1rem; color: var(--text-muted);">
      <div style="font-size: 2rem; margin-bottom: 0.5rem; animation: spin 1s linear infinite;">⏳</div>
      <div>Loading activities...</div>
    </div>
  `;

  try {
    const params = new URLSearchParams();

    // In week mode, pass start_date and end_date; in all mode, omit dates to fetch all history
    if (historyViewMode === "week") {
      const { start, end } = getHistoryWeekRange(historyWeekOffset);
      params.append("start_date", start.toISOString());
      params.append("end_date", end.toISOString());
    }

    if (currentSession && currentSession.role !== "admin") {
      historyCurrentFilter.userId = currentSession.user_id || currentSession.id;
    }

    if (historyCurrentFilter.userId) {
      params.append("user_id", historyCurrentFilter.userId);
    }
    if (historyCurrentFilter.category && historyCurrentFilter.category !== "all") {
      params.append("activity_category", historyCurrentFilter.category);
    }
    params.append("limit", "100");

    const res = await apiCall(`/choregroups/${currentSession.choregroup_id}/history?${params.toString()}`);
    const events = res && res.events ? res.events : [];
    const totalCount = (res && typeof res.total_count === "number") ? res.total_count : events.length;

    if (rangeText && historyViewMode === "all") {
      const label = (currentSession && currentSession.role === "admin") ? "All Activities" : "My Activities";
      rangeText.textContent = `${label} (${totalCount})`;
    }

    if (events.length === 0) {
      container.innerHTML = `
        <div class="history-empty-state">
          <div class="history-empty-icon">🍃</div>
          <h4 style="margin: 0 0 0.25rem 0; color: var(--text-main);">No Activities Found</h4>
          <p style="margin: 0; font-size: 0.85rem;">No actions matching this filter were recorded.</p>
        </div>
      `;
      return;
    }

    // Group events by day (YYYY-MM-DD)
    const dayGroups = {};
    events.forEach(evt => {
      const dayKey = evt.created_at ? evt.created_at.split("T")[0] : "Other";
      if (!dayGroups[dayKey]) dayGroups[dayKey] = [];
      dayGroups[dayKey].push(evt);
    });

    let html = "";
    Object.keys(dayGroups).forEach(dayKey => {
      const dayEvents = dayGroups[dayKey];
      const headerTitle = formatActivityDayHeader(dayKey);

      html += `
        <div class="history-day-group">
          <div class="history-day-header">
            <span>📅</span>
            <span>${headerTitle}</span>
            <span style="font-weight: 500; opacity: 0.7; margin-left: auto;">(${dayEvents.length})</span>
          </div>
          <div class="history-day-list">
      `;

      dayEvents.forEach(evt => {
        html += renderAuditFlowRow(evt, 'time');
      });

      html += `
          </div>
        </div>
      `;
    });

    container.innerHTML = html;
  } catch (err) {
    console.error("Failed to fetch full history:", err);
    container.innerHTML = `
      <div class="history-empty-state">
        <div class="history-empty-icon">⚠️</div>
        <h4 style="margin: 0 0 0.25rem 0; color: var(--text-main);">Error Loading History</h4>
        <p style="margin: 0; font-size: 0.85rem;">${escapeHTML(err.message || "Failed to communicate with server.")}</p>
      </div>
    `;
  }
}

window.loadRecentHistory = loadRecentHistory;
window.openFullHistoryModal = openFullHistoryModal;
window.openFullHistoryModalImpl = openFullHistoryModal;
window.closeFullHistoryModal = closeFullHistoryModal;
window.navigateHistoryWeek = navigateHistoryWeek;
window.jumpToCurrentWeek = jumpToCurrentWeek;
window.onHistoryFilterChange = onHistoryFilterChange;
window.setHistoryViewMode = setHistoryViewMode;
window.fetchAndRenderFullHistory = fetchAndRenderFullHistory;






