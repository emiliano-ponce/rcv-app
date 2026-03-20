    let dragged = null;

    function onDragStart(e) {
      dragged = e.currentTarget;
      dragged.classList.add("dragging");
      e.dataTransfer.effectAllowed = "move";
    }
    function onDragEnd(e) {
      e.currentTarget.classList.remove("dragging");
      document
        .querySelectorAll(".drag-over")
        .forEach((el) => el.classList.remove("drag-over"));
    }
    function onDragOver(e) {
      e.preventDefault();
      e.currentTarget.classList.add("drag-over");
      e.dataTransfer.dropEffect = "move";
    }
    function onDragLeave(e) {
      e.currentTarget.classList.remove("drag-over");
    }
    function onDrop(e, targetId) {
      e.preventDefault();
      e.currentTarget.classList.remove("drag-over");
      if (!dragged) return;
      const target = document.getElementById(targetId);
      // Insert before the element under the cursor if in same list
      const afterEl = getDragAfterElement(target, e.clientY);
      if (afterEl) {
        target.insertBefore(dragged, afterEl);
      } else {
        target.appendChild(dragged);
      }
      updateRanking();
    }

    function getDragAfterElement(container, y) {
      const els = [
        ...container.querySelectorAll(".cand-card:not(.dragging)"),
      ];
      return els.reduce(
        (closest, el) => {
          const box = el.getBoundingClientRect();
          const offset = y - box.top - box.height / 2;
          if (offset < 0 && offset > closest.offset) return { offset, el };
          return closest;
        },
        { offset: Number.NEGATIVE_INFINITY },
      ).el;
    }

    // Click to move between pool and ranking
    function moveToRanking(card) {
      const pool = document.getElementById("pool");
      const ranking = document.getElementById("ranking");
      if (card.closest("#pool")) {
        ranking.appendChild(card);
      } else {
        pool.appendChild(card);
      }
      updateRanking();
    }

    function updateRanking() {
      const ranking = document.getElementById("ranking");
      const cards = ranking.querySelectorAll(".cand-card");
      const ids = [];

      cards.forEach((card, i) => {
        ids.push(card.dataset.id);
        // Update or add rank badge
        let badge = card.querySelector(".rank-badge");
        if (!badge) {
          badge = document.createElement("span");
          badge.className = "rank-badge";
          card.insertBefore(badge, card.firstChild);
        }
        badge.textContent = i + 1;
        // Switch click to move back to pool
        card.onclick = () => moveToRanking(card);
      });

      // Remove badges from pool cards
      document
        .getElementById("pool")
        .querySelectorAll(".rank-badge")
        .forEach((b) => b.remove());

      document.getElementById("rankings-input").value = ids.join(",");

      const btn = document.getElementById("submit-btn");
      const hint = document.getElementById("rank-count");
      if (ids.length > 0) {
        btn.disabled = false;
        hint.textContent = `${ids.length} candidate${ids.length > 1 ? "s" : ""} ranked. You can rank more or submit now.`;
      } else {
        btn.disabled = true;
        hint.textContent =
          "Drag at least 1 candidate to your ranking to vote.";
      }
    }