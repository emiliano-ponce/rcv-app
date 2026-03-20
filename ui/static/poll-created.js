function copyText(inputId, btn) {
  const el = document.getElementById(inputId);
  navigator.clipboard.writeText(el.value).then(() => {
    const orig = btn.textContent;
    btn.textContent = "Copied!";
    setTimeout(() => (btn.textContent = orig), 2000);
  });
}
