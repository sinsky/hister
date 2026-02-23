window.addEventListener("load", () => {
  document.getElementById("theme-toggle")?.addEventListener("click", toggleTheme);

  const theme = localStorage.getItem("theme") ||
    (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");

  document.querySelector("html")?.setAttribute("data-theme", theme);
});

function toggleTheme(): void {
  const htmlElement = document.querySelector("html");
  if (!htmlElement) {
    return;
  }

  const currentTheme = htmlElement.getAttribute("data-theme");
  const newTheme = currentTheme === "dark" ? "light" : "dark";
  htmlElement.setAttribute("data-theme", newTheme);
  localStorage.setItem("theme", newTheme);
}
