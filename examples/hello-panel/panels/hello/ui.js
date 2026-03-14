// Hello World — minimal Vel panel example
(function () {
  const root = document.getElementById("panel-root");
  if (!root) return;

  function render() {
    const now = new Date().toLocaleString();
    root.innerHTML = `
      <div style="padding:2rem;font-family:system-ui,sans-serif;text-align:center">
        <h1>Hello from Vel!</h1>
        <p style="color:#666">${now}</p>
      </div>
    `;
  }

  render();
  setInterval(render, 1000);
})();
