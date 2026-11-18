if (navigator.clipboard?.writeText) {
  for (const snippet of document.querySelectorAll(".snippet")) {
    const code = snippet.querySelector("code");
    const button = document.createElement("button");
    button.type = "button";
    button.className = "copy-button";
    button.textContent = "Copy";

    const status = document.createElement("span");
    status.className = "copy-status";
    status.setAttribute("aria-live", "polite");
    let reset;

    button.addEventListener("click", async () => {
      clearTimeout(reset);
      try {
        await navigator.clipboard.writeText(code.textContent);
        button.textContent = "Copied";
        status.textContent = "Copied to clipboard.";
      } catch {
        button.textContent = "Retry copy";
        status.textContent = "Copy failed.";
      }
      reset = setTimeout(() => {
        button.textContent = "Copy";
        status.textContent = "";
      }, 2000);
    });

    snippet.prepend(button, status);
  }
}
