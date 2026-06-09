const markdownRenderer = (() => {
  const renderer = new marked.Renderer();

  renderer.heading = ({ tokens, depth }) => {
    const text = renderer.parser.parseInline(tokens);
    const level = Math.min(depth + 1, 6);
    return `<h${level}>${text}</h${level}>`;
  };

  marked.setOptions({ gfm: true, breaks: false });

  return rawText => {
    const dirty = marked.parse(rawText, { renderer });
    return DOMPurify.sanitize(dirty);
  }
})();

function parseEvent(rawEvent) {
  let eventType = "";
  const dataLines = [];
  for (const line of rawEvent.split("\n")) {
    if (line.startsWith("event:")) {
      eventType = line.slice(6).trim();
    } else if (line.startsWith("data:")) {
      dataLines.push(line.slice(5).replace(/^ /, "")); // strip one optional leading space
    }
  }
  return { type: eventType, data: dataLines.join("\n") };
}

async function* parseSseStream(reader) {
  let buf = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += value;
    const events = buf.split("\n\n");
    buf = events.pop();
    for (const ev of events) {
      if (ev.trim()) yield parseEvent(ev)
    }
  }
}

Alpine.store("chat", {
  isStreaming: false,
  _abortController: null,
  totals: null,
  conversationID: "",
  title: "",
  isBlank: true,

  start(ac) {
    this._abortController = ac;
    this.isStreaming = true;
  },

  stop() {
    if (this._abortController) {
      this._abortController.abort();
      this._abortController = null;
    }
    this.isStreaming = false;
  },

  setConversation({ id, title, totals }) {
    this.conversationID = id ?? "";
    this.title = title ?? "";
    this.totals = totals ?? null;
    this.isBlank = !id;
  },

  get totalCost() {
    if (!this.totals) return null;
    return this.totals.cost_usd < 0.000001
      ? "<$0.000001"
      : "$" + this.totals.cost_usd.toFixed(6);
  },

  get contextPercent() {
    if (!this.totals || !this.totals.context_window) return 0;
    return Math.min(
      100,
      Math.round((this.totals.input_tokens / this.totals.context_window) * 100),
    );
  },

  get contextNearLimit() {
    const t = this.totals;
    if (!t || !t.context_window || !t.last_input_tokens) return false;
    const remaining = t.context_window - t.input_tokens;
    return remaining < t.last_input_tokens * 3;
  },
});

Alpine.data("chat", function () {
  function hideTabs() {
    Alpine.store("bottomtabs")?.hide();
  }

  function scrollToBottom() {
    window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });
  }

  function cloneTemplate(id, messages, before) {
    const node = document
      .getElementById(id)
      .content.cloneNode(true).firstElementChild;
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

  function appendMessageUsage(messageNode, payload) {
    if (!payload) return;
    const { usage } = payload;
    const node = document
      .getElementById("message-details")
      .content.cloneNode(true).firstElementChild;
    node.setAttribute("id", usage.message_id);
    node.querySelector(".input-tokens").textContent =
      usage.input_tokens.toLocaleString();
    node.querySelector(".cache-write-tokens").textContent =
      usage.cache_creation_input_tokens.toLocaleString();
    node.querySelector(".cache-read-tokens").textContent =
      usage.cache_read_input_tokens.toLocaleString();
    node.querySelector(".output-tokens").textContent =
      usage.output_tokens.toLocaleString();
    node.querySelector(".cost").textContent =
      usage.cost_usd < 0.000001
        ? "<$0.000001"
        : "$" + usage.cost_usd.toFixed(6);
    if (usage.timing) {
      node.querySelector(".ttfb").textContent = usage.timing.ttfb_ms + " ms";
      node.querySelector(".ttlb").textContent = usage.timing.ttlb_ms + " ms";
    }
    const button = messageNode.querySelector(".message-details");
    button.after(node);
    button.setAttribute("popovertarget", usage.message_id);
  }

  function appendAssistantMessage(messages, before) {
    const node = cloneTemplate("message-assistant", messages, before);
    const textElement = node.querySelector(".message-text");
    let rawText = "";
    let renderTimer = null;

    function render() {
      textElement.innerHTML = markdownRenderer(rawText);
    }

    return {
      appendWord(word) {
        rawText += word;
        if (renderTimer) return;
        renderTimer = setTimeout(() => {
          renderTimer = null;
          render();
        }, 60);
        scrollToBottom();
      },
      finalise(payload) {
        if (renderTimer) clearTimeout(renderTimer);
        render();

        node.setAttribute("aria-live", "polite");
        node.querySelector(".assistant-badge").classList.remove("loading");
        node.querySelector(".output-tokens").textContent =
          `${payload?.usage?.output_tokens?.toLocaleString()} tok`;
        node
          .querySelector(".message-copy")
          .addEventListener("click", (e) =>
            copyWithFeedback(e.currentTarget, rawText),
          );
        appendMessageUsage(node, payload);
        node.querySelector(".message-toolbar").classList.remove("opacity-0");
      },
      cancel() {
        if (renderTimer) clearTimeout(renderTimer);

        const anchor = node.nextSibling;
        if (rawText) {
          render();
          node.querySelector(".assistant-badge").classList.remove("loading");
        } else {
          node.remove();
        }
        node.querySelector(".message-toolbar").remove();
        cloneTemplate("message-cancelled", messages, anchor);
      },
    };
  }

  function hydrateConversation(conv, messages, before) {
    for (let i = 0; i < conv.messages.length; i++) {
      const m = conv.messages[i];
      if (m.role === "user") {
        appendUserMessage(m.content, messages, before);
      } else if (m.role === "assistant") {
        if (m.cancelled) {
          const reply = appendAssistantMessage(messages, before);
          reply.cancel();
        } else {
          const reply = appendAssistantMessage(messages, before);
          reply.appendWord(m.content);
          reply.finalise({ usage: m.usage });
        }
      }
    }
    scrollToBottom();
  }

  function greeting() {
    const h = new Date().getHours();
    if (h < 12) return "Good morning.";
    if (h < 18) return "Good afternoon.";
    return "Good evening.";
  }

  return {
    _activeReply: null,
    greeting: greeting(),

    init() {
      this.$el.addEventListener("chat-submit", (e) =>
        this.onSubmit(e.detail.text),
      );
      this.$el.addEventListener("chat-stop", () => this.onStop());

      const conv = <<< .ConversationJSON >>>;
      if (conv?.messages?.length > 0) {
        Alpine.store("chat").setConversation(conv);
        hideTabs();
        hydrateConversation(
          conv,
          this.$refs.messages,
          this.$refs.loadingIndicator,
        );
      }
    },

    onStop() {
      if (this._activeReply) {
        this._activeReply.cancel();
        this._activeReply = null;
      }
      Alpine.store("chat").stop();
      const textarea = this.$el.querySelector("textarea");
      if (textarea) textarea.focus();
    },

    handleEvent(reply, { type, data }) {
      switch (type) {
        case "word":
          reply.appendWord(data);
          break;
        case "done": {
          let payload = null;
          try {
            payload = JSON.parse(data);
          } catch (_) {}
          const store = Alpine.store("chat");
          const isNew = !store.conversationID;
          if (payload?.conversation_id) {
            store.setConversation({
              id: payload.conversation_id,
              title: payload.title,
              totals: payload.totals,
            });
            if (payload.title) document.title = payload.title;
          }
          if (isNew && store.conversationID) {
            history.replaceState(null, "", "/chat/" + store.conversationID);
          }
          reply.finalise(payload);
          this._activeReply = null;
          store.stop();
          break;
        }
      }
    },

    async onSubmit(text) {
      const store = Alpine.store("chat");
      const before = this.$refs.loadingIndicator;

      appendUserMessage(text, this.$refs.messages, before);
      store.isBlank = false;
      hideTabs();

      const reply = appendAssistantMessage(this.$refs.messages, before);
      this._activeReply = reply;

      const ac = new AbortController();
      store.start(ac);

      try {
        const res = await fetch("/chat/stream", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            conversation_id: store.conversationID,
            message: text,
          }),
          signal: ac.signal,
        });

        const reader = res.body
          .pipeThrough(new TextDecoderStream())
          .getReader();

        for await (const event of parseSseStream(reader)) {
          this.handleEvent(reply, event);
        }
      } catch (e) {
        if (e.name !== "AbortError") store.stop();
      }
    },
  };
});
