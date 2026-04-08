(function () {
  "use strict";

  // Detect user OS
  function detectOS() {
    var ua = navigator.userAgent || navigator.vendor || window.opera;
    var platform = navigator.platform || "";

    if (/android/i.test(ua)) return "android";
    if (/iPad|iPhone|iPod/.test(ua) || (platform === "MacIntel" && navigator.maxTouchPoints > 1)) return "ios";
    if (/Mac|iPhone|iPad|iPod/.test(platform) || /Mac OS X/.test(ua)) return "macos";
    if (/Win/.test(platform) || /Windows/.test(ua)) return "windows";
    if (/Linux/.test(platform) || /Linux/.test(ua)) return "linux";
    return "unknown";
  }

  function highlightOS() {
    var os = detectOS();
    var cards = document.querySelectorAll(".download-card");

    if (!cards.length) return;

    // Map detected OS to data-os attribute
    var osMap = {
      windows: "windows",
      macos: "macos",
      ios: "ios",
      android: "android",
      linux: "linux",
    };

    var targetOS = osMap[os];
    if (!targetOS) return;

    cards.forEach(function (card) {
      if (card.getAttribute("data-os") === targetOS) {
        card.classList.add("highlighted");
      }
    });
  }

  // Run when DOM is ready
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", highlightOS);
  } else {
    highlightOS();
  }
})();
