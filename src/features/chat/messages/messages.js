// marked and DOMPurify load deferred, so the renderer is built on first use
// rather than at parse time of this inline script.
let renderMarkdown = null;

function markdownRenderer(rawText) {
  if (!renderMarkdown) {
    const renderer = new marked.Renderer();
    renderer.heading = ({ tokens, depth }) => {
      const text = renderer.parser.parseInline(tokens);
      const level = Math.min(depth + 1, 6);
      return `<h${level}>${text}</h${level}>`;
    };
    marked.setOptions({ gfm: true, breaks: false });
    renderMarkdown = (raw) =>
      DOMPurify.sanitize(marked.parse(raw, { renderer }));
  }
  return renderMarkdown(rawText);
}

function scrollToBottom() {
  window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });
}

function templateContent(id) {
  return document.getElementById(id).content.cloneNode(true).firstElementChild;
}

function cloneTemplate(id, messages, before) {
  const node = templateContent(id);
  messages.insertBefore(node, before);
  scrollToBottom();
  return node;
}

function copyWithFeedback(button, text) {
  navigator.clipboard.writeText(text).then(() => {
    const icon = button.querySelector(".icon");
    icon.textContent = "check";
    setTimeout(() => (icon.textContent = "content_copy"), 2500);
  });
}

function appendUserMessage(text, messages, before) {
  const node = cloneTemplate("message-user", messages, before);
  node.querySelector(".message-text").textContent = text;
}

function appendMessageUsage(messageNode, exchange) {
  if (!exchange?.usage || !exchange.id) return;
  const { usage } = exchange;
  const popoverTrigger = templateContent("message-details-trigger");
  const popover = templateContent("message-details");
  popover.setAttribute("id", exchange.id);
  popover.querySelector(".input-tokens").textContent =
    usage.display.input_tokens;
  popover.querySelector(".cache-write-tokens").textContent =
    usage.display.cache_creation_input_tokens;
  popover.querySelector(".cache-read-tokens").textContent =
    usage.display.cache_read_input_tokens;
  popover.querySelector(".output-tokens").textContent =
    usage.display.output_tokens;
  popover.querySelector(".cost").textContent = usage.display.cost;
  if (exchange.timing) {
    popover.querySelector(".ttfb").textContent = exchange.timing.ttfb_ms + " ms";
    popover.querySelector(".ttlb").textContent = exchange.timing.ttlb_ms + " ms";
  }
  popoverTrigger.querySelector(".output-tokens").textContent =
    `${usage.display.output_tokens} tok`;
  popoverTrigger.setAttribute("popovertarget", exchange.id);

  messageNode.querySelector(".message-copy").after(popoverTrigger);
  popoverTrigger.after(popover);
}

function appendAssistantMessage(messages, before) {
  const node = cloneTemplate("message-assistant", messages, before);
  const segments = node.querySelector(".message-segments");
  let rawText = "";
  let segmentText = "";
  let segmentElement = null;
  let openToolElement = null;
  let renderTimer = null;

  function render() {
    if (segmentElement) {
      segmentElement.innerHTML = markdownRenderer(segmentText);
    }
  }

  function flushRender() {
    if (renderTimer) {
      clearTimeout(renderTimer);
      renderTimer = null;
    }
    render();
  }

  return {
    appendText(text) {
      if (!segmentElement) {
        const segment = templateContent("message-assistant-text");
        segmentElement = segment.querySelector(".message-text");
        segments.appendChild(segment);
        segmentText = "";
        if (rawText) rawText += "\n\n";
      }
      segmentText += text;
      rawText += text;
      if (renderTimer) return;
      renderTimer = setTimeout(() => {
        renderTimer = null;
        render();
      }, 60);
      scrollToBottom();
    },
    appendTool(tool) {
      flushRender();
      segmentElement = null;

      openToolElement = templateContent("message-tool");
      openToolElement.querySelector(".tool-name").textContent = tool.name;
      openToolElement.querySelector(".tool-input").textContent =
        JSON.stringify(tool.input, null, 2);
      segments.appendChild(openToolElement);
      scrollToBottom();
    },
    toolResult(tool) {
      if (!openToolElement) return;
      const output = openToolElement.querySelector(".tool-output");
      output.textContent = tool.result;
      if (tool.is_error) {
        output.classList.add("text-error");
      }
      openToolElement = null;
      scrollToBottom();
    },
    finalise(exchange) {
      flushRender();

      node.setAttribute("aria-live", "polite");
      node
        .querySelector(".message-copy")
        .addEventListener("click", (e) =>
          copyWithFeedback(e.currentTarget, rawText),
        );
      appendMessageUsage(node, exchange);
      node.querySelector(".message-toolbar").classList.remove("opacity-0");
    },
    cancel() {
      flushRender();

      const anchor = node.nextSibling;
      if (segments.childElementCount > 0) {
        node.querySelector(".assistant-badge")?.classList?.remove("loading");
      } else {
        node.remove();
      }
      node.querySelector(".message-toolbar").remove();
      cloneTemplate("message-cancelled", messages, anchor);
    },
  };
}

// Server-rendered history arrives with raw markdown as text content. This
// one-shot pass renders it through the same markdown pipeline the live
// stream uses, wires the copy buttons, and jumps to the latest message.
document.addEventListener("DOMContentLoaded", () => {
  const messages = document.querySelector('[x-ref="messages"]');
  if (!messages) return;
  const hydrated = messages.querySelectorAll('[aria-label="Assistant"]');
  for (const node of hydrated) {
    const textElements = [
      ...node.querySelectorAll(".message-segments .message-text"),
    ];
    const rawText = textElements.map((el) => el.textContent).join("\n\n");
    for (const el of textElements) {
      el.innerHTML = markdownRenderer(el.textContent);
    }
    const copyButton = node.querySelector(".message-copy");
    if (copyButton) {
      copyButton.addEventListener("click", (e) =>
        copyWithFeedback(e.currentTarget, rawText),
      );
    }
  }
  if (hydrated.length > 0) {
    window.scrollTo(0, document.body.scrollHeight);
  }
});

window.chatMessages = { appendUserMessage, appendAssistantMessage };
