const API = "/api";
const ROUND_MS = 5000;

let timerId = null;
let endAt = 0;
let roundActive = false;
let leaderboardTab = "all";

async function fetchJSON(path, options = {}) {
  const res = await fetch(`${API}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  return res.json();
}

function escapeHtml(str) {
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function $(id) {
  return document.getElementById(id);
}

function setStatus(text, type = "info") {
  const el = $("status-message");
  el.textContent = text;
  el.dataset.type = type;
  el.hidden = !text;
}

function renderLeaderboard(scores) {
  const el = $("leaderboard");
  if (!scores.length) {
    el.innerHTML = '<p class="muted">No scores yet — be the first!</p>';
    return;
  }
  el.innerHTML = `
    <div class="score-row header">
      <span>#</span><span>Player</span><span>Word</span><span>Len</span><span>Score</span>
    </div>
    ${scores.map((s, i) => `
      <div class="score-row">
        <span>${i + 1}</span>
        <span>${escapeHtml(s.player_name)}</span>
        <span class="word-cell">${escapeHtml(s.word)}</span>
        <span>${s.length}</span>
        <span class="score-cell">${s.score} <small class="tier">${escapeHtml(formatRarity(s.rarity_tier))}</small></span>
      </div>
    `).join("")}
  `;
}

async function loadLeaderboard() {
  try {
    const path = leaderboardTab === "today" ? "/scores/today" : "/scores";
    const scores = await fetchJSON(path);
    renderLeaderboard(scores);
  } catch (err) {
    $("leaderboard").innerHTML =
      `<p class="muted">Failed to load scores: ${escapeHtml(err.message)}</p>`;
  }
}

async function loadStats() {
  try {
    const stats = await fetchJSON("/stats");
    $("stat-games").textContent = stats.total_games ?? 0;
    $("stat-avg").textContent = Math.round(stats.average_score ?? 0);
    $("stat-top").textContent = stats.top_score ?? 0;
    $("stat-words").textContent = stats.unique_words ?? 0;
    await loadPersonalBest();
  } catch {
    /* stats optional on first load */
  }
}

async function loadPersonalBest() {
  const name = $("player-name").value.trim() || "Player";
  try {
    const data = await fetchJSON(`/players/${encodeURIComponent(name)}/scores`);
    $("stat-personal").textContent = data.best_score ?? 0;
  } catch {
    $("stat-personal").textContent = "—";
  }
}

async function showHint() {
  const el = $("hint-line");
  el.hidden = false;
  el.textContent = "Finding a rare word…";
  try {
    const hint = await fetchJSON("/hint");
    el.textContent = `Try a ${formatRarity(hint.rarity_tier).toLowerCase()} word (${hint.length} letters): ${hint.word}`;
  } catch (err) {
    el.textContent = "Could not load hint.";
  }
}

async function checkHealth() {
  const badge = $("cluster-info");
  try {
    const res = await fetch("/health");
    badge.textContent = res.ok ? "API healthy" : "API degraded";
    badge.style.color = res.ok ? "var(--success)" : "var(--warning)";
  } catch {
    badge.textContent = "API unreachable";
    badge.style.color = "var(--danger)";
  }
}

function resetRoundUI() {
  roundActive = false;
  clearInterval(timerId);
  timerId = null;
  $("timer").textContent = "5.0";
  $("timer").className = "timer idle";
  $("word-input").value = "";
  $("word-input").disabled = true;
  $("char-count").textContent = "0";
  $("start-btn").disabled = false;
  $("submit-btn").disabled = true;
  $("result").hidden = true;
  setStatus("");
}

function startRound() {
  resetRoundUI();
  roundActive = true;
  $("start-btn").disabled = true;
  $("word-input").disabled = false;
  $("submit-btn").disabled = false;
  $("word-input").focus();
  $("timer").className = "timer running";

  endAt = Date.now() + ROUND_MS;
  tickTimer();
  timerId = setInterval(tickTimer, 50);
}

function tickTimer() {
  const left = Math.max(0, endAt - Date.now());
  $("timer").textContent = (left / 1000).toFixed(1);
  if (left <= 0) {
    finishRound(true);
  }
}

function sanitizeWord(raw) {
  return raw.trim().replace(/\s+/g, "");
}

// Hammer-throw style: fast ramp, slow settle at the final score.
function easeOutQuart(t) {
  return 1 - Math.pow(1 - t, 4);
}

function animateCounter(el, target, duration = 2400, prefix = "") {
  return new Promise((resolve) => {
    const start = performance.now();
    const tick = (now) => {
      const t = Math.min(1, (now - start) / duration);
      const value = Math.round(target * easeOutQuart(t));
      el.textContent = `${prefix}${value}`;
      if (t < 1) {
        requestAnimationFrame(tick);
      } else {
        el.textContent = `${prefix}${target}`;
        resolve();
      }
    };
    requestAnimationFrame(tick);
  });
}

function formatRarity(tier) {
  const labels = {
    common: "Common",
    everyday: "Everyday",
    uncommon: "Uncommon",
    rare: "Rare",
    obscure: "Obscure",
  };
  return labels[tier] || tier || "—";
}

async function revealScore(result) {
  $("result-word").textContent = result.word;
  $("rarity-tier").textContent = formatRarity(result.rarity_tier);
  $("final-score").textContent = "0";
  $("length-points").textContent = "+0";
  $("unique-points").textContent = "+0";
  $("result").hidden = false;
  $("result").classList.add("counting");

  await Promise.all([
    animateCounter($("final-score"), result.score, 2600),
    animateCounter($("length-points"), result.length_points, 2000, "+"),
    animateCounter($("unique-points"), result.uniqueness_points, 2000, "+"),
  ]);

  $("result").classList.remove("counting");
}

async function finishRound(timedOut) {
  if (!roundActive) return;
  roundActive = false;
  clearInterval(timerId);
  timerId = null;

  $("word-input").disabled = true;
  $("submit-btn").disabled = true;
  $("start-btn").disabled = true;
  $("timer").className = "timer done";
  $("timer").textContent = "0.0";

  const word = sanitizeWord($("word-input").value);
  if (!word) {
    setStatus(timedOut ? "Time's up — no word entered!" : "Enter a word first.", "error");
    $("start-btn").disabled = false;
    return;
  }

  setStatus("Scoring your word…", "info");

  try {
    const player = $("player-name").value.trim() || "Player";
    const result = await fetchJSON("/scores", {
      method: "POST",
      body: JSON.stringify({ word, player_name: player }),
    });

    setStatus("", "info");
    await revealScore(result);
    await Promise.all([loadLeaderboard(), loadStats()]);
  } catch (err) {
    let msg = err.message;
    try {
      const parsed = JSON.parse(msg);
      if (parsed.error) msg = parsed.error;
    } catch { /* keep raw message */ }
    setStatus(msg, "error");
    $("start-btn").disabled = false;
  }
}

$("word-input").addEventListener("input", (e) => {
  const cleaned = e.target.value.replace(/[^a-zA-Z]/g, "");
  if (cleaned !== e.target.value) {
    e.target.value = cleaned;
  }
  $("char-count").textContent = cleaned.length;
});

$("word-input").addEventListener("keydown", (e) => {
  if (e.key === "Enter" && roundActive) {
    e.preventDefault();
    finishRound(false);
  }
});

$("start-btn").addEventListener("click", startRound);
$("hint-btn").addEventListener("click", showHint);
$("submit-btn").addEventListener("click", () => finishRound(false));
$("again-btn").addEventListener("click", resetRoundUI);
$("refresh").addEventListener("click", () => Promise.all([loadLeaderboard(), loadStats()]));
$("player-name").addEventListener("change", loadPersonalBest);

document.querySelectorAll(".tab").forEach((btn) => {
  btn.addEventListener("click", () => {
    document.querySelectorAll(".tab").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    leaderboardTab = btn.dataset.tab;
    loadLeaderboard();
  });
});

loadLeaderboard();
loadStats();
checkHealth();
setInterval(() => { loadLeaderboard(); loadStats(); }, 15000);
