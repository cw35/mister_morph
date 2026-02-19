(() => {
  const root = document.documentElement;
  const themeToggle = document.querySelector("[data-theme-toggle]");
  const THEME_KEY = "mistermorph-theme";

  const resolveInitialTheme = () => {
    const saved = window.localStorage.getItem(THEME_KEY);
    if (saved === "dark" || saved === "light") {
      return saved;
    }
    return "light";
  };

  const applyTheme = (theme) => {
    root.dataset.theme = theme;
    if (!themeToggle) {
      return;
    }
    themeToggle.setAttribute("aria-pressed", theme === "light" ? "true" : "false");
  };

  applyTheme(resolveInitialTheme());
  if (themeToggle) {
    themeToggle.addEventListener("click", () => {
      const next = root.dataset.theme === "light" ? "dark" : "light";
      applyTheme(next);
      window.localStorage.setItem(THEME_KEY, next);
    });
  }

  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  const header = document.querySelector(".site-header");
  const onScroll = () => {
    if (!header) {
      return;
    }
    header.classList.toggle("is-scrolled", window.scrollY > 8);
  };

  onScroll();
  window.addEventListener("scroll", onScroll, { passive: true });

  const langMenu = document.querySelector(".lang-menu");
  if (langMenu) {
    document.addEventListener("click", (event) => {
      const target = event.target;
      if (target instanceof Node && !langMenu.contains(target)) {
        langMenu.removeAttribute("open");
      }
    });
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        langMenu.removeAttribute("open");
      }
    });
  }

  const writeClipboard = async (text) => {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return;
    }
    const fallback = document.createElement("textarea");
    fallback.value = text;
    fallback.setAttribute("readonly", "");
    fallback.style.position = "fixed";
    fallback.style.opacity = "0";
    document.body.appendChild(fallback);
    fallback.select();
    document.execCommand("copy");
    fallback.remove();
  };

  const heroVisual = document.getElementById("hero-visual");
  if (heroVisual && !reduceMotion) {
    const setTilt = (event) => {
      const rect = heroVisual.getBoundingClientRect();
      const x = (event.clientX - rect.left) / rect.width;
      const y = (event.clientY - rect.top) / rect.height;

      const rx = (0.5 - y) * 7;
      const ry = (x - 0.5) * 7;

      heroVisual.style.setProperty("--rx", `${rx.toFixed(2)}deg`);
      heroVisual.style.setProperty("--ry", `${ry.toFixed(2)}deg`);
    };

    heroVisual.addEventListener("pointermove", setTilt);
    heroVisual.addEventListener("pointerleave", () => {
      heroVisual.style.setProperty("--rx", "0deg");
      heroVisual.style.setProperty("--ry", "0deg");
    });
  }

  const terminalShell = document.querySelector(".terminal-shell");
  const terminalCode = terminalShell?.querySelector("code");
  if (terminalCode) {
    const escapeHTML = (value) =>
      value
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;");

    const inlineTokenize = (raw) => {
      const tokenRe =
        /(--[a-z-]+|"(?:[^"\\]|\\.)*"|\b(?:read_file|contacts_send|web_search|bash|url_fetch|maep|agent)\b|\b\d+(?:\.\d+)?s\b|->)/gi;
      let out = "";
      let last = 0;

      for (const match of raw.matchAll(tokenRe)) {
        const idx = match.index ?? 0;
        const token = match[0];
        out += escapeHTML(raw.slice(last, idx));

        let className = "tok-cmd";
        if (token.startsWith("--")) {
          className = "tok-flag";
        } else if (token.startsWith("\"")) {
          className = "tok-str";
        } else if (token === "->") {
          className = "tok-arrow";
        } else if (/^\d+(\.\d+)?s$/i.test(token)) {
          className = "tok-time";
        } else {
          className = "tok-tool";
        }

        out += `<span class="${className}">${escapeHTML(token)}</span>`;
        last = idx + token.length;
      }

      out += escapeHTML(raw.slice(last));
      return out;
    };

    const highlightLine = (line) => {
      const prompt = line.match(/^(\s*)\$ (.*)$/);
      if (prompt) {
        const [, indent, cmd] = prompt;
        if (cmd.trim() === "_") {
          return `${escapeHTML(indent)}<span class="tok-prompt">$</span> <span class="tok-cursor" aria-hidden="true"></span>`;
        }
        return `${escapeHTML(indent)}<span class="tok-prompt">$</span> <span class="tok-cmd">${inlineTokenize(cmd)}</span>`;
      }

      const tagged = line.match(/^(\s*)\[([^\]]+)\](.*)$/);
      if (tagged) {
        const [, indent, tag, rest] = tagged;
        let tagClass = "tok-tag";
        if (/guard/i.test(tag)) {
          tagClass = "tok-tag-guard";
        } else if (/(maep|agent)/i.test(tag)) {
          tagClass = "tok-tag-link";
        } else if (/result/i.test(tag)) {
          tagClass = "tok-tag-result";
        }

        return `${escapeHTML(indent)}<span class="${tagClass}">[${escapeHTML(tag)}]</span>${inlineTokenize(rest)}`;
      }

      return inlineTokenize(line);
    };

    const raw = terminalCode.textContent ?? "";
    terminalCode.innerHTML = raw.split("\n").map(highlightLine).join("\n");

    const terminalCopyButton = terminalShell?.querySelector("[data-terminal-copy]");
    if (terminalCopyButton instanceof HTMLButtonElement) {
      const copyLabel =
        terminalCopyButton.dataset.copyLabel ?? terminalCopyButton.textContent ?? "Copy";
      const copiedLabel = terminalCopyButton.dataset.copiedLabel ?? "Copied";
      const copyText = raw
        .split("\n")
        .map((line) => {
          const prompt = line.match(/^\s*\$\s+(.*)$/);
          if (!prompt) {
            return "";
          }
          const command = prompt[1].trim();
          if (!command || command === "_") {
            return "";
          }
          return command;
        })
        .filter(Boolean)
        .join("\n");
      let copyTimer;

      terminalCopyButton.addEventListener("click", async () => {
        try {
          await writeClipboard(copyText || raw.trimEnd());
          terminalCopyButton.textContent = copiedLabel;
          terminalCopyButton.classList.add("is-copied");
          if (copyTimer) {
            window.clearTimeout(copyTimer);
          }
          copyTimer = window.setTimeout(() => {
            terminalCopyButton.textContent = copyLabel;
            terminalCopyButton.classList.remove("is-copied");
          }, 1600);
        } catch (_error) {
          terminalCopyButton.textContent = copyLabel;
          terminalCopyButton.classList.remove("is-copied");
        }
      });
    }
  }

  const docCodeBlocks = document.querySelectorAll(".doc-page-content pre");
  if (docCodeBlocks.length > 0) {
    const lang = (document.documentElement.lang ?? "").toLowerCase();
    let copyLabel = "Copy";
    let copiedLabel = "Copied";
    if (lang.startsWith("zh")) {
      copyLabel = "复制";
      copiedLabel = "已复制";
    } else if (lang.startsWith("ja")) {
      copyLabel = "コピー";
      copiedLabel = "コピー済み";
    }

    for (const block of docCodeBlocks) {
      const code = block.querySelector("code");
      const text = (code?.textContent ?? block.textContent ?? "").trimEnd();
      if (!text) {
        continue;
      }

      const button = document.createElement("button");
      button.type = "button";
      button.className = "doc-code-copy-btn";
      button.textContent = copyLabel;
      button.setAttribute("aria-label", copyLabel);
      block.appendChild(button);

      let timer;
      button.addEventListener("click", async () => {
        try {
          await writeClipboard(text);
          button.textContent = copiedLabel;
          button.classList.add("is-copied");
          if (timer) {
            window.clearTimeout(timer);
          }
          timer = window.setTimeout(() => {
            button.textContent = copyLabel;
            button.classList.remove("is-copied");
          }, 1500);
        } catch (_error) {
          button.textContent = copyLabel;
          button.classList.remove("is-copied");
        }
      });
    }
  }

  const animatedAsciiCat = document.querySelector(
    '.ascii-cat[data-ascii-animation="cat-404"]',
  );
  if (animatedAsciiCat && !reduceMotion) {
    const catTemplate = [
      "　　　　　　　　　　　,,:''\"´\"ヽ''\"´´\"''::､",
      "　　　　　　　 　 ,;'':;;'　　　,;'':;,　  ':,",
      "　　　　　　　　,;' 　 ';.,,.,.,.,.,.,;' 　   ';,　　　ヽ,,",
      "　　　　　　　,:'　　　　　　　　　　　｀:､　　　'; ''\"''::,",
      "　　　　　　,:'　　　 　 　 　 　 　 　 　 ',　;'　　　　:;",
      "　　　　　 ,'　　　　　　　　　　　　　 　 ;',:'　　 　 ,;'",
      "　　　　　 ;　     　{eyeL}　　　　　　{eyeR}　　　　.;'､''ｰ'''\"´",
      "　　　 ,,::'\"';,二　　（{mouth}） 　 　二二;'　｀'::,",
      "　　　;' 　　｀':.､　　　　　　　　　 ,.,,::''　　　　'';",
      "　　　'':.,.,.,.,::-'' '' '' '\"\"´ 　 ｀''::､.,.,.,.::'",
    ];

    const eyes = [
      "○",
      "○",
      "○",
      "○",
      "◕",
      "○",
      "○",
      "－",
      "○",
      "○",
      "◕",
      "○",
      "○",
      "○",
      "○",
      "－",
      "○",
      "◕",
      "○",
      "○",
      "○",
      "○",
      "－",
      "○",
    ];

    const mouths = [
      "__人__",
      "__人__",
      "__ω__",
      "__人__",
      "__▽__",
      "__人__",
      "__ω__",
      "__人__",
      "__ᴗ__",
      "__人__",
      "__ω__",
      "__人__",
      "__▽__",
      "__人__",
      "__人__",
      "__ω__",
      "__人__",
      "__▽__",
      "__人__",
      "__ω__",
      "__人__",
      "__ᴗ__",
      "__人__",
      "__ω__",
    ];

    const shifts = [
      0,
      1,
      2,
      1,
      0,
      0,
      1,
      2,
      1,
      0,
      0,
      1,
      2,
      1,
      0,
      0,
      1,
      2,
      1,
      0,
      0,
      1,
      2,
      1,
    ];

    const frameCount = eyes.length;
    const frameDurationMs = 90;
    let frameIndex = 0;
    let lastPaint = 0;

    const renderFrame = (index) => {
      const shift = " ".repeat(shifts[index]);
      const face = catTemplate
        .map((line) =>
          shift +
          line
            .replace("{eyeL}", eyes[index])
            .replace("{eyeR}", eyes[index])
            .replace("{mouth}", mouths[index]),
        )
        .join("\n");
      animatedAsciiCat.textContent = face;
    };

    const animate = (ts) => {
      if (ts - lastPaint >= frameDurationMs) {
        renderFrame(frameIndex);
        frameIndex = (frameIndex + 1) % frameCount;
        lastPaint = ts;
      }
      window.requestAnimationFrame(animate);
    };

    renderFrame(0);
    window.requestAnimationFrame(animate);
  }
})();
