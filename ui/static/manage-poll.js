(() => {
  const page = document.querySelector("[data-poll-key]");
  const pollKey = page?.dataset.pollKey;
  if (!pollKey) return;

  const BASE = `/polls/${pollKey}`;

  // ── Utility ────────────────────────────────────────────────────────────────

  function flash(el, ok) {
    el.classList.remove("flash-ok", "flash-err");
    void el.offsetWidth; // reflow
    el.classList.add(ok ? "flash-ok" : "flash-err");
  }

  async function api(method, path, body) {
    const res = await fetch(BASE + path, {
      method,
      headers: body ? { "Content-Type": "application/json" } : {},
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      const msg = await res.text().catch(() => res.statusText);
      throw new Error(msg || `HTTP ${res.status}`);
    }
    return res;
  }

  // ── Inline title / description editing ─────────────────────────────────────

  document
    .querySelectorAll(".editable-title, .editable-description")
    .forEach((el) => {
      if (el.classList.contains("closed")) return; // poll closed, no editing

      el.style.cursor = "pointer";

      el.addEventListener("click", () => {
        if (el.querySelector("input, textarea")) return; // already editing

        const field = el.dataset.field;
        const original =
          field === "description"
            ? el.querySelector("em")
              ? ""
              : el.textContent.trim()
            : el.textContent.trim();

        const isTextarea = field === "description";
        const input = document.createElement(isTextarea ? "textarea" : "input");
        input.value = original;
        input.className = "inline-edit-input";
        if (!isTextarea) input.type = "text";

        el.textContent = "";
        el.appendChild(input);
        input.focus();
        input.select();

        async function save() {
          const newVal = input.value.trim();
          if (newVal === original) {
            restore(newVal);
            return;
          }

          // Grab the current sibling value so we send both fields.
          const titleEl = document.querySelector(".editable-title");
          const descEl = document.querySelector(".editable-description");
          const title = field === "title" ? newVal : titleEl.textContent.trim();
          const desc =
            field === "description"
              ? newVal
              : descEl.querySelector("em")
                ? ""
                : descEl.textContent.trim();

          try {
            await api("PATCH", "", { title, description: desc });
            restore(newVal, true);
          } catch (err) {
            alert(`Could not save: ${err.message}`);
            restore(original);
          }
        }

        function restore(val, ok = false) {
          if (field === "description" && !val) {
            el.innerHTML = "<em>No description — click to add one</em>";
          } else {
            el.textContent = val;
          }
          if (ok) flash(el, true);
        }

        input.addEventListener("blur", save);
        input.addEventListener("keydown", (e) => {
          if (e.key === "Enter" && !isTextarea) {
            e.preventDefault();
            save();
          }
          if (e.key === "Escape") {
            e.preventDefault();
            restore(original);
          }
        });
      });
    });

  // ── Candidate list ──────────────────────────────────────────────────────────

  const list = document.getElementById("candidates-list");

  function candidateRow(id, name) {
    const row = document.createElement("div");
    row.className = "candidate-row";
    row.dataset.id = id;
    row.innerHTML = `
      <span class="candidate-name">${escHtml(name)}</span>
      <button class="btn-icon rename-btn" title="Rename" aria-label="Rename ${escHtml(name)}">✎</button>
      <button class="btn-icon delete-btn" title="Remove" aria-label="Remove ${escHtml(name)}">×</button>
    `;
    attachRowListeners(row);
    return row;
  }

  function escHtml(s) {
    return s
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function attachRowListeners(row) {
    row
      .querySelector(".rename-btn")
      ?.addEventListener("click", () => startRename(row));
    row
      .querySelector(".delete-btn")
      ?.addEventListener("click", () => deleteCandidate(row));
  }

  // Wire up existing rows on page load.
  list?.querySelectorAll(".candidate-row").forEach(attachRowListeners);

  function startRename(row) {
    if (row.querySelector("input")) return;
    const nameEl = row.querySelector(".candidate-name");
    const original = nameEl.textContent;

    const input = document.createElement("input");
    input.type = "text";
    input.value = original;
    input.className = "inline-edit-input";

    nameEl.textContent = "";
    nameEl.appendChild(input);
    input.focus();
    input.select();

    async function save() {
      const newName = input.value.trim();
      if (!newName || newName === original) {
        nameEl.textContent = original;
        return;
      }
      try {
        await api("PATCH", `/candidates/${row.dataset.id}`, { name: newName });
        nameEl.textContent = newName;
        flash(row, true);
      } catch (err) {
        alert(`Could not rename: ${err.message}`);
        nameEl.textContent = original;
      }
    }

    input.addEventListener("blur", save);
    input.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        save();
      }
      if (e.key === "Escape") {
        e.preventDefault();
        nameEl.textContent = original;
      }
    });
  }

  async function deleteCandidate(row) {
    const name = row.querySelector(".candidate-name").textContent;
    if (!confirm(`Remove "${name}" from the poll?`)) return;
    try {
      await api("DELETE", `/candidates/${row.dataset.id}`);
      row.remove();
    } catch (err) {
      alert(`Could not remove: ${err.message}`);
    }
  }

  // Add candidate
  const addForm = document.getElementById("add-candidate-form");
  const addInput = document.getElementById("new-candidate-input");

  addForm?.addEventListener("submit", async () => {
    const name = addInput.value.trim();
    if (!name) return;

    try {
      const res = await api("POST", "/candidates", { name });
      const data = await res.json();
      list.appendChild(candidateRow(data.id, data.name));
      addInput.value = "";
      addInput.focus();
    } catch (err) {
      alert(`Could not add candidate: ${err.message}`);
    }
  });

  // ── Close poll ──────────────────────────────────────────────────────────────

  window.closePoll = async function (btn) {
    if (
      !confirm(
        "Close this poll? Voters will no longer be able to submit ballots. This cannot be undone.",
      )
    )
      return;
    btn.disabled = true;
    try {
      await api("POST", "/close");
      // Reload so the page reflects the closed state (badges, no edit controls).
      location.reload();
    } catch (err) {
      alert(`Could not close poll: ${err.message}`);
      btn.disabled = false;
    }
  };

  window.deletePoll = async function (btn) {
    const confirmation = prompt(
      "Type DELETE to permanently delete this poll and all associated ballots/results.",
    );
    if (confirmation !== "DELETE") return;

    btn.disabled = true;
    try {
      await api("DELETE", "");
      location.href = "/";
    } catch (err) {
      alert(`Could not delete poll: ${err.message}`);
      btn.disabled = false;
    }
  };

  // ── copyText (used by the share card) ──────────────────────────────────────

  window.copyText = function (id, btn) {
    const el = document.getElementById(id);
    navigator.clipboard.writeText(el.value).then(() => {
      const orig = btn.textContent;
      btn.textContent = "Copied!";
      setTimeout(() => (btn.textContent = orig), 1500);
    });
  };
})();
