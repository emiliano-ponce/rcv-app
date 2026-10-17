// Auto-dismiss the "Ballot received!" toast after ~4s, or on click.
(function () {
  function initToast() {
    var toast = document.getElementById("voted-toast");
    if (!toast || toast.dataset.bound) return;
    toast.dataset.bound = "1";
    var done = false;
    function remove() {
      if (toast.parentNode) toast.parentNode.removeChild(toast);
    }
    function dismiss() {
      if (done) return;
      done = true;
      var reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      if (reduced) return remove();
      toast.classList.add("hide");
      toast.addEventListener("transitionend", remove, { once: true });
      setTimeout(remove, 600); // fallback if transitionend doesn't fire
    }
    toast.addEventListener("click", dismiss);
    setTimeout(dismiss, 4000);
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initToast);
  } else {
    initToast();
  }
  document.addEventListener("htmx:afterSettle", initToast);
})();
