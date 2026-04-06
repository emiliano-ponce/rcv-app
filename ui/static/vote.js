document.addEventListener("DOMContentLoaded", () => {
  Sortable.create(document.getElementById("ranking"), {
    animation: 150, // ms — items slide into place
    ghostClass: "dragging", // applied to the placeholder slot
    onEnd: updateRanking,
  });

  updateRanking();
});

function updateRanking() {
  const cards = document.querySelectorAll("#ranking .cand-card");
  const ids = [];

  cards.forEach((card, i) => {
    ids.push(card.dataset.id);
    card.querySelector(".rank-badge").textContent = i + 1;
  });

  document.getElementById("rankings-input").value = ids.join(",");

  const count = ids.length;
  document.getElementById("rank-count").textContent =
    `${count} candidate${count !== 1 ? "s" : ""} ranked.`;
}
