"use strict";

// ---------- Labels ----------

const LEVELS = { easy: "fácil", medium: "médio", hard: "difícil" };
const VERDICTS = {
  passed: "Aprovado",
  failed: "Reprovado",
  rejected: "Rejeitado",
  compile_error: "Erro de compilação",
  timeout: "Tempo esgotado",
  internal_error: "Erro interno",
};
const STATUSES = { ok: "ok", warning: "aviso", error: "erro", skipped: "ignorado" };

// ---------- Helpers ----------

// el creates an element. Text is always set with textContent, so that
// content coming from submissions is never interpreted as HTML.
function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(attrs)) {
    if (key === "class") node.className = value;
    else if (key.startsWith("on")) node.addEventListener(key.slice(2), value);
    else node.setAttribute(key, value);
  }
  for (const child of children.flat()) {
    if (child === null || child === undefined || child === false) continue;
    node.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
  return node;
}

function badge(level) {
  return el("span", { class: `badge ${level}` }, LEVELS[level] || level);
}

function pct(x) {
  return `${Math.round(x * 100)}%`;
}

function showError(message) {
  const banner = document.getElementById("error");
  banner.textContent = message;
  banner.hidden = false;
  clearTimeout(showError.timer);
  showError.timer = setTimeout(() => { banner.hidden = true; }, 6000);
}

async function api(method, path, body) {
  const options = { method, headers: {} };
  if (body !== undefined) {
    options.headers["Content-Type"] = "application/json";
    options.body = JSON.stringify(body);
  }
  const resp = await fetch(path, options);
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) throw new Error(data.error || `erro ${resp.status}`);
  return data;
}

// ---------- Editor and report (shared by both modes) ----------

// Drafts are kept per exercise so switching exercises does not lose code.
const drafts = new Map();

function skeleton(ex) {
  return `package solution\n\n${ex.signature} {\n\t\n}\n`;
}

// renderExercise shows an exercise with its editor in container.
// onSubmitted is called with each report.
async function renderExercise(container, exerciseId, attemptId, onSubmitted) {
  container.replaceChildren(el("p", { class: "placeholder" }, "A carregar…"));
  let ex;
  try {
    ex = await api("GET", `/api/exercises/${encodeURIComponent(exerciseId)}`);
  } catch (err) {
    container.replaceChildren(el("p", { class: "test-error" }, err.message));
    return;
  }

  const draftKey = `${attemptId || ""}/${ex.id}`;
  const editor = el("textarea", { class: "editor", spellcheck: "false", "aria-label": "Código da solução" });
  editor.value = drafts.get(draftKey) ?? skeleton(ex);
  editor.addEventListener("input", () => drafts.set(draftKey, editor.value));
  editor.addEventListener("keydown", (event) => {
    if (event.key !== "Tab") return;
    event.preventDefault();
    const { selectionStart: start, selectionEnd: end } = editor;
    editor.setRangeText("\t", start, end, "end");
    drafts.set(draftKey, editor.value);
  });

  const report = el("div", { class: "report" });
  const status = el("span", { class: "muted" });
  const button = el("button", { type: "button" }, "Submeter");
  button.addEventListener("click", async () => {
    button.disabled = true;
    status.textContent = "A avaliar…";
    try {
      const body = { exercise_id: ex.id, code: editor.value };
      if (attemptId) body.attempt_id = attemptId;
      const r = await api("POST", "/api/submissions", body);
      report.replaceChildren(renderReport(r));
      status.textContent = `Submissão ${r.submission_id}`;
      if (onSubmitted) onSubmitted(r);
    } catch (err) {
      status.textContent = "";
      showError(err.message);
    } finally {
      button.disabled = false;
    }
  });

  container.replaceChildren(
    el("div", { class: "exercise-head" }, el("h2", {}, ex.title), badge(ex.difficulty)),
    el("p", {}, ex.description),
    el("p", { class: "muted" }, `Assinatura esperada (tempo limite: ${ex.timeout_ms} ms por teste, ${ex.tests.length} testes):`),
    el("pre", { class: "signature" }, ex.signature),
    editor,
    el("div", { class: "actions" }, button, status),
    report,
  );
}

function renderReport(r) {
  const s = r.summary;
  const verdict = el("div", { class: `verdict ${s.verdict}` },
    VERDICTS[s.verdict] || s.verdict,
    r.dynamic.executed ? el("small", {}, `${s.passed}/${s.total} testes · ${pct(s.score)}`) : null,
    s.stopped_at ? el("small", {}, `parou na etapa ${s.stopped_at}`) : null,
  );

  const checks = el("ul", { class: "checks" }, r.static.checks.map((c) =>
    el("li", {},
      el("span", { class: `status ${c.status}` }, STATUSES[c.status] || c.status),
      c.name,
      c.messages && c.messages.length ? el("ul", {}, c.messages.map((m) => el("li", {}, m))) : null,
    )));

  const parts = [verdict, el("h3", {}, "Verificações estáticas"), checks];
  if (r.dynamic.executed) {
    const rows = r.dynamic.tests.map((t, i) => el("tr", { class: t.passed ? "pass" : "fail" },
      el("td", {}, i + 1),
      el("td", { class: "value" }, t.input.map((v) => JSON.stringify(v)).join(", ")),
      el("td", { class: "value" }, JSON.stringify(t.expected)),
      el("td", { class: "value" }, t.got === undefined ? "—" : JSON.stringify(t.got)),
      el("td", {}, t.passed ? "✔" : "✘"),
      el("td", {}, `${t.duration_ms} ms`),
      el("td", {},
        t.error ? el("div", { class: "test-error" }, t.error) : null,
        t.hint ? el("div", { class: "hint" }, `Dica: ${t.hint}`) : null),
    ));
    parts.push(
      el("h3", {}, "Testes"),
      el("div", { class: "table-wrap" }, el("table", { class: "tests" },
        el("thead", {}, el("tr", {}, ["#", "Entrada", "Esperado", "Obtido", "", "Tempo", "Observações"].map((h) => el("th", {}, h)))),
        el("tbody", {}, rows),
      )),
    );
  }
  return el("div", {}, parts);
}

// ---------- Practice mode ----------

const practice = { level: "", selected: null };

async function loadExercises() {
  const list = document.getElementById("exercise-list");
  const query = practice.level ? `?difficulty=${practice.level}` : "";
  try {
    const exercises = await api("GET", `/api/exercises${query}`);
    const order = Object.keys(LEVELS);
    exercises.sort((a, b) =>
      order.indexOf(a.difficulty) - order.indexOf(b.difficulty) || a.title.localeCompare(b.title, "pt"));
    list.replaceChildren(...exercises.map((ex) => {
      const item = el("li", { "data-id": ex.id, title: ex.description }, el("span", {}, ex.title), badge(ex.difficulty));
      if (ex.id === practice.selected) item.classList.add("selected");
      item.addEventListener("click", () => selectExercise(ex.id));
      return item;
    }));
  } catch (err) {
    showError(err.message);
  }
}

function selectExercise(id) {
  practice.selected = id;
  for (const li of document.querySelectorAll("#exercise-list li")) {
    li.classList.toggle("selected", li.dataset.id === id);
  }
  renderExercise(document.getElementById("practice-content"), id, null, null);
}

// ---------- Exam mode ----------

const exam = { attempt: null, selected: null };

async function loadExams() {
  const cards = document.getElementById("exam-cards");
  try {
    const exams = await api("GET", "/api/exams");
    cards.replaceChildren(...exams.map((e) => {
      const composition = ["easy", "medium", "hard"]
        .filter((level) => e.composition[level])
        .map((level) => el("span", { class: `badge ${level}` }, `${e.composition[level]} × ${LEVELS[level]}`));
      return el("div", { class: "exam-card" },
        el("h3", {}, e.title),
        el("p", {}, e.description),
        el("div", {}, composition),
        el("button", { type: "button", onclick: () => startAttempt(e.id) }, "Iniciar"));
    }));
  } catch (err) {
    showError(err.message);
  }
}

async function startAttempt(examId) {
  try {
    const attempt = await api("POST", `/api/exams/${encodeURIComponent(examId)}/attempts`);
    openAttempt(attempt);
  } catch (err) {
    showError(err.message);
  }
}

async function resumeAttempt(id) {
  try {
    openAttempt(await api("GET", `/api/attempts/${encodeURIComponent(id)}`));
  } catch (err) {
    showError(err.message);
    if (location.hash) history.replaceState(null, "", location.pathname);
  }
}

function openAttempt(attempt) {
  exam.attempt = attempt;
  exam.selected = null;
  // The attempt id lives in the URL, so reloading the page resumes it.
  history.replaceState(null, "", `#attempt=${attempt.attempt_id}`);
  document.getElementById("exam-start").hidden = true;
  document.getElementById("exam-attempt").hidden = false;
  document.getElementById("attempt-title").textContent = attempt.exam_title || attempt.exam_id;
  document.getElementById("attempt-id").textContent = attempt.attempt_id;
  document.getElementById("exam-content").replaceChildren(el("p", { class: "placeholder" }, "Escolha um exercício da prova."));
  renderScore();
}

function renderScore() {
  const score = exam.attempt.score;
  document.getElementById("attempt-total").textContent =
    `${score.points.toFixed(2)} / ${score.max_points} pontos (${pct(score.total)})`;
  const body = document.querySelector("#score-table tbody");
  body.replaceChildren(...score.exercises.map((e) => {
    const row = el("tr", { "data-id": e.exercise_id },
      el("td", {}, e.title || e.exercise_id),
      el("td", {}, badge(e.difficulty)),
      el("td", {}, e.submissions ? pct(e.best_score) : "—"),
      el("td", {}, `${e.points.toFixed(2)} / ${e.max_points}`));
    if (e.exercise_id === exam.selected) row.classList.add("selected");
    row.addEventListener("click", () => selectExamExercise(e.exercise_id));
    return row;
  }));
}

function selectExamExercise(id) {
  exam.selected = id;
  renderScore();
  renderExercise(document.getElementById("exam-content"), id, exam.attempt.attempt_id, refreshAttempt);
}

async function refreshAttempt() {
  try {
    exam.attempt = await api("GET", `/api/attempts/${encodeURIComponent(exam.attempt.attempt_id)}`);
    renderScore();
  } catch (err) {
    showError(err.message);
  }
}

function leaveAttempt() {
  exam.attempt = null;
  history.replaceState(null, "", location.pathname);
  document.getElementById("exam-attempt").hidden = true;
  document.getElementById("exam-start").hidden = false;
}

// ---------- Mode switching and start-up ----------

function setMode(mode) {
  for (const tab of document.querySelectorAll(".tab")) {
    tab.classList.toggle("active", tab.dataset.mode === mode);
  }
  document.getElementById("practice").hidden = mode !== "practice";
  document.getElementById("exam").hidden = mode !== "exam";
}

document.querySelectorAll(".tab").forEach((tab) => {
  tab.addEventListener("click", () => setMode(tab.dataset.mode));
});

document.querySelectorAll(".filter").forEach((button) => {
  button.addEventListener("click", () => {
    practice.level = button.dataset.level;
    document.querySelectorAll(".filter").forEach((b) => b.classList.toggle("active", b === button));
    loadExercises();
  });
});

document.getElementById("resume-form").addEventListener("submit", (event) => {
  event.preventDefault();
  const id = document.getElementById("resume-id").value.trim();
  if (id) resumeAttempt(id);
});

document.getElementById("leave-attempt").addEventListener("click", leaveAttempt);

loadExercises();
loadExams();

const resumeMatch = location.hash.match(/^#attempt=([\w-]+)$/);
if (resumeMatch) {
  setMode("exam");
  resumeAttempt(resumeMatch[1]);
}
