"use strict";

// ============================================================
// Labels
// ============================================================

const LEVELS = ["easy", "medium", "hard"];
const LEVEL_LABELS = { easy: "Fácil", medium: "Médio", hard: "Difícil" };
const LEVEL_POINTS = { easy: 1, medium: 2, hard: 3 };

const STAGES = ["limits", "parse", "ast", "gofmt", "complexity", "vet", "gosec", "build"];
const STAGE_LABELS = {
  limits: "Limites",
  parse: "Sintaxe",
  ast: "Regras",
  gofmt: "Formatação",
  complexity: "Complexidade",
  vet: "go vet",
  gosec: "gosec",
  build: "Compilação",
  sandbox: "Sandbox",
  run: "Execução",
};

const VERDICTS = {
  passed: { label: "Aprovado", tone: "ok", icon: "check" },
  failed: { label: "Reprovado", tone: "warn", icon: "x" },
  timeout: { label: "Tempo esgotado", tone: "warn", icon: "clock" },
  rejected: { label: "Rejeitado", tone: "err", icon: "alert" },
  compile_error: { label: "Erro de compilação", tone: "err", icon: "alert" },
  internal_error: { label: "Erro interno", tone: "err", icon: "alert" },
};

// ============================================================
// Icons (static SVG paths, never user content)
// ============================================================

const ICONS = {
  check: '<path d="M20 6 9 17l-5-5"/>',
  x: '<path d="M18 6 6 18M6 6l12 12"/>',
  alert: '<path d="M12 9v4m0 4h.01"/><path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
  copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/>',
  chevron: '<path d="m9 6 6 6-6 6"/>',
  minus: '<path d="M5 12h14"/>',
  play: '<path d="M7 4v16l13-8z"/>',
  reset: '<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 16v-4m0-4h.01"/>',
  bulb: '<path d="M9 18h6M10 22h4M12 2a7 7 0 0 0-4 12.7V17h8v-2.3A7 7 0 0 0 12 2z"/>',
  file: '<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><path d="M14 3v6h6"/>',
  exit: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9"/>',
  list: '<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>',
  user: '<circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/>',
  lock: '<rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/>',
  refresh: '<path d="M21 12a9 9 0 1 1-3-6.7L21 8"/><path d="M21 3v5h-5"/>',
};

function icon(name, size = 16) {
  const t = document.createElement("template");
  t.innerHTML = `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name]}</svg>`;
  return t.content.firstChild;
}

// ============================================================
// Helpers
// ============================================================

// el creates an element. Text is always set with text nodes, so content
// coming from submissions is never interpreted as HTML.
function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(attrs)) {
    if (value === undefined || value === null || value === false) continue;
    if (key === "class") node.className = value;
    else if (key === "style") node.style.cssText = value;
    else if (key.startsWith("on")) node.addEventListener(key.slice(2), value);
    else node.setAttribute(key, value === true ? "" : value);
  }
  for (const child of children.flat()) {
    if (child === null || child === undefined || child === false) continue;
    node.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
  return node;
}

const $ = (id) => document.getElementById(id);
const pct = (x) => `${Math.round(x * 100)}%`;
const fmtPoints = (x) => (Number.isInteger(x) ? String(x) : x.toFixed(1));
const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

function toast(message, kind = "info") {
  const node = el("div", { class: `toast ${kind}`, role: "status" },
    icon(kind === "error" ? "alert" : kind === "success" ? "check" : "info"), message);
  $("toasts").append(node);
  setTimeout(() => node.remove(), 5000);
}

async function api(method, path, body, headers = {}) {
  const options = { method, headers: { ...headers } };
  if (body !== undefined) {
    options.headers["Content-Type"] = "application/json";
    options.body = JSON.stringify(body);
  }
  let resp;
  try {
    resp = await fetch(path, options);
  } catch {
    throw new Error("Não foi possível contactar o servidor.");
  }
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) throw new Error(data.error || `Erro ${resp.status}`);
  return data;
}

// Per-browser storage for drafts and progress. It may be unavailable
// (private mode, blocked storage), so every access is guarded.
const storage = {
  get(key, fallback) {
    try {
      const raw = localStorage.getItem(key);
      return raw === null ? fallback : JSON.parse(raw);
    } catch {
      return fallback;
    }
  },
  set(key, value) {
    try {
      localStorage.setItem(key, JSON.stringify(value));
    } catch {
      // Storage is a convenience; ignore failures.
    }
  },
  remove(key) {
    try {
      localStorage.removeItem(key);
    } catch {
      // Ignore.
    }
  },
};

// ============================================================
// State and routing
// ============================================================

// Routes: #/ (practice), #/exercicio/<id>, #/prova, #/prova/<attempt>/<id?>
const state = {
  exercises: [],
  exams: [],
  details: new Map(), // exercise id -> detail
  reports: new Map(), // scope/exercise id -> last report
  filter: "",
  query: "",
  attempt: null,
  // Self-declared student name ("fake" login: no password).
  student: storage.get("gavel:student", ""),
  // Admin password, kept only for this browser tab.
  adminToken: sessionGet("gavel:admin"),
  admin: { tab: "sessions", query: "", verdict: "", attemptsQuery: "", sessions: [], submissions: [], attempts: [], timer: null },
  attemptDeadline: 0,
  sessionsTimer: null,
};

// Browser storage is per browser, so progress and drafts are keyed by student.
const studentKey = (key) => `gavel:${state.student}:${key}`;

function sessionGet(key) {
  try {
    return sessionStorage.getItem(key) || "";
  } catch {
    return "";
  }
}

function sessionSet(key, value) {
  try {
    if (value) sessionStorage.setItem(key, value);
    else sessionStorage.removeItem(key);
  } catch {
    // Without session storage the admin logs in again on reload.
  }
}

function parseRoute() {
  const parts = location.hash.replace(/^#\/?/, "").split("/").filter(Boolean).map(decodeURIComponent);
  if (parts[0] === "prova") return { mode: "exam", attemptId: parts[1] || null, exerciseId: parts[2] || null };
  if (parts[0] === "exercicio") return { mode: "practice", exerciseId: parts[1] || null };
  if (parts[0] === "docente") return { mode: "admin" };
  return { mode: "practice", exerciseId: null };
}

function go(...parts) {
  location.hash = "#/" + parts.filter(Boolean).map(encodeURIComponent).join("/");
}

async function route() {
  const r = parseRoute();
  for (const tab of document.querySelectorAll(".segmented button")) {
    tab.setAttribute("aria-selected", String(tab.dataset.mode === r.mode));
  }
  renderUser();
  clearInterval(state.admin.timer);
  if (r.mode === "admin") {
    showSidebar(false);
    renderAdmin();
    return;
  }
  if (!state.student) {
    showSidebar(false);
    renderStudentLogin();
    return;
  }
  if (r.mode === "practice") {
    showSidebar(true);
    renderPracticeSidebar(r.exerciseId);
    if (r.exerciseId) renderWorkspace({ exerciseId: r.exerciseId, scope: "practice" });
    else renderWelcome();
    return;
  }

  if (!r.attemptId) {
    showSidebar(false);
    renderOpenSessions();
    return;
  }
  if (!state.attempt || state.attempt.attempt_id !== r.attemptId) {
    try {
      setAttempt(await api("GET", `/api/attempts/${encodeURIComponent(r.attemptId)}`));
    } catch (err) {
      toast(err.message, "error");
      go("prova");
      return;
    }
  }
  const exerciseId = r.exerciseId || state.attempt.exercise_ids[0];
  showSidebar(true);
  renderAttemptSidebar(exerciseId);
  renderWorkspace({ exerciseId, scope: state.attempt.attempt_id });
}

function showSidebar(visible) {
  $("shell").classList.toggle("no-sidebar", !visible);
}

// ============================================================
// Practice sidebar
// ============================================================

function bestScores() {
  return storage.get(studentKey("best"), {});
}

function progressMark(score, attempted) {
  if (attempted && score >= 1) return el("span", { class: "progress-mark solved", title: "Resolvido" }, icon("check", 11));
  if (attempted) return el("span", { class: "progress-mark partial", title: `Melhor resultado: ${pct(score)}` });
  return el("span", { class: "progress-mark none", title: "Por resolver" });
}

function renderPracticeSidebar(selectedId) {
  const sidebar = $("sidebar");
  const counts = { "": state.exercises.length };
  for (const level of LEVELS) counts[level] = state.exercises.filter((e) => e.difficulty === level).length;

  const search = el("input", {
    type: "search",
    placeholder: "Procurar exercício…",
    value: state.query,
    "aria-label": "Procurar exercício",
  });
  search.addEventListener("input", () => {
    state.query = search.value;
    renderList();
  });

  const chips = el("div", { class: "chips", role: "group", "aria-label": "Filtrar por nível" },
    ["", ...LEVELS].map((level) => el("button", {
      type: "button",
      class: "chip",
      "aria-pressed": String(state.filter === level),
      onclick: () => {
        state.filter = level;
        renderPracticeSidebar(selectedId);
      },
    }, level ? LEVEL_LABELS[level] : "Todos", el("span", { class: "count" }, counts[level]))));

  const body = el("div", { class: "sidebar-body" });
  function renderList() {
    const best = bestScores();
    const query = state.query.trim().toLowerCase();
    const visible = state.exercises.filter((e) =>
      (!state.filter || e.difficulty === state.filter) &&
      (!query || e.title.toLowerCase().includes(query) || e.id.includes(query)));
    const groups = LEVELS.map((level) => {
      const items = visible.filter((e) => e.difficulty === level);
      if (!items.length) return null;
      return [
        el("div", { class: "group-label" }, `${LEVEL_LABELS[level]} · ${plural(LEVEL_POINTS[level], "ponto", "pontos")}`),
        items.map((e) => el("a", {
          class: "list-item",
          href: `#/exercicio/${encodeURIComponent(e.id)}`,
          "aria-current": String(e.id === selectedId),
          title: e.description,
        }, el("span", { class: `dot ${e.difficulty}` }), el("span", { class: "item-title" }, e.title),
        progressMark(best[e.id] || 0, e.id in best))),
      ];
    });
    body.replaceChildren(...(visible.length ? groups.flat(2).filter(Boolean)
      : [el("p", { class: "empty-list" }, "Nenhum exercício encontrado.")]));
  }
  renderList();

  sidebar.replaceChildren(
    el("div", { class: "sidebar-head" }, el("label", { class: "search" }, icon("search"), search), chips),
    body,
  );
}

function renderWelcome() {
  const best = bestScores();
  const solved = state.exercises.filter((e) => best[e.id] >= 1).length;
  const statCard = (value, label, level) => el("button", {
    type: "button",
    class: "card stat",
    onclick: () => {
      state.filter = level;
      renderPracticeSidebar(null);
    },
  }, el("span", { class: "stat-value" }, value),
  el("span", { class: "stat-label" }, level ? el("span", { class: `dot ${level}` }) : null, label));

  $("main").replaceChildren(el("div", { class: "page" },
    el("div", { class: "page-head" },
      el("h1", {}, "Modo de prática"),
      el("p", {}, "Escolha um exercício na lista, escreva a função pedida e submeta. O Gavel compila o código, corre os testes e mostra o que passou e o que falhou.")),
    el("div", { class: "stats" },
      statCard(state.exercises.length, `Total · ${solved} resolvidos`, ""),
      LEVELS.map((level) => statCard(
        state.exercises.filter((e) => e.difficulty === level).length,
        `${LEVEL_LABELS[level]} · ${plural(LEVEL_POINTS[level], "ponto", "pontos")}`,
        level))),
    el("h2", { class: "panel-title" }, "Como funciona"),
    el("div", { class: "steps" },
      [
        ["Escreva a função", "Defina a função pedida em package solution, com a assinatura exata. Pode usar fmt, strings, math, sort e outros pacotes básicos."],
        ["Análise estática", "Sem executar o código: sintaxe, regras (imports permitidos, assinatura), formatação, complexidade, go vet, gosec e compilação."],
        ["Análise dinâmica", "O programa compilado corre com cada teste, isolado e com limite de tempo. São detetados resultados errados, panics e ciclos infinitos."],
        ["Veja o relatório", "Cada teste mostra o valor esperado e o obtido. A nota é a percentagem de testes que passaram."],
      ].map(([title, text], i) => el("div", { class: "card step" },
        el("span", { class: "step-num" }, i + 1), el("h3", {}, title), el("p", {}, text)))),
  ));
}

// ============================================================
// Exam: start page and attempt sidebar
// ============================================================

// ============================================================
// Countdowns (exam time limits)
// ============================================================

// deadlineOf turns the server's remaining_seconds into a local timestamp,
// so countdowns do not depend on both clocks agreeing.
const deadlineOf = (view) => Date.now() + view.remaining_seconds * 1000;

function fmtRemaining(ms) {
  const total = Math.max(0, Math.ceil(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const sec = total % 60;
  return h ? `${h} h ${String(m).padStart(2, "0")} min` : `${m}:${String(sec).padStart(2, "0")}`;
}

const fmtClock = (iso) => new Date(iso).toLocaleTimeString("pt-PT", { hour: "2-digit", minute: "2-digit" });

// countdown is an element that the ticker below keeps up to date.
function countdown(deadline, done = "terminada") {
  return el("span", { class: "countdown", "data-deadline": String(deadline), "data-done": done }, fmtRemaining(deadline - Date.now()));
}

function tickCountdowns() {
  for (const node of document.querySelectorAll("[data-deadline]")) {
    const left = Number(node.dataset.deadline) - Date.now();
    node.textContent = left > 0 ? fmtRemaining(left) : node.dataset.done;
  }
  // When the exam time runs out, lock the editor without waiting for a reload.
  if (state.attempt?.open && Date.now() >= state.attemptDeadline) {
    state.attempt.open = false;
    toast("O tempo da prova terminou. Já não é possível submeter.", "error");
    if (parseRoute().mode === "exam") route();
  }
}

function setAttempt(attempt) {
  state.attempt = attempt;
  state.attemptDeadline = deadlineOf(attempt);
  storage.set(studentKey("last-attempt"), attempt.attempt_id);
}

// ============================================================
// Exam: open sessions and the student's attempt
// ============================================================

function compositionPills(composition) {
  return el("div", { class: "meta" }, LEVELS.filter((level) => composition[level]).map((level) =>
    el("span", { class: `pill ${level}` }, `${composition[level]} × ${LEVEL_LABELS[level].toLowerCase()}`)));
}

function compositionFacts(composition) {
  const total = LEVELS.reduce((sum, level) => sum + (composition[level] || 0), 0);
  const points = LEVELS.reduce((sum, level) => sum + (composition[level] || 0) * LEVEL_POINTS[level], 0);
  return { total, points };
}

async function renderOpenSessions() {
  let sessions;
  try {
    sessions = (await api("GET", "/api/sessions")) || [];
  } catch (err) {
    $("main").replaceChildren(el("div", { class: "page" }, el("div", { class: "results-empty" },
      icon("alert", 28), el("strong", {}, "Não foi possível carregar as provas"), err.message)));
    return;
  }
  const r = parseRoute();
  if (r.mode !== "exam" || r.attemptId) return; // the student moved on meanwhile

  const lastAttempt = storage.get(studentKey("last-attempt"), null);
  const cards = sessions.map((s) => {
    const { total, points } = compositionFacts(s.composition);
    return el("div", { class: "card exam-card" },
      el("div", { class: "exam-card-top" },
        el("h2", {}, s.title),
        el("span", { class: "pill tone-ok" }, "Aberta")),
      el("div", { class: "exam-bar", "aria-hidden": "true" },
        LEVELS.flatMap((level) => Array.from({ length: s.composition[level] || 0 }, () => el("span", { class: level })))),
      compositionPills(s.composition),
      el("div", { class: "exam-facts" },
        el("span", {}, el("strong", {}, total), " exercícios"),
        el("span", {}, el("strong", {}, points), " pontos")),
      el("p", { class: "exam-time" }, icon("clock", 14), `Termina às ${fmtClock(s.ends_at)} · faltam `, countdown(deadlineOf(s))),
      el("button", { type: "button", class: "btn btn-primary", onclick: (event) => joinSession(s.session_id, event.currentTarget) },
        icon("play", 14), "Entrar na prova"));
  });

  $("main").replaceChildren(el("div", { class: "page" },
    el("div", { class: "page-head" },
      el("h1", {}, "Provas"),
      el("p", {}, "O docente abre as provas e define o tempo. Todos recebem os mesmos exercícios. Pode submeter cada um as vezes que quiser: conta a melhor submissão, pesada pelos pontos do nível.")),
    cards.length ? el("div", { class: "exam-grid" }, cards)
      : el("div", { class: "results-empty" }, icon("clock", 28), el("strong", {}, "Não há provas abertas"),
        "Quando o docente abrir uma prova, ela aparece aqui automaticamente."),
    lastAttempt ? el("div", { class: "card resume" },
      el("div", { class: "resume-text" },
        el("h3", {}, "A sua última prova"),
        el("p", {}, "Veja os exercícios e a nota, ou continue se ainda estiver aberta.")),
      el("button", { type: "button", class: "btn btn-ghost", onclick: () => go("prova", lastAttempt) }, "Abrir")) : null,
  ));
  // Refresh the list while the student waits on it.
  clearTimeout(state.sessionsTimer);
  state.sessionsTimer = setTimeout(() => {
    const now = parseRoute();
    if (now.mode === "exam" && !now.attemptId && state.student) renderOpenSessions();
  }, 10000);
}

async function joinSession(sessionId, button) {
  button.disabled = true;
  try {
    setAttempt(await api("POST", `/api/sessions/${encodeURIComponent(sessionId)}/join`, { student: state.student }));
    toast("Entrou na prova. Boa sorte!", "success");
    go("prova", state.attempt.attempt_id);
  } catch (err) {
    toast(err.message, "error");
    button.disabled = false;
  }
}

function renderAttemptSidebar(selectedId) {
  const a = state.attempt;
  const score = a.score;
  const copy = () => {
    navigator.clipboard?.writeText(a.attempt_id)
      .then(() => toast("Código copiado.", "success"))
      .catch(() => toast(a.attempt_id));
  };
  const leave = () => {
    state.attempt = null;
    go("prova");
  };

  $("sidebar").replaceChildren(
    el("div", { class: "sidebar-head attempt-card" },
      el("div", { class: "attempt-top" },
        el("div", { class: "ring", style: `--value: ${Math.round(score.total * 100)}`, role: "img", "aria-label": `Pontuação ${pct(score.total)}` },
          el("span", {}, pct(score.total))),
        el("div", {},
          el("h2", {}, a.title),
          el("p", {}, `${fmtPoints(score.points)} de ${score.max_points} pontos`))),
      a.open
        ? el("div", { class: "time-box" }, icon("clock", 15), el("span", {}, "Tempo restante"), countdown(state.attemptDeadline, "0:00"))
        : el("div", { class: "time-box ended" }, icon("alert", 15), el("span", {}, `Prova terminada às ${fmtClock(a.ends_at)}`)),
      el("div", { class: "attempt-id", title: "Código da tentativa" },
        el("span", {}, a.attempt_id),
        el("button", { type: "button", class: "btn btn-ghost btn-sm btn-icon", title: "Copiar código", "aria-label": "Copiar código", onclick: copy }, icon("copy", 14))),
      el("div", { class: "attempt-actions" },
        el("button", { type: "button", class: "btn btn-ghost btn-sm", onclick: leave }, icon("exit", 14), "Sair da prova"))),
    el("div", { class: "sidebar-body" },
      el("div", { class: "group-label" }, "Exercícios da prova"),
      score.exercises.map((e) => el("a", {
        class: "list-item",
        href: `#/prova/${encodeURIComponent(a.attempt_id)}/${encodeURIComponent(e.exercise_id)}`,
        "aria-current": String(e.exercise_id === selectedId),
      },
      progressMark(e.best_score, e.submissions > 0),
      el("span", { class: "item-body" },
        el("span", { class: "item-title", style: "display:block" }, e.title || e.exercise_id),
        el("span", { class: "mini-bar" }, el("span", { style: `width: ${e.best_score * 100}%` }))),
      el("span", { class: "item-meta" }, `${fmtPoints(e.points)}/${e.max_points}`)))),
  );
}

// ============================================================
// Workspace: problem, editor and results
// ============================================================

function skeleton(ex) {
  return `package solution\n\n${ex.signature} {\n\t// Escreva aqui a sua solução.\n}\n`;
}

const draftKey = (scope, id) => studentKey(`draft:${scope}:${id}`);

async function renderWorkspace({ exerciseId, scope }) {
  const main = $("main");
  let ex = state.details.get(exerciseId);
  if (!ex) {
    main.replaceChildren(el("div", { class: "workspace" }, el("div", { class: "results-empty" }, el("div", { class: "spinner" }))));
    try {
      ex = await api("GET", `/api/exercises/${encodeURIComponent(exerciseId)}`);
      state.details.set(exerciseId, ex);
    } catch (err) {
      main.replaceChildren(el("div", { class: "page" }, el("div", { class: "results-empty" },
        icon("alert", 28), el("strong", {}, "Exercício não encontrado"), err.message)));
      return;
    }
  }

  const results = el("section", { class: "results", "aria-live": "polite" });
  const locked = scope !== "practice" && state.attempt?.attempt_id === scope && !state.attempt.open;
  const editor = createEditor(ex, scope, (code, button) => submit(ex, scope, code, button, results), locked);
  const reportKey = `${scope}/${ex.id}`;
  if (state.reports.has(reportKey)) showReport(results, state.reports.get(reportKey), ex);
  else if (locked) {
    results.replaceChildren(el("div", { class: "results-empty" }, icon("clock", 26),
      el("strong", {}, "A prova terminou"),
      el("span", {}, "Já não é possível submeter. A nota final está na barra lateral.")));
  } else showEmptyResults(results);

  const examples = ex.tests.slice(0, 6).map((t) =>
    el("code", { class: "example", title: `${ex.function}(${t.input.map((v) => JSON.stringify(v)).join(", ")})` },
      `${ex.function}(${t.input.map((v) => JSON.stringify(v)).join(", ")})`));

  main.replaceChildren(el("div", { class: "workspace" },
    el("div", { class: "problem-head" },
      el("div", {},
        el("h1", {}, ex.title),
        el("div", { class: "meta" },
          el("span", { class: `pill ${ex.difficulty}` }, el("span", { class: `dot ${ex.difficulty}` }),
            `${LEVEL_LABELS[ex.difficulty]} · ${plural(LEVEL_POINTS[ex.difficulty], "ponto", "pontos")}`),
          el("span", { class: "pill" }, icon("list", 13), plural(ex.tests.length, "teste", "testes")),
          el("span", { class: "pill" }, icon("clock", 13), `${ex.timeout_ms} ms por teste`)))),
    el("p", { class: "description" }, ex.description),
    el("div", { class: "split" },
      el("div", {},
        el("h2", { class: "panel-title" }, "Algumas chamadas de teste"),
        el("div", { class: "examples" }, examples,
          ex.tests.length > examples.length ? el("span", { class: "example" }, `+${ex.tests.length - examples.length}`) : null),
        editor),
      results),
  ));
}

// createEditor returns a code editor with line numbers, auto-indent and
// Ctrl+Enter to submit.
function createEditor(ex, scope, onSubmit, locked = false) {
  const key = draftKey(scope, ex.id);
  const textarea = el("textarea", {
    spellcheck: "false",
    autocapitalize: "off",
    autocomplete: "off",
    wrap: "off",
    "aria-label": `Código da solução de ${ex.title}`,
  });
  textarea.value = storage.get(key, null) ?? skeleton(ex);
  const gutter = el("pre", { class: "gutter", "aria-hidden": "true" });
  const position = el("span", {});
  const submitButton = el("button", { type: "button", class: "btn btn-primary btn-sm" }, icon("play", 13), "Submeter", el("kbd", {}, "Ctrl ↵"));

  function refresh() {
    const lines = textarea.value.split("\n").length;
    gutter.textContent = Array.from({ length: lines }, (_, i) => i + 1).join("\n");
    gutter.scrollTop = textarea.scrollTop;
    const before = textarea.value.slice(0, textarea.selectionStart).split("\n");
    position.textContent = `Ln ${before.length}, Col ${before[before.length - 1].length + 1}`;
  }
  function save() {
    storage.set(key, textarea.value);
    refresh();
  }
  function insert(text, cursorOffset = text.length) {
    const start = textarea.selectionStart;
    textarea.setRangeText(text, start, textarea.selectionEnd, "end");
    textarea.selectionStart = textarea.selectionEnd = start + cursorOffset;
    save();
  }

  textarea.addEventListener("input", save);
  textarea.addEventListener("scroll", () => { gutter.scrollTop = textarea.scrollTop; });
  textarea.addEventListener("click", refresh);
  textarea.addEventListener("keyup", refresh);
  textarea.addEventListener("keydown", (event) => {
    const { value, selectionStart: pos } = textarea;
    const lineStart = value.lastIndexOf("\n", pos - 1) + 1;
    const indent = value.slice(lineStart, pos).match(/^[\t ]*/)[0];

    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      submitButton.click();
    } else if (event.key === "Tab") {
      event.preventDefault();
      insert("\t");
    } else if (event.key === "Enter") {
      event.preventDefault();
      const opens = value[pos - 1] === "{";
      if (opens && value[pos] === "}") insert(`\n${indent}\t\n${indent}`, indent.length + 2);
      else insert(`\n${indent}${opens ? "\t" : ""}`);
    } else if (event.key === "}" && /^[\t ]+$/.test(value.slice(lineStart, pos)) && indent.endsWith("\t")) {
      // Closing brace on an indented empty line: dedent it.
      event.preventDefault();
      textarea.setRangeText(`${indent.slice(0, -1)}}`, lineStart, pos, "end");
      save();
    }
  });

  submitButton.addEventListener("click", () => onSubmit(textarea.value, submitButton));
  const reset = el("button", {
    type: "button",
    class: "btn btn-ghost btn-sm",
    title: "Repor o esqueleto inicial",
    onclick: () => {
      if (!confirm("Apagar o código e repor o esqueleto inicial?")) return;
      textarea.value = skeleton(ex);
      storage.remove(key);
      refresh();
      textarea.focus();
    },
  }, icon("reset", 13), "Repor");

  refresh();
  if (locked) {
    submitButton.disabled = true;
    submitButton.title = "A prova terminou";
    textarea.readOnly = true;
  }
  return el("div", { class: "editor" },
    el("div", { class: "editor-bar" },
      el("span", { class: "editor-file" }, icon("file", 14), "solution.go", el("span", { class: "sig" }, `— ${ex.signature}`)),
      reset, submitButton),
    el("div", { class: "editor-body" }, gutter, textarea),
    el("div", { class: "editor-foot" },
      el("span", {}, locked ? "Prova terminada: o código já não pode ser submetido" : "Go · tabs · rascunho guardado automaticamente"),
      position));
}

async function submit(ex, scope, code, button, results) {
  button.disabled = true;
  results.replaceChildren(el("div", { class: "results-empty" },
    el("div", { class: "spinner" }), el("strong", {}, "A avaliar…"), "A verificar, compilar e correr os testes."));
  try {
    // gofmt expects a final newline; add it so it does not cause a warning.
    const body = { exercise_id: ex.id, code: code.endsWith("\n") ? code : `${code}\n`, student: state.student };
    if (scope !== "practice") body.attempt_id = scope;
    const report = await api("POST", "/api/submissions", body);
    state.reports.set(`${scope}/${ex.id}`, report);
    showReport(results, report, ex);

    if (scope === "practice") {
      const best = bestScores();
      best[ex.id] = Math.max(best[ex.id] || 0, report.summary.score);
      storage.set(studentKey("best"), best);
      renderPracticeSidebar(ex.id);
    } else if (state.attempt && state.attempt.attempt_id === scope) {
      setAttempt(await api("GET", `/api/attempts/${encodeURIComponent(scope)}`));
      renderAttemptSidebar(ex.id);
    }
  } catch (err) {
    toast(err.message, "error");
    showEmptyResults(results);
  } finally {
    button.disabled = false;
  }
}

// ============================================================
// Report
// ============================================================

function showEmptyResults(results) {
  results.replaceChildren(el("div", { class: "results-empty" },
    icon("play", 26),
    el("strong", {}, "Ainda sem resultados"),
    el("span", {}, "Escreva a função e carregue em Submeter (ou Ctrl+Enter).")));
}

function showReport(results, r, ex) {
  const s = r.summary;
  const v = VERDICTS[s.verdict] || { label: s.verdict, tone: "err", icon: "alert" };
  const checks = new Map(r.static.checks.map((c) => [c.name, c]));
  const failedCheck = s.stopped_at ? checks.get(s.stopped_at) : null;

  let subtitle;
  switch (s.verdict) {
    case "passed": subtitle = `Todos os ${s.total} testes passaram.`; break;
    case "failed": subtitle = `${s.passed} de ${s.total} testes passaram.`; break;
    case "timeout": subtitle = `Um teste excedeu o tempo limite. ${s.passed} de ${s.total} passaram.`; break;
    case "rejected": subtitle = `O código foi rejeitado na etapa «${STAGE_LABELS[s.stopped_at] || s.stopped_at}».`; break;
    case "compile_error": subtitle = "O código não compila."; break;
    default: subtitle = "Ocorreu um erro interno durante a avaliação.";
  }

  const parts = [];

  // Verdict with score and one bar segment per test.
  parts.push(el("div", { class: `card verdict tone-${v.tone}` },
    el("div", { class: "verdict-top" },
      el("span", { class: "verdict-icon" }, icon(v.icon, 20)),
      el("div", { class: "verdict-text" }, el("h2", {}, v.label), el("p", {}, subtitle)),
      el("span", { class: "verdict-score" }, pct(s.score))),
    r.dynamic.executed ? el("div", { class: "test-bar", "aria-hidden": "true" },
      r.dynamic.tests.map((t) => el("span", { class: t.passed ? "" : "fail", title: t.passed ? "passou" : "falhou" }))) : null));

  // What stopped the pipeline, shown prominently.
  if (failedCheck && failedCheck.messages && failedCheck.messages.length) {
    parts.push(el("div", { class: "card stop-detail" },
      el("h3", {}, icon("alert"), `${STAGE_LABELS[failedCheck.name] || failedCheck.name}: o que corrigir`),
      el("pre", { class: "code-block" }, failedCheck.messages.join("\n"))));
  }

  // Pipeline stages and non-blocking notes.
  const stageChip = (name, status) => el("span", { class: `stage ${status}`, title: status },
    icon({ ok: "check", warning: "alert", error: "x", skipped: "minus", pending: "minus" }[status], 12),
    STAGE_LABELS[name] || name);
  const staticStages = STAGES.map((name) => stageChip(name, checks.get(name)?.status || "pending"));
  const runStatus = r.dynamic.executed ? (s.verdict === "passed" ? "ok" : "error") : "pending";
  const totalMs = (r.dynamic.tests || []).reduce((sum, t) => sum + t.duration_ms, 0);
  const notes = r.static.checks.filter((c) => (c.status === "warning" || c.status === "skipped") && c.messages && c.messages.length);
  parts.push(el("div", { class: "card pipeline" },
    el("h3", { class: "panel-title", style: "margin:0" }, "Análise estática"),
    el("div", { class: "stages" }, staticStages),
    el("h3", { class: "panel-title", style: "margin:0" }, "Análise dinâmica"),
    el("div", { class: "stages" }, stageChip("run", runStatus),
      el("span", { class: "stage-note" }, r.dynamic.executed
        ? `${s.passed}/${s.total} testes passaram · ${totalMs} ms no total`
        : "não executada: o código não passou a análise estática")),
    notes.length ? el("ul", { class: "notes" }, notes.map((c) => el("li", { class: c.status },
      icon(c.status === "warning" ? "alert" : "info", 14),
      el("span", {}, el("strong", {}, `${STAGE_LABELS[c.name] || c.name}: `), c.messages.join(" · "))))) : null));

  // Tests.
  if (r.dynamic.executed) {
    const failedOnly = el("input", { type: "checkbox" });
    const list = el("div", {});
    const renderTests = () => {
      list.replaceChildren(...r.dynamic.tests.map((t, i) => [t, i])
        .filter(([t]) => !failedOnly.checked || !t.passed)
        .map(([t, i]) => testRow(t, i, ex)));
    };
    failedOnly.addEventListener("change", renderTests);
    renderTests();
    parts.push(el("div", { class: "card tests-card" },
      el("div", { class: "tests-head" },
        el("h3", {}, `Testes · ${s.passed}/${s.total}`),
        s.passed < s.total ? el("label", { class: "toggle" }, failedOnly, "Só os que falharam") : null),
      list));
  }

  parts.push(el("p", { class: "submission-id" }, "Submissão ", el("code", {}, r.submission_id)));
  results.replaceChildren(...parts);
}

function testRow(t, i, ex) {
  const call = `${ex.function}(${t.input.map((v) => JSON.stringify(v)).join(", ")})`;
  const got = t.got === undefined ? "—" : JSON.stringify(t.got);
  return el("details", { class: `test ${t.passed ? "pass" : "fail"}`, open: !t.passed },
    el("summary", {},
      el("span", { class: "test-status" }, icon(t.passed ? "check" : "x", 12)),
      el("span", { class: "test-num" }, `#${i + 1}`),
      el("code", { class: "test-call", title: call }, call),
      el("span", { class: "test-time" }, `${t.duration_ms} ms`),
      el("span", { class: "chevron" }, icon("chevron", 14))),
    el("div", { class: "test-detail" },
      el("div", { class: "compare" },
        el("div", {}, el("label", {}, "Esperado"), el("pre", { class: "value" }, JSON.stringify(t.expected))),
        el("div", {}, el("label", {}, "Obtido"), el("pre", { class: `value ${t.passed ? "" : "bad"}` }, got))),
      t.error ? el("div", { class: "callout error" }, icon("alert", 14), t.error) : null,
      t.hint ? el("div", { class: "callout hint" }, icon("bulb", 14), `Dica: ${t.hint}`) : null));
}

// ============================================================
// Student identity ("fake" login: a name, no password)
// ============================================================

function renderUser() {
  const box = $("user");
  if (!state.student) {
    box.replaceChildren();
    return;
  }
  box.replaceChildren(
    el("span", { class: "user-name", title: "Aluno" }, icon("user", 14), el("span", {}, state.student)),
    el("button", {
      type: "button",
      class: "btn btn-ghost btn-sm",
      title: "Mudar de aluno",
      onclick: () => {
        state.student = "";
        state.attempt = null;
        storage.remove("gavel:student");
        go();
        route();
      },
    }, "Sair"));
}

function renderStudentLogin() {
  const input = el("input", {
    class: "input input-lg",
    placeholder: "ex.: Ana Silva ou up202412345",
    maxlength: "80",
    autocomplete: "name",
    "aria-label": "Nome ou número de aluno",
    required: true,
  });
  $("main").replaceChildren(el("div", { class: "login" },
    el("form", {
      class: "card login-card",
      onsubmit: (event) => {
        event.preventDefault();
        const name = input.value.trim();
        if (!name) return;
        state.student = name;
        storage.set("gavel:student", name);
        route();
      },
    },
    el("div", { class: "login-icon" }, icon("user", 22)),
    el("h1", {}, "Bem-vindo ao Gavel"),
    el("p", {}, "Indique o seu nome ou número de aluno. Fica associado às suas submissões, para o docente as poder consultar."),
    el("label", { class: "field" }, el("span", {}, "Nome ou número"), input),
    el("button", { type: "submit", class: "btn btn-primary" }, "Entrar"),
    el("p", { class: "login-note" }, "Não há palavra-passe: o nome serve apenas para identificar as submissões. ",
      el("a", { href: "#/docente" }, "Sou docente →")))));
  input.focus();
}

// ============================================================
// Admin (teacher) area
// ============================================================

const TONE_BY_VERDICT = { passed: "ok", failed: "warn", timeout: "warn" };

function verdictPill(verdict) {
  const v = VERDICTS[verdict] || { label: verdict };
  return el("span", { class: `pill tone-${TONE_BY_VERDICT[verdict] || "err"}` }, v.label);
}

const fmtDate = (iso) => new Date(iso).toLocaleString("pt-PT", { dateStyle: "short", timeStyle: "medium" });
const exerciseTitle = (id) => state.exercises.find((e) => e.id === id)?.title || id;
const anonymous = (name) => name || "(anónimo)";

function adminApi(path, method = "GET", body = undefined) {
  return api(method, path, body, { Authorization: `Bearer ${state.adminToken}` });
}

async function loadAdminData() {
  const [sessions, submissions, attempts] = await Promise.all([
    adminApi("/api/admin/sessions"),
    adminApi("/api/admin/submissions"),
    adminApi("/api/admin/attempts"),
  ]);
  // Newest first.
  state.admin.sessions = sessions.reverse();
  state.admin.submissions = submissions.reverse();
  state.admin.attempts = attempts.reverse();
}

function renderAdmin() {
  if (!state.adminToken) {
    renderAdminLogin();
    return;
  }
  loadAdminData().then(renderAdminDashboard).catch((err) => {
    state.adminToken = "";
    sessionSet("gavel:admin", "");
    renderAdminLogin(err.message);
  });
}

function renderAdminLogin(error) {
  const input = el("input", { class: "input input-lg", type: "password", autocomplete: "current-password", "aria-label": "Palavra-passe", required: true });
  const button = el("button", { type: "submit", class: "btn btn-primary" }, "Entrar");
  $("main").replaceChildren(el("div", { class: "login" },
    el("form", {
      class: "card login-card",
      onsubmit: async (event) => {
        event.preventDefault();
        button.disabled = true;
        state.adminToken = input.value;
        try {
          await loadAdminData();
          sessionSet("gavel:admin", state.adminToken);
          renderAdminDashboard();
        } catch (err) {
          state.adminToken = "";
          renderAdminLogin(err.message);
        }
      },
    },
    el("div", { class: "login-icon" }, icon("lock", 22)),
    el("h1", {}, "Área do docente"),
    el("p", {}, "Consulte todas as submissões, tentativas e resultados dos alunos."),
    error ? el("div", { class: "callout error" }, icon("alert", 14), error) : null,
    el("label", { class: "field" }, el("span", {}, "Palavra-passe"), input),
    button,
    el("p", { class: "login-note" }, "A palavra-passe é definida por quem arranca o servidor, na variável GAVEL_ADMIN_PASSWORD."))));
  input.focus();
}

function renderAdminDashboard() {
  const a = state.admin;
  const subs = a.submissions;
  const students = new Set(subs.map((s) => s.student || "").concat(a.attempts.map((t) => t.student || "")));
  const passed = subs.filter((s) => s.verdict === "passed").length;

  const openSessions = a.sessions.filter((x) => x.open).length;
  const tabs = [["sessions", "Provas"], ["submissions", "Submissões"], ["attempts", "Tentativas"], ["students", "Alunos"]];
  const autoRefresh = el("input", { type: "checkbox" });
  autoRefresh.checked = Boolean(a.autoRefresh);
  autoRefresh.addEventListener("change", () => {
    a.autoRefresh = autoRefresh.checked;
    renderAdminDashboard();
  });
  clearInterval(a.timer);
  if (a.autoRefresh) a.timer = setInterval(refreshAdmin, 15000);

  const content = el("div", {});
  $("main").replaceChildren(el("div", { class: "page page-wide" },
    el("div", { class: "admin-head" },
      el("div", {},
        el("h1", {}, "Área do docente"),
        el("p", { class: "muted-text" }, `Atualizado às ${new Date().toLocaleTimeString("pt-PT")}`)),
      el("div", { class: "admin-actions" },
        el("label", { class: "toggle" }, autoRefresh, "Atualizar a cada 15 s"),
        el("button", { type: "button", class: "btn btn-ghost btn-sm", onclick: refreshAdmin }, icon("refresh", 13), "Atualizar"),
        el("button", {
          type: "button",
          class: "btn btn-ghost btn-sm",
          onclick: () => {
            state.adminToken = "";
            sessionSet("gavel:admin", "");
            clearInterval(a.timer);
            renderAdminLogin();
          },
        }, icon("exit", 13), "Terminar sessão"))),
    el("div", { class: "stats" },
      [[openSessions, "provas abertas"], [subs.length, "submissões"], [students.size, "alunos"],
        [subs.length ? pct(passed / subs.length) : "—", "submissões aprovadas"]]
        .map(([value, label]) => el("div", { class: "card stat" }, el("span", { class: "stat-value" }, value), el("span", { class: "stat-label" }, label)))),
    el("div", { class: "admin-toolbar" },
      el("nav", { class: "segmented" }, tabs.map(([id, label]) => el("button", {
        type: "button",
        "aria-selected": String(a.tab === id),
        onclick: () => {
          a.tab = id;
          renderAdminDashboard();
        },
      }, label)))),
    content));
  renderAdminTab(content);
}

async function refreshAdmin() {
  try {
    await loadAdminData();
    if (parseRoute().mode === "admin") renderAdminDashboard();
  } catch (err) {
    toast(err.message, "error");
  }
}

function renderAdminTab(content) {
  const a = state.admin;
  if (a.tab === "sessions") content.replaceChildren(...sessionsPanel());
  else if (a.tab === "attempts") content.replaceChildren(attemptsTable());
  else if (a.tab === "students") content.replaceChildren(studentsTable());
  else content.replaceChildren(submissionsTable());
}

// showSubmissionsFor switches to the submissions tab filtered by text.
function showSubmissionsFor(text) {
  state.admin.tab = "submissions";
  state.admin.query = text;
  state.admin.verdict = "";
  renderAdminDashboard();
}

function submissionsTable() {
  const a = state.admin;
  const search = el("input", { class: "input", type: "search", placeholder: "Filtrar por aluno, exercício ou tentativa…", value: a.query, "aria-label": "Filtrar submissões" });
  const verdict = el("select", { class: "input select", "aria-label": "Filtrar por veredito" },
    el("option", { value: "" }, "Todos os vereditos"),
    Object.entries(VERDICTS).map(([id, v]) => el("option", { value: id }, v.label)));
  verdict.value = a.verdict;
  const body = el("tbody", {});
  const count = el("span", { class: "muted-text" });

  function fill() {
    const q = a.query.trim().toLowerCase();
    const rows = a.submissions.filter((s) =>
      (!a.verdict || s.verdict === a.verdict) &&
      (!q || [s.student, s.exercise_id, exerciseTitle(s.exercise_id), s.attempt_id, s.submission_id]
        .some((v) => (v || "").toLowerCase().includes(q))));
    count.textContent = plural(rows.length, "submissão", "submissões");
    body.replaceChildren(...(rows.length ? rows.map((s) => el("tr", { tabindex: "0", onclick: () => openSubmission(s), onkeydown: (e) => e.key === "Enter" && openSubmission(s) },
      el("td", { class: "nowrap" }, fmtDate(s.submitted_at)),
      el("td", {}, anonymous(s.student)),
      el("td", {}, exerciseTitle(s.exercise_id)),
      el("td", { class: "mono" }, s.attempt_id || "prática"),
      el("td", {}, verdictPill(s.verdict)),
      el("td", { class: "num" }, s.total ? `${s.passed}/${s.total}` : "—"),
      el("td", { class: "num" }, el("strong", {}, pct(s.score)))))
      : [el("tr", {}, el("td", { colspan: "7", class: "empty-cell" }, "Nenhuma submissão."))]));
  }
  search.addEventListener("input", () => { a.query = search.value; fill(); });
  verdict.addEventListener("change", () => { a.verdict = verdict.value; fill(); });
  fill();

  return el("div", { class: "card table-card" },
    el("div", { class: "table-filters" }, search, verdict, count),
    el("div", { class: "table-wrap" }, el("table", { class: "data" },
      el("thead", {}, el("tr", {}, ["Data", "Aluno", "Exercício", "Tentativa", "Veredito", "Testes", "Nota"].map((h) => el("th", {}, h)))),
      body)));
}

function attemptsTable() {
  const a = state.admin;
  const search = el("input", { class: "input", type: "search", placeholder: "Filtrar por aluno, prova ou código…", value: a.attemptsQuery, "aria-label": "Filtrar tentativas" });
  const body = el("tbody", {});
  const count = el("span", { class: "muted-text" });
  function fill() {
    const q = a.attemptsQuery.trim().toLowerCase();
    const rows = a.attempts.filter((t) => !q || [t.student, t.title, t.session_id, t.attempt_id].some((v) => (v || "").toLowerCase().includes(q)));
    count.textContent = plural(rows.length, "tentativa", "tentativas");
    body.replaceChildren(...(rows.length ? rows.map((t) => {
      const submitted = t.score.exercises.filter((e) => e.submissions > 0).length;
      return el("tr", { tabindex: "0", title: "Ver as submissões desta tentativa", onclick: () => showSubmissionsFor(t.attempt_id), onkeydown: (e) => e.key === "Enter" && showSubmissionsFor(t.attempt_id) },
        el("td", { class: "nowrap" }, fmtDate(t.started_at)),
        el("td", {}, anonymous(t.student)),
        el("td", {}, t.title || t.session_id),
        el("td", {}, t.open ? el("span", { class: "pill tone-ok" }, "a decorrer") : el("span", { class: "pill" }, "terminada")),
        el("td", { class: "num" }, `${submitted}/${t.exercise_ids.length}`),
        el("td", { class: "num" }, `${fmtPoints(t.score.points)} / ${t.score.max_points}`),
        el("td", {}, el("div", { class: "score-cell" },
          el("span", { class: "mini-bar" }, el("span", { style: `width: ${t.score.total * 100}%` })),
          el("strong", {}, pct(t.score.total)))));
    }) : [el("tr", {}, el("td", { colspan: "7", class: "empty-cell" }, "Nenhuma tentativa."))]));
  }
  search.addEventListener("input", () => { a.attemptsQuery = search.value; fill(); });
  fill();
  return el("div", { class: "card table-card" },
    el("div", { class: "table-filters" }, search, count),
    el("div", { class: "table-wrap" }, el("table", { class: "data" },
      el("thead", {}, el("tr", {}, ["Início", "Aluno", "Prova", "Estado", "Exercícios submetidos", "Pontos", "Nota"].map((h) => el("th", {}, h)))),
      body)));
}

// ------------------------------------------------------------
// Admin: exam sessions (create, follow, close)
// ------------------------------------------------------------

function sessionsPanel() {
  return [newSessionForm(), sessionsTable()];
}

function newSessionForm() {
  const available = Object.fromEntries(LEVELS.map((level) => [level, state.exercises.filter((e) => e.difficulty === level).length]));
  const first = state.exams[0];
  const title = el("input", { class: "input", id: "new-title", value: first ? first.title : "", maxlength: "80", required: true });
  const preset = el("select", { class: "input select", id: "new-preset" },
    state.exams.map((e) => el("option", { value: e.id }, e.title)),
    el("option", { value: "custom" }, "Personalizada (sorteio)"),
    el("option", { value: "pick" }, "Escolher exercícios"));
  const counts = Object.fromEntries(LEVELS.map((level) => [level, el("input", {
    class: "input num-input", id: `new-${level}`, type: "number", min: "0", max: String(available[level]),
    value: String(first?.composition[level] || 0),
  })]));
  const duration = el("input", { class: "input num-input", id: "new-duration", type: "number", min: "1", max: "480", value: "60", required: true });
  const summary = el("p", { class: "form-summary" });
  const intro = el("p", { class: "muted-text" });
  const button = el("button", { type: "submit", class: "btn btn-primary" }, icon("play", 14), "Abrir prova");

  // Picked exercises, in the order of the list (by level, then title).
  const picked = new Set();
  const picker = exercisePicker(picked, updateSummary);
  const countFields = LEVELS.map((level) => el("label", { class: "field", for: `new-${level}` },
    el("span", {}, el("span", { class: `dot ${level}` }), ` ${LEVEL_LABELS[level]} (máx. ${available[level]})`), counts[level]));

  const picking = () => preset.value === "pick";
  const pickedIDs = () => state.exercises.filter((e) => picked.has(e.id)).map((e) => e.id);
  const composition = () => {
    if (picking()) {
      const comp = {};
      for (const id of picked) {
        const level = state.exercises.find((e) => e.id === id)?.difficulty;
        if (level) comp[level] = (comp[level] || 0) + 1;
      }
      return comp;
    }
    return Object.fromEntries(LEVELS.map((level) => [level, Number(counts[level].value) || 0]).filter(([, n]) => n > 0));
  };
  function updateSummary() {
    const { total, points } = compositionFacts(composition());
    const minutes = Number(duration.value) || 0;
    const ends = new Date(Date.now() + minutes * 60000).toLocaleTimeString("pt-PT", { hour: "2-digit", minute: "2-digit" });
    summary.textContent = `${plural(total, "exercício", "exercícios")} · ${plural(points, "ponto", "pontos")} · ${minutes} min, termina às ${ends}`;
  }
  function updateMode() {
    for (const field of countFields) field.hidden = picking();
    picker.hidden = !picking();
    intro.textContent = picking()
      ? "Escolha os exercícios da prova. Todos os alunos recebem estes exercícios, pela ordem da lista (nível, depois título). A prova fecha sozinha quando o tempo acabar."
      : "Os exercícios são sorteados uma vez e são os mesmos para todos os alunos. A prova fecha sozinha quando o tempo acabar.";
    updateSummary();
  }

  let lastPresetTitle = first ? first.title : "";
  preset.addEventListener("change", () => {
    const chosen = state.exams.find((e) => e.id === preset.value);
    if (chosen) {
      for (const level of LEVELS) counts[level].value = String(chosen.composition[level] || 0);
      if (!title.value.trim() || title.value === lastPresetTitle) title.value = chosen.title;
      lastPresetTitle = chosen.title;
    }
    updateMode();
  });
  for (const level of LEVELS) {
    counts[level].addEventListener("input", () => {
      preset.value = "custom";
      updateSummary();
    });
  }
  duration.addEventListener("input", updateSummary);
  updateMode();

  return el("form", {
    class: "card new-session",
    onsubmit: async (event) => {
      event.preventDefault();
      if (picking() && picked.size === 0) {
        toast("Escolha pelo menos um exercício.", "error");
        return;
      }
      button.disabled = true;
      const body = { title: title.value.trim(), duration_minutes: Number(duration.value) };
      if (picking()) body.exercise_ids = pickedIDs();
      else body.composition = composition();
      try {
        const created = await adminApi("/api/admin/sessions", "POST", body);
        toast(`Prova "${created.title}" aberta até às ${fmtClock(created.ends_at)}.`, "success");
        await loadAdminData();
        renderAdminDashboard();
      } catch (err) {
        toast(err.message, "error");
        button.disabled = false;
      }
    },
  },
  el("h2", {}, "Abrir uma prova"),
  intro,
  el("div", { class: "form-grid" },
    el("label", { class: "field field-wide", for: "new-title" }, el("span", {}, "Título"), title),
    el("label", { class: "field", for: "new-preset" }, el("span", {}, "Exercícios"), preset),
    countFields,
    el("label", { class: "field", for: "new-duration" }, el("span", {}, "Duração (minutos)"), duration)),
  picker,
  el("div", { class: "form-actions" }, summary, button));
}

// exercisePicker is a searchable checklist of every exercise, grouped by
// level. It fills the picked set and calls onChange after every change.
function exercisePicker(picked, onChange) {
  const search = el("input", { class: "input", type: "search", id: "pick-search", placeholder: "Procurar exercício…", "aria-label": "Procurar exercício" });
  const count = el("span", { class: "muted-text" });
  const list = el("div", { class: "pick-list" });
  const clear = el("button", { type: "button", class: "btn btn-ghost btn-sm" }, "Limpar");

  function render() {
    const q = search.value.trim().toLowerCase();
    count.textContent = `${plural(picked.size, "escolhido", "escolhidos")}`;
    list.replaceChildren(...LEVELS.flatMap((level) => {
      const items = state.exercises.filter((e) => e.difficulty === level &&
        (!q || e.title.toLowerCase().includes(q) || e.id.includes(q)));
      if (!items.length) return [];
      return [
        el("div", { class: "group-label" }, `${LEVEL_LABELS[level]} · ${plural(LEVEL_POINTS[level], "ponto", "pontos")}`),
        ...items.map((e) => {
          const box = el("input", { type: "checkbox", id: `pick-${e.id}`, value: e.id });
          box.checked = picked.has(e.id);
          box.addEventListener("change", () => {
            if (box.checked) picked.add(e.id);
            else picked.delete(e.id);
            count.textContent = `${plural(picked.size, "escolhido", "escolhidos")}`;
            onChange();
          });
          return el("label", { class: "pick-item", for: `pick-${e.id}`, title: `${e.id}: ${e.description}` },
            box, el("span", { class: `dot ${e.difficulty}` }), el("span", { class: "item-title" }, e.title));
        }),
      ];
    }));
  }
  search.addEventListener("input", render);
  clear.addEventListener("click", () => {
    picked.clear();
    render();
    onChange();
  });
  render();
  return el("div", { class: "picker" },
    el("div", { class: "picker-bar" }, search, count, clear),
    list);
}

function sessionsTable() {
  const rows = state.admin.sessions.map((s) => {
    const close = async (event) => {
      event.stopPropagation();
      if (!confirm(`Terminar a prova "${s.title}" agora? Os alunos deixam de poder submeter.`)) return;
      try {
        await adminApi(`/api/admin/sessions/${encodeURIComponent(s.session_id)}/close`, "POST");
        toast(`Prova "${s.title}" terminada.`, "success");
        await loadAdminData();
        renderAdminDashboard();
      } catch (err) {
        toast(err.message, "error");
      }
    };
    const showAttempts = () => {
      state.admin.tab = "attempts";
      state.admin.attemptsQuery = s.session_id;
      renderAdminDashboard();
    };
    const { points } = compositionFacts(s.composition);
    return el("tr", { tabindex: "0", title: "Ver as tentativas desta prova", onclick: showAttempts, onkeydown: (e) => e.key === "Enter" && showAttempts() },
      el("td", {}, el("strong", {}, s.title)),
      el("td", {}, compositionPills(s.composition)),
      el("td", { class: "num" }, `${points}`),
      el("td", { class: "nowrap" }, fmtDate(s.created_at)),
      el("td", { class: "nowrap" }, s.open
        ? el("span", { class: "status-cell" }, el("span", { class: "pill tone-ok" }, "Aberta"), " faltam ", countdown(deadlineOf(s), "0:00"))
        : el("span", { class: "pill" }, `Terminada às ${fmtClock(s.ends_at)}`)),
      el("td", { class: "num" }, s.students),
      el("td", {}, s.open ? el("button", { type: "button", class: "btn btn-ghost btn-sm", onclick: close }, "Terminar agora") : null));
  });
  return el("div", { class: "card table-card" },
    el("div", { class: "table-wrap" }, el("table", { class: "data" },
      el("thead", {}, el("tr", {}, ["Prova", "Composição", "Pontos", "Aberta em", "Estado", "Alunos", ""].map((h) => el("th", {}, h)))),
      el("tbody", {}, rows.length ? rows : el("tr", {}, el("td", { colspan: "7", class: "empty-cell" }, "Ainda não abriu nenhuma prova."))))));
}

function studentsTable() {
  const byStudent = new Map();
  const entry = (name) => {
    if (!byStudent.has(name)) byStudent.set(name, { name, submissions: 0, solved: new Set(), tried: new Set(), last: "", bestExam: null });
    return byStudent.get(name);
  };
  for (const s of state.admin.submissions) {
    const e = entry(s.student || "");
    e.submissions++;
    e.tried.add(s.exercise_id);
    if (s.score >= 1) e.solved.add(s.exercise_id);
    if (s.submitted_at > e.last) e.last = s.submitted_at;
  }
  for (const t of state.admin.attempts) {
    const e = entry(t.student || "");
    if (e.bestExam === null || t.score.total > e.bestExam) e.bestExam = t.score.total;
    if (t.started_at > e.last) e.last = t.started_at;
  }
  const rows = [...byStudent.values()]
    .sort((x, y) => y.solved.size - x.solved.size || x.name.localeCompare(y.name, "pt"))
    .map((e) => el("tr", { tabindex: "0", title: "Ver as submissões deste aluno", onclick: () => showSubmissionsFor(e.name), onkeydown: (ev) => ev.key === "Enter" && showSubmissionsFor(e.name) },
      el("td", {}, el("strong", {}, anonymous(e.name))),
      el("td", { class: "num" }, e.submissions),
      el("td", { class: "num" }, `${e.solved.size} / ${e.tried.size}`),
      el("td", { class: "num" }, e.bestExam === null ? "—" : pct(e.bestExam)),
      el("td", { class: "nowrap" }, e.last ? fmtDate(e.last) : "—")));
  return el("div", { class: "card table-card" },
    el("div", { class: "table-wrap" }, el("table", { class: "data" },
      el("thead", {}, el("tr", {}, ["Aluno", "Submissões", "Resolvidos / tentados", "Melhor prova", "Última atividade"].map((h) => el("th", {}, h)))),
      el("tbody", {}, rows.length ? rows : el("tr", {}, el("td", { colspan: "5", class: "empty-cell" }, "Ainda não há alunos."))))));
}

// openSubmission shows a report, with the student's code, in a side panel.
async function openSubmission(summary) {
  const results = el("section", { class: "results" }, el("div", { class: "results-empty" }, el("div", { class: "spinner" })));
  const code = el("pre", { class: "code-block code-full" });
  const close = () => {
    backdrop.remove();
    document.removeEventListener("keydown", onKey);
  };
  const onKey = (event) => event.key === "Escape" && close();
  const backdrop = el("div", { class: "drawer-backdrop", onclick: (event) => event.target === backdrop && close() },
    el("aside", { class: "drawer", role: "dialog", "aria-modal": "true", "aria-label": "Submissão" },
      el("div", { class: "drawer-head" },
        el("div", {},
          el("h2", {}, `${anonymous(summary.student)} · ${exerciseTitle(summary.exercise_id)}`),
          el("p", { class: "muted-text" }, `${fmtDate(summary.submitted_at)} · ${summary.attempt_id ? `tentativa ${summary.attempt_id}` : "prática"}`)),
        el("button", { type: "button", class: "btn btn-ghost btn-sm btn-icon", "aria-label": "Fechar", onclick: close }, icon("x", 14))),
      el("h3", { class: "panel-title" }, "Código submetido"),
      code,
      el("h3", { class: "panel-title" }, "Relatório"),
      results));
  document.addEventListener("keydown", onKey);
  document.body.append(backdrop);

  try {
    const [report, ex] = await Promise.all([
      api("GET", `/api/submissions/${encodeURIComponent(summary.submission_id)}`),
      state.details.get(summary.exercise_id) || api("GET", `/api/exercises/${encodeURIComponent(summary.exercise_id)}`),
    ]);
    state.details.set(ex.id, ex);
    code.textContent = report.code;
    showReport(results, report, ex);
  } catch (err) {
    results.replaceChildren(el("div", { class: "callout error" }, icon("alert", 14), err.message));
  }
}

// ============================================================
// Start-up
// ============================================================

document.querySelectorAll(".segmented button").forEach((tab) => {
  tab.addEventListener("click", () => {
    if (tab.dataset.mode === "exam") {
      if (state.attempt) go("prova", state.attempt.attempt_id);
      else go("prova");
    } else if (tab.dataset.mode === "admin") {
      go("docente");
    } else {
      go();
    }
  });
});

async function init() {
  try {
    const [exercises, exams] = await Promise.all([api("GET", "/api/exercises"), api("GET", "/api/exams")]);
    state.exercises = exercises.sort((a, b) =>
      LEVELS.indexOf(a.difficulty) - LEVELS.indexOf(b.difficulty) || a.title.localeCompare(b.title, "pt"));
    state.exams = exams;
  } catch (err) {
    $("main").replaceChildren(el("div", { class: "page" }, el("div", { class: "results-empty" },
      icon("alert", 28), el("strong", {}, "Não foi possível carregar o Gavel"), err.message)));
    return;
  }
  window.addEventListener("hashchange", route);
  setInterval(tickCountdowns, 1000);
  route();
}

init();
