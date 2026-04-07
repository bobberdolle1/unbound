(function () {
  "use strict";

  var REPO_OWNER = "bobberdolle1";
  var REPO_NAME = "unbound";
  var API_URL =
    "https://api.github.com/repos/" + REPO_OWNER + "/" + REPO_NAME + "/releases/latest";

  function fetchReleases() {
    fetch(API_URL, {
      headers: { Accept: "application/vnd.github+json" },
    })
      .then(function (res) {
        if (!res.ok) throw new Error("Failed to fetch releases");
        return res.json();
      })
      .then(function (data) {
        updateDownloadLinks(data);
      })
      .catch(function (err) {
        console.warn("Could not fetch latest release:", err);
        // Fallback: keep the generic latest release URL
      });
  }

  function updateDownloadLinks(release) {
    var tagName = release.tag_name || "latest";
    var assets = release.assets || [];

    // Map asset names to platforms
    var platformMap = {
      windows: null,
      macos: null,
      linux: null,
    };

    assets.forEach(function (asset) {
      var name = asset.name.toLowerCase();
      var browserDownloadUrl = asset.browser_download_url;

      if (name.includes("win") || name.includes("windows") || name.includes("win64")) {
        platformMap.windows = { url: browserDownloadUrl, name: asset.name };
      } else if (
        name.includes("mac") ||
        name.includes("darwin") ||
        name.includes("osx") ||
        name.includes("apple")
      ) {
        platformMap.macos = { url: browserDownloadUrl, name: asset.name };
      } else if (name.includes("linux") || name.includes("ubuntu") || name.includes("deb") || name.includes("rpm") || name.includes("appimage")) {
        platformMap.linux = { url: browserDownloadUrl, name: asset.name };
      }
    });

    // Update download buttons
    document.querySelectorAll(".download-btn[data-platform]").forEach(function (btn) {
      var platform = btn.getAttribute("data-platform");
      var asset = platformMap[platform];

      if (asset) {
        btn.href = asset.url;
        var versionSpan = btn.querySelector(".btn-version");
        if (versionSpan) {
          versionSpan.textContent = tagName;
        }
      }
    });

    // Update the badge on hero section
    var badge = document.querySelector(".hero-badge span:last-child");
    if (badge) {
      badge.textContent = tagName + " — Ready for Download";
    }
  }

  // Run when DOM is ready
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", fetchReleases);
  } else {
    fetchReleases();
  }
})();
