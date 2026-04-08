(function () {
  "use strict";

  var THEMES = ["theme-dark", "theme-skeuomorphic", "theme-doodle"];
  var STORAGE_KEY = "unbound-theme";

  function getCurrentTheme() {
    return localStorage.getItem(STORAGE_KEY) || "theme-dark";
  }

  function setTheme(theme) {
    // Remove all theme classes
    THEMES.forEach(function (t) {
      document.body.classList.remove(t);
    });
    // Add new theme class
    document.body.classList.add(theme);
    localStorage.setItem(STORAGE_KEY, theme);

    // Update toggle thumb
    var thumb = document.getElementById("themeThumb");
    if (thumb) {
      if (theme === "theme-dark") {
        thumb.classList.remove("easter-active");
      } else {
        thumb.classList.add("easter-active");
      }
    }
  }

  function cycleTheme() {
    var current = getCurrentTheme();
    var idx = THEMES.indexOf(current);
    var nextIdx = (idx + 1) % THEMES.length;
    setTheme(THEMES[nextIdx]);
  }

  // Initialize theme from storage
  function initTheme() {
    var saved = getCurrentTheme();
    setTheme(saved);
  }

  // Attach click handler
  function initToggle() {
    var toggle = document.getElementById("themeToggle");
    if (toggle) {
      toggle.addEventListener("click", cycleTheme);
    }
  }

  // Run when DOM is ready
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () {
      initTheme();
      initToggle();
    });
  } else {
    initTheme();
    initToggle();
  }
})();
